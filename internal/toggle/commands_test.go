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

func runCLI(t *testing.T, cwd, global string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	a := app{cwd: cwd, globalRoot: global, out: &out, errOut: &errOut}
	code := a.run(context.Background(), args)
	return code, out.String(), errOut.String()
}

func TestCommandRootGuards(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	testFeature(t, global, "review", "instructions.md", "source")
	sub := filepath.Join(r.root, "docs")
	os.Mkdir(sub, 0755)
	outside := t.TempDir()
	for _, cwd := range []string{sub, outside} {
		for _, args := range [][]string{{"on", "review"}, {"off", "review"}, {"on", "review", "--dry-run"}, {"off", "review", "--dry-run"}} {
			before := snapshot(t, r.root, global, outside)
			code, _, diagnostics := runCLI(t, cwd, global, args...)
			if code == 0 || !strings.Contains(diagnostics, "repo root") {
				t.Fatalf("accepted %v in %s: %d %s", args, cwd, code, diagnostics)
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, global, outside)) {
				t.Fatal("rejected command changed files")
			}
		}
	}
}

func TestPreviewsAndConflicts(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	f := testFeature(t, global, "review", "instructions.md", "source instructions")
	before := snapshot(t, r.root, global)
	code, out, diagnostics := runCLI(t, r.root, global, "on", "review", "--dry-run")
	if code != 0 || !strings.Contains(out, "instructions.md") || !strings.Contains(out, "exclusion") {
		t.Fatal(code, out, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("preview wrote files")
	}
	put(t, filepath.Join(r.root, "instructions.md"), "existing instructions", 0644)
	before = snapshot(t, r.root, global)
	realCode, _, realErr := runCLI(t, r.root, global, "on", "review")
	previewCode, _, previewErr := runCLI(t, r.root, global, "on", "review", "--dry-run")
	if realCode == 0 || previewCode != realCode || previewErr != realErr || !strings.Contains(realErr, filepath.Join(f.dir, "source/instructions.md")) {
		t.Fatal(realCode, realErr, previewCode, previewErr)
	}
	for _, text := range []string{"existing instructions", "source instructions", "---", "+++"} {
		if strings.Contains(realErr, text) {
			t.Fatal("generated content/diff in diagnostics", realErr)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("conflict changed files")
	}
	os.Remove(filepath.Join(r.root, "instructions.md"))
	applyFeature(t, r, f, "on")
	put(t, filepath.Join(r.root, "instructions.md"), "saved edit", 0755)
	put(t, filepath.Join(f.dir, "source/instructions.md"), "saved edit", 0755)
	before = snapshot(t, r.root, global)
	code, out, diagnostics = runCLI(t, r.root, global, "off", "review", "--dry-run")
	if code != 0 || !strings.Contains(out, "remove\tinstructions.md") || !strings.Contains(out, "remove exclusion") {
		t.Fatal(code, out, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("off preview changed files")
	}
}

func TestStatusClassificationsAndLocations(t *testing.T) {
	for _, kind := range []string{"off", "on", "edit", "preserved", "missing-file", "missing-definition", "tracked-edit", "source-drift", "redirected", "blocked-on"} {
		t.Run(kind, func(t *testing.T) {
			r := testRepo(t)
			global := t.TempDir()
			f := testFeature(t, global, "review", "instructions.md", "source")
			target := filepath.Join(r.root, "instructions.md")
			if kind != "off" && kind != "blocked-on" {
				applyFeature(t, r, f, "on")
			}
			want := "on"
			switch kind {
			case "off":
				want = "off"
			case "edit", "preserved", "tracked-edit", "blocked-on":
				put(t, target, "edited", 0755)
				want = "conflicted"
				if kind == "preserved" || kind == "tracked-edit" {
					put(t, filepath.Join(f.dir, "source/instructions.md"), "edited", 0755)
				}
				if kind == "preserved" {
					want = "modified"
				}
				if kind == "tracked-edit" {
					testGit(t, r.root, "add", "-f", "instructions.md")
				}
			case "missing-file":
				os.Remove(target)
				want = "modified"
			case "missing-definition":
				os.RemoveAll(f.dir)
				want = "conflicted"
			case "source-drift":
				put(t, filepath.Join(f.dir, "source/instructions.md"), "new global", 0644)
			case "redirected":
				os.Rename(target, filepath.Join(r.root, "old.md"))
				os.Symlink("other.md", target)
				want = "conflicted"
			}
			sub := filepath.Join(r.root, "docs")
			os.Mkdir(sub, 0755)
			before := snapshot(t, r.root, global)
			code, output, diagnostics := runCLI(t, r.root, global, "status")
			if code != 0 || !strings.Contains(output, "review\t"+want+"\n") && !strings.Contains(output, "review\t"+want+"\t") {
				t.Fatal(kind, code, output, diagnostics)
			}
			if want == "conflicted" && !strings.Contains(output, "blocked") {
				t.Fatal("missing blocked transition", output)
			}
			subCode, subOutput, subErr := runCLI(t, sub, global, "status")
			if subCode != code || subOutput != output || subErr != diagnostics {
				t.Fatal("subdirectory status differs", subCode, subOutput, subErr)
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
				t.Fatal("status changed files")
			}
		})
	}
	outside := t.TempDir()
	global := t.TempDir()
	testFeature(t, global, "review", "instructions.md", "source")
	before := snapshot(t, outside, global)
	code, _, diagnostics := runCLI(t, outside, global, "status")
	if code == 0 || !strings.Contains(diagnostics, "requires a Git repo") || !reflect.DeepEqual(before, snapshot(t, outside, global)) {
		t.Fatal(code, diagnostics)
	}
}

func TestCLIReportsPartialEffects(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	testFeature(t, global, "review", "instructions.md", "source")
	var out, errOut bytes.Buffer
	ops := defaultOps()
	ops.write = func(path string, data []byte, mode os.FileMode) error { return errors.New("injected write failure") }
	a := app{cwd: r.root, globalRoot: global, out: &out, errOut: &errOut, ops: &ops}
	if code := a.run(context.Background(), []string{"on", "review"}); code == 0 || !strings.Contains(errOut.String(), "Completed effects:") || !strings.Contains(errOut.String(), "instructions.md") || out.Len() != 0 {
		t.Fatal(code, &out, &errOut)
	}
}

func TestStatusUsesCurrentRepo(t *testing.T) {
	a, b := testRepo(t), testRepo(t)
	global := t.TempDir()
	f := testFeature(t, global, "review", "instructions.md", "source")
	applyFeature(t, a, f, "on")
	before := snapshot(t, a.root, b.root, global)
	for _, test := range []struct{ dir, label string }{{a.root, "on"}, {b.root, "off"}} {
		code, out, diagnostics := runCLI(t, test.dir, global, "status")
		if code != 0 || out != "review\t"+test.label+"\n" {
			t.Fatal(code, out, diagnostics)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, a.root, b.root, global)) {
		t.Fatal("cross-repo status changed files")
	}
}

func TestAllConflictsBlockEveryTarget(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	f := testFeature(t, global, "review", "absent.md", "new file")
	put(t, filepath.Join(f.dir, "source/conflicting.md"), "proposed", 0644)
	put(t, filepath.Join(r.root, "conflicting.md"), "existing edit", 0644)
	before := snapshot(t, r.root, global)
	for _, args := range [][]string{{"on", "review"}, {"on", "review", "--dry-run"}} {
		code, _, diagnostics := runCLI(t, r.root, global, args...)
		if code == 0 || !strings.Contains(diagnostics, "conflicting.md") {
			t.Fatal(code, diagnostics)
		}
		assertAbsent(t, filepath.Join(r.root, "absent.md"))
		if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
			t.Fatal("conflict allowed part of the operation to apply")
		}
	}
}

func TestSeparateGitDirectoryCLI(t *testing.T) {
	dir := t.TempDir()
	metadata := filepath.Join(t.TempDir(), "gitdata")
	testGit(t, dir, "init", "-q", "--separate-git-dir", metadata)
	global := t.TempDir()
	testFeature(t, global, "review", "instructions.md", "source")
	pointer := mustRead(t, filepath.Join(dir, ".git"))
	code, _, diagnostics := runCLI(t, dir, global, "on", "review")
	if code != 0 {
		t.Fatal(code, diagnostics)
	}
	if mustRead(t, filepath.Join(dir, ".git")) != pointer {
		t.Fatal("Git pointer file changed")
	}
	state, _, err := loadRecords(filepath.Join(metadata, "toggle.json"))
	if err != nil || len(state.Features) != 1 {
		t.Fatal(state, err)
	}
	if !strings.Contains(mustRead(t, filepath.Join(metadata, "info/exclude")), "/instructions.md") {
		t.Fatal("exclusion not in separate Git directory")
	}
	testGit(t, dir, "check-ignore", "-q", "instructions.md")
}

func TestOffAfterDirectorySymlinkRetargeting(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	relative := ".agents/skills/review/SKILL.md"
	f := testFeature(t, global, "review", relative, "installed skill")
	if err := os.Mkdir(filepath.Join(r.root, ".agents"), 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(r.root, ".agents/skills")
	if err := os.Symlink("../.local-skills", alias); err != nil {
		t.Fatal(err)
	}
	applyFeature(t, r, f, "on")
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../.other-skills", alias); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(r.root, ".other-skills/review/SKILL.md"), "unrelated skill", 0644)
	before := snapshot(t, r.root, global)
	code, _, diagnostics := runCLI(t, r.root, global, "off", "review")
	if code == 0 || !strings.Contains(diagnostics, "recorded") || !strings.Contains(diagnostics, "current") {
		t.Fatal(code, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("retargeted directory alias allowed files to change")
	}
}
