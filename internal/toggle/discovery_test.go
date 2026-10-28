package toggle

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoLookup(t *testing.T) {
	r := testRepo(t)
	sub := filepath.Join(r.root, "docs")
	os.Mkdir(sub, 0755)
	for _, dir := range []string{r.root, sub} {
		found, err := findRepo(context.Background(), dir)
		if err != nil || found == nil || found.root != r.root {
			t.Fatalf("%s: %v %v", dir, found, err)
		}
	}
	if err := r.requireRoot(sub); err == nil {
		t.Fatal("accepted subdirectory")
	}
	dir := t.TempDir()
	found, err := findRepo(context.Background(), dir)
	entries, _ := os.ReadDir(dir)
	if err != nil || found != nil || len(entries) != 0 {
		t.Fatalf("outside: %v %v %v", found, err, entries)
	}
}

func TestDiscoveryAndList(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	g := testFeature(t, global, "Review Notes", "instructions.md", "global")
	testFeature(t, global, "openspec", "skill.md", "global")
	testFeature(t, filepath.Join(global, "group"), "nested", "skill.md", "nested")
	l := testFeature(t, filepath.Join(r.root, ".toggle"), g.name, "instructions.md", "local")
	testFeature(t, filepath.Join(r.root, ".toggle"), "checks", "checks.md", "local only")
	for _, inside := range []bool{false, true} {
		var target *repo
		if inside {
			target = r
		}
		features, err := discover(global, target)
		wantCount := 2
		if inside {
			wantCount = 3
		}
		if err != nil || len(features) != wantCount {
			t.Fatalf("discovery: %v %v", features, err)
		}
		want := g.dir
		if inside {
			want = l.dir
		}
		if features[g.name].dir != want {
			t.Fatalf("selected %s, want %s", features[g.name].dir, want)
		}
	}
	os.RemoveAll(filepath.Join(r.root, ".toggle"))
	features, err := discover(global, r)
	if err != nil || features[g.name].dir != g.dir {
		t.Fatalf("fallback: %v %v", features, err)
	}
	missing := filepath.Join(t.TempDir(), "missing")
	empty, err := discover(missing, nil)
	if err != nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatal("discovery created files")
	}
	var out, errOut bytes.Buffer
	a := app{cwd: t.TempDir(), globalRoot: global, out: &out, errOut: &errOut}
	if code := a.run(context.Background(), []string{"list"}); code != 0 || !strings.Contains(out.String(), g.dir) {
		t.Fatalf("list: %d %s %s", code, &out, &errOut)
	}
}
