package toggle

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func comparedPlan(t *testing.T, r *repo, action string, f feature, cfg config) *plan {
	t.Helper()
	ctx := context.Background()
	if err := r.metadata(ctx); err != nil {
		t.Fatal(err)
	}
	protected, err := protectedDefinitions(cfg, r, []feature{f})
	if err != nil {
		t.Fatal(err)
	}
	view, err := newView(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	p, err := prepareDefinition(ctx, r, action, f, protected, view, false)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCurrentComparisonRules(t *testing.T) {
	for _, state := range []string{"absent", "complete", "partial", "different", "mode", "directory", "tracked"} {
		t.Run(state, func(t *testing.T) {
			r := testRepo(t)
			f := testFeature(t, t.TempDir(), "review", "first.md", "first")
			f.store = "team"
			put(t, filepath.Join(f.dir, "source/second.md"), "second", 0644)
			if state != "absent" {
				put(t, filepath.Join(r.root, "first.md"), "first", 0644)
			}
			if state != "absent" && state != "partial" {
				put(t, filepath.Join(r.root, "second.md"), "second", 0644)
			}
			switch state {
			case "different":
				put(t, filepath.Join(r.root, "second.md"), "edited", 0644)
			case "mode":
				os.Chmod(filepath.Join(r.root, "second.md"), 0755)
			case "directory":
				os.Remove(filepath.Join(r.root, "second.md"))
				os.Mkdir(filepath.Join(r.root, "second.md"), 0755)
			case "tracked":
				testGit(t, r.root, "add", "first.md")
			}
			before := snapshot(t, r.root, f.dir)
			for _, action := range []string{"on", "off"} {
				p := comparedPlan(t, r, action, f, config{})
				wantConflict := state != "absent" && state != "complete" && !(state == "partial" && action == "on")
				if (p.err() != nil) != wantConflict {
					t.Fatalf("%s/%s: %v", state, action, p.err())
				}
				if state == "partial" && action == "off" && !strings.Contains(p.err().Error(), "partial presence") {
					t.Fatal(p.err())
				}
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
				t.Fatal("comparison made persistent changes")
			}
		})
	}
}

func TestComparisonViewsCacheContentsAndGitIndex(t *testing.T) {
	ctx := context.Background()
	r := testRepo(t)
	r.metadata(ctx)
	queries := map[string]int{}
	r.query = func(ctx context.Context, cwd string, data []byte, args ...string) ([]byte, error) {
		queries[args[0]]++
		return git(ctx, cwd, data, args...)
	}
	f := testFeature(t, t.TempDir(), "review", "instructions.md", "instructions")
	put(t, filepath.Join(r.root, "instructions.md"), "instructions", 0644)
	view, err := newView(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	reads := map[string]int{}
	view.readBytes = func(path string) ([]byte, error) { reads[path]++; return os.ReadFile(path) }
	for i := 0; i < 3; i++ {
		p, err := prepareDefinition(ctx, r, "off", f, nil, view, true)
		if err != nil || p.err() != nil || p.matching != 1 {
			t.Fatal(p, err)
		}
	}
	other := testFeature(t, t.TempDir(), "other", "instructions.md", "instructions")
	if p, err := prepareDefinition(ctx, r, "off", other, nil, view, true); err != nil || p.err() != nil || p.matching != 1 {
		t.Fatal("shared destination comparison failed", p, err)
	}
	sourcePath, err := resolvePath(filepath.Join(f.dir, "source/instructions.md"))
	if err != nil {
		t.Fatal(err)
	}
	if queries["ls-files"] != 1 || reads[filepath.Join(r.root, "instructions.md")] != 1 || reads[sourcePath] != 1 {
		t.Fatal("repeated hashing or full-index queries", queries, reads)
	}

	missing := testFeature(t, t.TempDir(), "absent", "missing.md", "source")
	p, err := prepareDefinition(ctx, r, "off", missing, nil, view, true)
	missingSource, pathErr := resolvePath(filepath.Join(missing.dir, "source/missing.md"))
	if err != nil || pathErr != nil || p.present != 0 || reads[missingSource] != 0 {
		t.Fatal("all-absent scan read source contents", p, err, reads)
	}
}

func TestAllCoLocatedDefinitionsAreProtected(t *testing.T) {
	r := testRepo(t)
	store := filepath.Join(r.root, "catalog")
	checks := testFeature(t, store, "checks", "config.md", "same")
	put(t, filepath.Join(store, "README.md"), "store metadata", 0644)
	f := testFeature(t, filepath.Join(r.root, ".toggle"), "review", "catalog/checks/source/config.md", "same")
	f.store = "research"
	cfg := config{Stores: map[string]string{"team": store, "research": filepath.Dir(f.dir), "unavailable": filepath.Join(t.TempDir(), "missing")}}
	for _, action := range []string{"on", "off"} {
		p := comparedPlan(t, r, action, f, cfg)
		if p.err() == nil || !strings.Contains(p.err().Error(), "protected feature definition") {
			t.Fatal("unselected source was not protected", p.err())
		}
	}
	other := testFeature(t, store, "review", "payload.md", "same")
	other.store = "team"
	os.Symlink("catalog/checks/source/config.md", filepath.Join(r.root, "payload.md"))
	if p := comparedPlan(t, r, "off", other, cfg); p.err() == nil || !strings.Contains(p.err().Error(), checks.dir) {
		t.Fatal("source alias accepted", p.err())
	}
}

func TestComparisonIgnoresHistoryAndAllowsMatchingFork(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	f := testFeature(t, global, "review", "instructions.md", "old version")
	put(t, filepath.Join(r.root, "instructions.md"), "edited version", 0755)
	put(t, filepath.Join(r.root, ".git/toggle.json"), "invalid obsolete record", 0600)
	if p := comparedPlan(t, r, "off", f, config{}); p.err() == nil {
		t.Fatal("source mismatch accepted")
	}
	fork := testFeature(t, filepath.Join(r.root, ".toggle"), "review", "instructions.md", "edited version")
	fork.store = "research"
	os.Chmod(filepath.Join(fork.dir, "source/instructions.md"), 0755)
	cfg := config{Stores: map[string]string{"research": filepath.Dir(fork.dir)}}
	for _, action := range []string{"on", "off"} {
		if p := comparedPlan(t, r, action, fork, cfg); p.err() != nil {
			t.Fatal("matching configured fork required a handoff", p.err())
		}
	}
}

func TestDuplicateResolvedDestinationsConflictBeforeChanges(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "one.md", "same")
	put(t, filepath.Join(f.dir, "source/two.md"), "same", 0644)
	put(t, filepath.Join(r.root, "shared.md"), "same", 0644)
	for _, name := range []string{"one.md", "two.md"} {
		if err := os.Symlink("shared.md", filepath.Join(r.root, name)); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(t, r.root, f.dir)
	for _, action := range []string{"on", "off"} {
		p := mustPlan(t, r, action, f)
		if err := p.apply(context.Background(), defaultOps()); err == nil || !strings.Contains(err.Error(), "overlapping resolved destination") || !strings.Contains(err.Error(), "one.md") || !strings.Contains(err.Error(), "two.md") {
			t.Fatal("duplicate aliases did not conflict", action, err)
		}
		if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
			t.Fatal("duplicate destination conflict changed files")
		}
	}
}
