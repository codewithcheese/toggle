package toggle

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func runConfigured(t *testing.T, cwd, configPath string, args ...string) (int, string, string) {
	t.Helper()
	var out, diagnostics bytes.Buffer
	a := app{cwd: cwd, configPath: configPath, out: &out, errOut: &diagnostics}
	code := a.run(context.Background(), args)
	return code, out.String(), diagnostics.String()
}

// The store root is fixture data, never an implicit global store.
func runCLI(t *testing.T, cwd, storeRoot string, args ...string) (int, string, string) {
	t.Helper()
	stores := map[string]string{}
	if storeRoot != "" {
		stores["team"] = storeRoot
	}
	return runConfigured(t, cwd, testConfig(t, stores), args...)
}

func legacyPath(r *repo) string { return filepath.Join(r.gitDir, "toggle.json") }

func TestCommandRootGuardsAndReadOnlyPreviews(t *testing.T) {
	r := testRepo(t)
	store := t.TempDir()
	f := testFeature(t, store, "review", ".agents/skills/review/SKILL.md", "skill")
	sub, outside := filepath.Join(r.root, "docs"), t.TempDir()
	os.Mkdir(sub, 0755)
	for _, cwd := range []string{sub, outside} {
		for _, args := range [][]string{{"on", "review"}, {"off", "review"}, {"on", "review", "--dry-run"}, {"off", "review", "--dry-run"}} {
			before := snapshot(t, r.root, store, outside)
			code, _, diagnostics := runCLI(t, cwd, store, args...)
			if code == 0 || !strings.Contains(diagnostics, "repo root") {
				t.Fatal(code, diagnostics)
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, store, outside)) {
				t.Fatal("root rejection changed files")
			}
		}
	}
	before := snapshot(t, r.root, store)
	code, output, diagnostics := runCLI(t, r.root, store, "on", "review", "--store", "team", "--dry-run")
	if code != 0 || !strings.Contains(output, "install") || !strings.Contains(output, "add exclusion") {
		t.Fatal(code, output, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, store)) {
		t.Fatal("on preview changed files")
	}
	applyFeature(t, r, f, "on")
	before = snapshot(t, r.root, store)
	code, output, diagnostics = runCLI(t, r.root, store, "off", "review", "--store", "team", "--dry-run")
	if code != 0 || !strings.Contains(output, "remove empty directory\t.agents") {
		t.Fatal(code, output, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, store)) {
		t.Fatal("off preview changed files")
	}
	put(t, filepath.Join(r.root, ".agents/skills/review/SKILL.md"), "edited file", 0755)
	before = snapshot(t, r.root, store)
	realCode, _, realError := runCLI(t, r.root, store, "off", "review", "--store", "team")
	previewCode, _, previewError := runCLI(t, r.root, store, "off", "review", "--store", "team", "--dry-run")
	if realCode == 0 || previewCode != realCode || realError != previewError || !strings.Contains(realError, f.dir) {
		t.Fatal(realCode, realError, previewCode, previewError)
	}
	if strings.Contains(realError, "edited file") || strings.Contains(realError, "---") || strings.Contains(realError, "+++") {
		t.Fatal("generated content diff", realError)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, store)) {
		t.Fatal("conflicting commands changed files")
	}
}

func TestStatusCandidateComparisons(t *testing.T) {
	for _, kind := range []string{"absent", "complete", "partial", "different", "mode", "tracked", "directory", "zero-matching-partial", "path-blocker"} {
		t.Run(kind, func(t *testing.T) {
			r := testRepo(t)
			store := t.TempDir()
			f := testFeature(t, store, "review", "instructions.md", "source")
			path := filepath.Join(r.root, "instructions.md")
			if kind != "absent" {
				put(t, path, "source", 0644)
			}
			switch kind {
			case "partial", "zero-matching-partial":
				put(t, filepath.Join(f.dir, "source/missing.md"), "missing", 0644)
				if kind == "zero-matching-partial" {
					put(t, path, "edited", 0644)
				}
			case "different":
				put(t, path, "edited", 0644)
			case "mode":
				os.Chmod(path, 0755)
			case "tracked":
				testGit(t, r.root, "add", "instructions.md")
			case "directory":
				os.Remove(path)
				os.Mkdir(path, 0755)
			case "path-blocker":
				put(t, filepath.Join(f.dir, "source/.git/config"), "reserved", 0644)
			}
			sub := filepath.Join(r.root, "docs")
			os.Mkdir(sub, 0755)
			before := snapshot(t, r.root, store)
			code, out, diagnostics := runCLI(t, r.root, store, "status")
			if code != 0 {
				t.Fatal("blocked comparison failed scan", code, out, diagnostics)
			}
			want := "blocked"
			if kind == "complete" {
				want = "ready for off"
			}
			if kind == "absent" {
				if strings.Contains(out, "review\t") || !strings.Contains(out, "other setup may be active") {
					t.Fatal("absent definition reported as candidate", out)
				}
			} else if !strings.Contains(out, "review\t--store team\t"+want) || !strings.Contains(out, f.dir) {
				t.Fatal(kind, out)
			}
			if kind == "different" || kind == "zero-matching-partial" {
				if !strings.Contains(out, "0 matching") || !strings.Contains(out, "1 different") {
					t.Fatal("zero-match conflict hidden", out)
				}
			}
			if subCode, subOut, subError := runCLI(t, sub, store, "status"); subCode != code || subOut != out || subError != diagnostics {
				t.Fatal("status changed with working directory", subCode, subOut, subError)
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, store)) {
				t.Fatal("status changed files")
			}
		})
	}
}

func TestStatusSeparatesScanErrorsAndDuplicateDefinitions(t *testing.T) {
	r := testRepo(t)
	a, b := t.TempDir(), t.TempDir()
	testFeature(t, a, "review", "instructions.md", "source")
	testFeature(t, b, "review", "instructions.md", "source")
	put(t, filepath.Join(r.root, "instructions.md"), "source", 0644)
	path := testConfig(t, map[string]string{"personal": a, "team": b})
	code, out, diagnostics := runConfigured(t, r.root, path, "status")
	if code != 0 || strings.Count(out, "ready for off") != 2 || !strings.Contains(out, "--store personal") || !strings.Contains(out, "--store team") {
		t.Fatal(code, out, diagnostics)
	}
	put(t, path, "invalid JSON", 0644)
	if code, _, diagnostics := runConfigured(t, r.root, path, "status"); code == 0 || !strings.Contains(diagnostics, path) {
		t.Fatal("invalid config scan succeeded", code, diagnostics)
	}
	path = testConfig(t, map[string]string{"missing": filepath.Join(t.TempDir(), "missing")})
	if code, _, diagnostics := runConfigured(t, r.root, path, "status"); code == 0 || !strings.Contains(diagnostics, "missing") {
		t.Fatal("unavailable source scan succeeded", code, diagnostics)
	}
	if code, _, diagnostics := runConfigured(t, t.TempDir(), path, "status"); code == 0 || !strings.Contains(diagnostics, "requires a Git repo") {
		t.Fatal(code, diagnostics)
	}
}

func TestStatusUsesFreshCurrentRepo(t *testing.T) {
	a, b := testRepo(t), testRepo(t)
	store := t.TempDir()
	testFeature(t, store, "review", "instructions.md", "source")
	put(t, filepath.Join(a.root, "instructions.md"), "source", 0644)
	path := testConfig(t, map[string]string{"team": store})
	_, out, _ := runConfigured(t, a.root, path, "status")
	if !strings.Contains(out, "ready for off") {
		t.Fatal(out)
	}
	_, out, _ = runConfigured(t, b.root, path, "status")
	if strings.Contains(out, "review\t") {
		t.Fatal("history leaked across repos", out)
	}
	put(t, filepath.Join(a.root, "instructions.md"), "edited", 0644)
	_, out, _ = runConfigured(t, a.root, path, "status")
	if !strings.Contains(out, "blocked") || strings.Contains(out, "ready for off") {
		t.Fatal("status reused old cache", out)
	}
}

func TestCLIPartialFailureHasNoRecordLocations(t *testing.T) {
	r := testRepo(t)
	store := t.TempDir()
	testFeature(t, store, "review", "instructions.md", "source")
	var out, diagnostics bytes.Buffer
	ops := defaultOps()
	ops.write = func(string, []byte, os.FileMode) error { return errors.New("injected copy failure") }
	a := app{cwd: r.root, configPath: testConfig(t, map[string]string{"team": store}), out: &out, errOut: &diagnostics, ops: &ops}
	if code := a.run(context.Background(), []string{"on", "review", "--store", "team"}); code == 0 || !strings.Contains(diagnostics.String(), "Completed effects:") || !strings.Contains(diagnostics.String(), "instructions.md") || strings.Contains(diagnostics.String(), "bookkeeping") || out.Len() != 0 {
		t.Fatal(code, out.String(), diagnostics.String())
	}
}
