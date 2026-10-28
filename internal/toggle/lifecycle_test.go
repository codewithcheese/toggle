package toggle

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCLILifecycle(t *testing.T) {
	r := testRepo(t)
	if err := r.metadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	global := t.TempDir()
	relative := ".agents/skills/review/SKILL.md"
	f := testFeature(t, global, "review", relative, "global skill")
	outside := t.TempDir()
	code, out, diagnostics := runCLI(t, outside, global, "list")
	if code != 0 || !strings.Contains(out, f.dir) {
		t.Fatal(code, out, diagnostics)
	}
	target := filepath.Join(r.root, relative)
	put(t, target, "global skill", 0644)
	if code, _, diagnostics := runCLI(t, r.root, global, "on", "review"); code != 0 {
		t.Fatal(diagnostics)
	}
	testGit(t, r.root, "add", ".")
	if status := testGit(t, r.root, "status", "--porcelain"); status != "" {
		t.Fatal("installed files were staged", status)
	}
	put(t, target, "repo-specific skill edit", 0755)
	before := snapshot(t, r.root, global)
	for _, args := range [][]string{{"off", "review"}, {"off", "review", "--dry-run"}} {
		code, _, diagnostics := runCLI(t, r.root, global, args...)
		if code == 0 || !strings.Contains(diagnostics, "not preserved") {
			t.Fatal(code, diagnostics)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("blocked commands changed state")
	}
	local := testFeature(t, filepath.Join(r.root, ".toggle"), "review", relative, "repo-specific skill edit")
	os.Chmod(filepath.Join(local.dir, "source", relative), 0755)
	before = snapshot(t, r.root, global)
	if code, _, diagnostics := runCLI(t, r.root, global, "on", "review"); code != 0 {
		t.Fatal(diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("repeated on switched to the new local definition")
	}
	code, out, diagnostics = runCLI(t, r.root, global, "list")
	if code != 0 || !strings.Contains(out, local.dir) || strings.Contains(out, f.dir) {
		t.Fatal("local selection", code, out, diagnostics)
	}
	_, out, _ = runCLI(t, r.root, global, "status")
	if !strings.Contains(out, "review\tmodified") {
		t.Fatal(out)
	}
	match, _ := r.ignore(context.Background(), ".toggle/review/source/"+relative)
	if match.pattern != "" {
		t.Fatal("toggle hid the local definition", match)
	}
	// The owner chooses an ordinary Git ignore rule for their local definitions.
	userExclusions := mustRead(t, r.excludePath) + "\n/.toggle/\n"
	put(t, r.excludePath, userExclusions, 0644)
	before = snapshot(t, r.root, global)
	if code, _, diagnostics := runCLI(t, r.root, global, "off", "review", "--dry-run"); code != 0 {
		t.Fatal(diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("off preview wrote files")
	}
	if code, _, diagnostics := runCLI(t, r.root, global, "off", "review"); code != 0 {
		t.Fatal(diagnostics)
	}
	assertAbsent(t, target)
	if !strings.Contains(mustRead(t, r.excludePath), "/.toggle/") {
		t.Fatal("owner ignore rule removed")
	}
	for cycle := 0; cycle < 2; cycle++ {
		if code, _, diagnostics := runCLI(t, r.root, global, "off", "review"); code != 0 {
			t.Fatal(diagnostics)
		}
		if code, _, diagnostics := runCLI(t, r.root, global, "on", "review"); code != 0 {
			t.Fatal(diagnostics)
		}
		if mustRead(t, target) != "repo-specific skill edit" {
			t.Fatal("saved edit was not installed")
		}
		info, _ := os.Stat(target)
		if info.Mode().Perm()&0111 != 0111 {
			t.Fatal("saved executable bits were lost")
		}
		before = snapshot(t, r.root, global)
		if code, _, diagnostics := runCLI(t, r.root, global, "on", "review"); code != 0 {
			t.Fatal(diagnostics)
		}
		if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
			t.Fatal("repeated on changed files")
		}
		if code, _, diagnostics := runCLI(t, r.root, global, "off", "review"); code != 0 {
			t.Fatal(diagnostics)
		}
	}
	if mustRead(t, filepath.Join(f.dir, "source", relative)) != "global skill" || mustRead(t, filepath.Join(local.dir, "source", relative)) != "repo-specific skill edit" {
		t.Fatal("toggle rewrote source definitions")
	}
	testGit(t, r.root, "add", ".")
	if status := testGit(t, r.root, "status", "--porcelain"); status != "" {
		t.Fatal(status)
	}
}

func TestCommittedLocalDefinitionAndGlobalFallback(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	testFeature(t, global, "review", "instructions.md", "global")
	local := testFeature(t, filepath.Join(r.root, ".toggle"), "review", "instructions.md", "local")
	testGit(t, r.root, "add", ".toggle")
	testGit(t, r.root, "-c", "user.name=Toggle Tests", "-c", "user.email=toggle-tests@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-qm", "local definition fixture")
	before := snapshot(t, local.dir)
	for _, action := range []string{"on", "off"} {
		if code, _, diagnostics := runCLI(t, r.root, global, action, "review"); code != 0 {
			t.Fatal(diagnostics)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, local.dir)) || testGit(t, r.root, "status", "--porcelain") != "" {
		t.Fatal("committed definition or index changed")
	}
	if code, _, diagnostics := runCLI(t, r.root, global, "on", "review"); code != 0 {
		t.Fatal(diagnostics)
	}
	os.RemoveAll(local.dir)
	if code, _, diagnostics := runCLI(t, r.root, global, "off", "review"); code != 0 {
		t.Fatal("global fallback did not allow cleanup", diagnostics)
	}
	assertAbsent(t, filepath.Join(r.root, "instructions.md"))
}
