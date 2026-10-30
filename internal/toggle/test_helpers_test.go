package toggle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git(context.Background(), dir, nil, args...)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func testRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	testGit(t, dir, "init", "-q")
	r, err := findRepo(context.Background(), dir)
	if err != nil || r == nil {
		t.Fatalf("repo: %v", err)
	}
	return r
}

func put(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func testFeature(t *testing.T, root, name, relative, content string) feature {
	t.Helper()
	f := feature{name: name, dir: filepath.Join(root, name), store: "team", local: filepath.Base(root) == ".toggle"}
	put(t, filepath.Join(f.dir, "source", relative), content, 0644)
	return f
}

func applyFeature(t *testing.T, r *repo, f feature, action string) *plan {
	t.Helper()
	p := mustPlan(t, r, action, f)
	if err := p.apply(context.Background(), defaultOps()); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("expected absent %s: %v", path, err)
	}
}
