package toggle

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOffRemovesEmptyParentDirectories(t *testing.T) {
	for _, kind := range []string{"last-file", "shared-files", "unrelated-empty-directory", "missing-file"} {
		t.Run(kind, func(t *testing.T) {
			r := testRepo(t)
			f := testFeature(t, t.TempDir(), "review", ".agents/skills/review/SKILL.md", "skill")
			put(t, filepath.Join(f.dir, "source/.agents/skills/review/extra.md"), "extra", 0644)
			applyFeature(t, r, f, "on")
			switch kind {
			case "shared-files":
				put(t, filepath.Join(r.root, ".agents/skills/personal/SKILL.md"), "personal", 0644)
			case "unrelated-empty-directory":
				if err := os.Mkdir(filepath.Join(r.root, ".agents/skills/personal"), 0755); err != nil {
					t.Fatal(err)
				}
			case "missing-file":
				if err := os.Remove(filepath.Join(r.root, ".agents/skills/review/SKILL.md")); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "missing-file" {
				before := snapshot(t, r.root)
				if err := mustPlan(t, r, "off", f).apply(context.Background(), defaultOps()); err == nil {
					t.Fatal("partial presence was silently skipped")
				}
				if !reflect.DeepEqual(before, snapshot(t, r.root)) {
					t.Fatal("partial off changed directories or files")
				}
				// An agent can reconcile by completing the definition first.
				applyFeature(t, r, f, "on")
			}
			applyFeature(t, r, f, "off")
			assertAbsent(t, filepath.Join(r.root, ".agents/skills/review"))
			if kind == "last-file" || kind == "missing-file" {
				assertAbsent(t, filepath.Join(r.root, ".agents"))
			} else if info, err := os.Stat(filepath.Join(r.root, ".agents/skills/personal")); err != nil || !info.IsDir() {
				t.Fatal("unrelated directory removed", info, err)
			}
			if _, err := os.Stat(r.root); err != nil {
				t.Fatal("repo root removed", err)
			}
			if kind == "shared-files" && mustRead(t, filepath.Join(r.root, ".agents/skills/personal/SKILL.md")) != "personal" {
				t.Fatal("unrelated file changed")
			}
		})
	}
}

func TestDirectoryCleanupStopsAtDirectorySymlinks(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "alias/review/SKILL.md", "skill")
	alias := filepath.Join(r.root, "alias")
	if err := os.Symlink("local", alias); err != nil {
		t.Fatal(err)
	}
	applyFeature(t, r, f, "on")
	applyFeature(t, r, f, "off")
	assertAbsent(t, filepath.Join(r.root, "local/review"))
	if target, err := os.Readlink(alias); err != nil || target != "local" {
		t.Fatal("directory symlink changed", target, err)
	}
	if info, err := os.Stat(alias); err != nil || !info.IsDir() {
		t.Fatal("directory symlink was left dangling", info, err)
	}
}

func TestOffPreviewIncludesDirectoryCleanupWithoutWrites(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	f := testFeature(t, global, "review", ".agents/skills/review/SKILL.md", "skill")
	applyFeature(t, r, f, "on")
	before := snapshot(t, r.root, global)
	code, out, diagnostics := runCLI(t, r.root, global, "off", "review", "--dry-run")
	if code != 0 || !strings.Contains(out, "remove empty directory\t.agents/skills/review") || !strings.Contains(out, "remove empty directory\t.agents\n") {
		t.Fatal(code, out, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
		t.Fatal("directory cleanup preview changed files")
	}
}

func TestDirectoryCleanupRetainsNewUnrelatedEntries(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "skills/review/SKILL.md", "skill")
	applyFeature(t, r, f, "on")
	p := mustPlan(t, r, "off", f)
	ops := defaultOps()
	unrelated := filepath.Join(r.root, "skills/review/personal.md")
	ops.remove = func(path string) error {
		if err := os.Remove(path); err != nil {
			return err
		}
		put(t, unrelated, "new user file", 0644)
		return nil
	}
	if err := p.apply(context.Background(), ops); err != nil {
		t.Fatal(err)
	}
	if mustRead(t, unrelated) != "new user file" {
		t.Fatal("new unrelated entry removed")
	}
}

func TestDirectoryCleanupCannotFollowReplacementSymlink(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "skills/review/SKILL.md", "skill")
	applyFeature(t, r, f, "on")
	p := mustPlan(t, r, "off", f)
	outside := t.TempDir()
	put(t, filepath.Join(outside, "personal.md"), "unrelated", 0644)
	ops := defaultOps()
	ops.remove = func(path string) error {
		if err := os.Remove(path); err != nil {
			return err
		}
		if err := os.Remove(filepath.Dir(path)); err != nil {
			return err
		}
		return os.Symlink(outside, filepath.Dir(path))
	}
	if err := p.apply(context.Background(), ops); err == nil || !strings.Contains(err.Error(), "directory redirected") {
		t.Fatal("replacement symlink accepted", err)
	}
	if _, err := os.Readlink(filepath.Join(r.root, "skills/review")); err != nil {
		t.Fatal("replacement symlink removed", err)
	}
	if mustRead(t, filepath.Join(outside, "personal.md")) != "unrelated" {
		t.Fatal("outside file changed")
	}
}

func TestDirectoryRemovalFailureKeepsExclusions(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "skills/review/SKILL.md", "skill")
	applyFeature(t, r, f, "on")
	p := mustPlan(t, r, "off", f)
	ops := defaultOps()
	ops.rmdir = func(string) error { return os.ErrPermission }
	if err := p.apply(context.Background(), ops); err == nil || !strings.Contains(err.Error(), "prune empty directory") {
		t.Fatal("directory removal failure was not reported", err)
	}
	assertAbsent(t, filepath.Join(r.root, "skills/review/SKILL.md"))
	if mustRead(t, r.excludePath) != string(p.oldExclude) {
		t.Fatal("directory removal failure cleared exclusions")
	}
	assertAbsent(t, filepath.Join(r.gitDir, "toggle.json"))
}
