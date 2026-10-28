package toggle

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func snapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	state := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			var data []byte
			if info.Mode().IsRegular() {
				data, err = os.ReadFile(path)
			} else if info.Mode()&os.ModeSymlink != 0 {
				var target string
				target, err = os.Readlink(path)
				data = []byte(target)
			}
			state[path] = fmt.Sprintf("%v:%s", info.Mode(), data)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return state
}

func mustPlan(t *testing.T, r *repo, action string, f feature) *plan {
	t.Helper()
	if err := r.metadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, err := prepare(context.Background(), r, action, f.name, map[string]feature{f.name: f})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// seedOn establishes a successful installation for read-only off preparation.
func seedOn(t *testing.T, r *repo, f feature) {
	t.Helper()
	p := mustPlan(t, r, "on", f)
	if err := p.err(); err != nil {
		t.Fatal(err)
	}
	put(t, r.excludePath, string(p.newExclude), 0644)
	put(t, r.recordPath, string(p.newRecord), 0600)
	for _, change := range p.changes {
		put(t, filepath.Join(r.root, change.file.Resolved), string(change.desired.data), 0644|fs.FileMode(change.desired.exec))
	}
}

func TestOnPreflight(t *testing.T) {
	for _, kind := range []string{"absent", "matching", "different", "mode", "tracked", "directory", "override"} {
		t.Run(kind, func(t *testing.T) {
			r := testRepo(t)
			f := testFeature(t, t.TempDir(), "review", "instructions.md", "source")
			target := filepath.Join(r.root, "instructions.md")
			switch kind {
			case "matching", "tracked":
				put(t, target, "source", 0644)
			case "different":
				put(t, target, "user edit", 0644)
			case "mode":
				put(t, target, "source", 0755)
			case "directory":
				os.Mkdir(target, 0755)
			case "override":
				put(t, filepath.Join(r.root, ".gitignore"), "!/instructions.md\n", 0644)
			}
			if kind == "tracked" {
				testGit(t, r.root, "add", "instructions.md")
			}
			before := snapshot(t, r.root, f.dir)
			p := mustPlan(t, r, "on", f)
			wantConflict := kind != "absent" && kind != "matching"
			if (p.err() != nil) != wantConflict {
				t.Fatalf("%s: %v", kind, p.err())
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
				t.Fatal("preparation changed files")
			}
			if wantConflict && !strings.Contains(p.err().Error(), "instructions.md") {
				t.Fatal(p.err())
			}
		})
	}
}

func TestDestinationOwnership(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "checks", "instructions.md", "same")
	seedOn(t, r, f)
	review := testFeature(t, t.TempDir(), "review", "alias.md", "same")
	os.Symlink("instructions.md", filepath.Join(r.root, "alias.md"))
	p := mustPlan(t, r, "on", review)
	if p.err() == nil || !strings.Contains(p.err().Error(), "checks") {
		t.Fatal("accepted shared destination", p.err())
	}
	other := testFeature(t, t.TempDir(), "duplicate", "one.md", "same")
	put(t, filepath.Join(other.dir, "source", "two.md"), "same", 0644)
	os.Symlink("shared.md", filepath.Join(r.root, "one.md"))
	os.Symlink("shared.md", filepath.Join(r.root, "two.md"))
	if p := mustPlan(t, r, "on", other); p.err() == nil {
		t.Fatal("accepted two destinations resolving to the same file")
	}
}

func TestOffPreflight(t *testing.T) {
	for _, kind := range []string{"unchanged", "edit", "mode", "preserved-global", "preserved-local", "missing", "source-drift", "new-source-path", "tracked", "redirect"} {
		t.Run(kind, func(t *testing.T) {
			r := testRepo(t)
			f := testFeature(t, t.TempDir(), "review", "instructions.md", "source")
			seedOn(t, r, f)
			target := filepath.Join(r.root, "instructions.md")
			source := filepath.Join(f.dir, "source", "instructions.md")
			switch kind {
			case "edit", "preserved-global", "preserved-local":
				put(t, target, "edited", 0755)
			case "mode":
				os.Chmod(target, 0755)
			case "missing":
				os.Remove(target)
			case "source-drift":
				put(t, source, "changed global", 0644)
			case "new-source-path":
				os.Remove(source)
				put(t, filepath.Join(f.dir, "source", "new.md"), "new", 0644)
				put(t, filepath.Join(r.root, "new.md"), "unrelated", 0644)
			case "tracked":
				testGit(t, r.root, "add", "-f", "instructions.md")
			case "redirect":
				os.Rename(target, filepath.Join(r.root, "original.md"))
				put(t, filepath.Join(r.root, "other.md"), "unrelated", 0644)
				os.Symlink("other.md", target)
			}
			if kind == "preserved-global" {
				put(t, source, "edited", 0755)
			}
			if kind == "preserved-local" {
				f = testFeature(t, filepath.Join(r.root, ".toggle"), "review", "instructions.md", "edited")
				os.Chmod(filepath.Join(f.dir, "source", "instructions.md"), 0755)
			}
			before := snapshot(t, r.root, f.dir)
			p := mustPlan(t, r, "off", f)
			wantConflict := kind == "edit" || kind == "mode" || kind == "tracked" || kind == "redirect"
			if (p.err() != nil) != wantConflict {
				t.Fatalf("%s: %v", kind, p.err())
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
				t.Fatal("off preparation changed files")
			}
		})
	}
}

func TestNoopAndMissingDefinition(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "instructions.md", "source")
	if p := mustPlan(t, r, "off", f); !p.noop {
		t.Fatal("off not idempotent")
	}
	seedOn(t, r, f)
	put(t, filepath.Join(r.root, "instructions.md"), "edit", 0644)
	if p := mustPlan(t, r, "on", f); !p.noop || p.err() != nil {
		t.Fatal("on tried to replace edit")
	}
	p, err := prepare(context.Background(), r, "off", f.name, nil)
	if err != nil || p.err() == nil || !strings.Contains(p.err().Error(), "manual cleanup") {
		t.Fatal(p, err)
	}
}

func TestTrackedFileAliasConflictsBeforeExclusions(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "alias.md", "same content")
	put(t, filepath.Join(r.root, "resolved.md"), "same content", 0644)
	os.Symlink("resolved.md", filepath.Join(r.root, "alias.md"))
	testGit(t, r.root, "add", "alias.md")
	before := snapshot(t, r.root, f.dir)
	p := mustPlan(t, r, "on", f)
	if p.err() == nil || !strings.Contains(p.err().Error(), "tracked by Git") || !strings.Contains(p.err().Error(), "alias.md") {
		t.Fatal("accepted a tracked declared alias", p.err())
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
		t.Fatal("tracked-alias conflict changed files")
	}
}
