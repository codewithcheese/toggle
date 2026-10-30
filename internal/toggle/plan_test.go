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
	cfg := config{Stores: map[string]string{}}
	if !f.local {
		cfg.Stores[f.store] = filepath.Dir(f.dir)
	}
	return comparedPlan(t, r, action, f, cfg)
}

func seedOn(t *testing.T, r *repo, f feature) {
	t.Helper()
	p := mustPlan(t, r, "on", f)
	if err := p.apply(context.Background(), defaultOps()); err != nil {
		t.Fatal(err)
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
