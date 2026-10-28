package toggle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceTreeAndDestinations(t *testing.T) {
	r := testRepo(t)
	if err := r.metadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	f := testFeature(t, t.TempDir(), "review", ".agents/skills/review/SKILL.md", "skill")
	put(t, filepath.Join(f.dir, "README.md"), "metadata", 0644)
	put(t, filepath.Join(f.dir, "source", "run.sh"), "echo hi", 0751)
	files, err := sourceFiles(f)
	if err != nil || len(files) != 2 {
		t.Fatalf("sources: %v %v", files, err)
	}
	if files[1].relative != "run.sh" || files[1].content.exec != 0111 {
		t.Fatalf("mode/path: %+v", files[1])
	}
	for _, relative := range []string{"../outside", ".git/config", ".gitignore", "nested/.gitignore", "nested/.git", "nested/.git/config", ".toggle/other/source/a.md"} {
		if _, err := r.destination(relative); err == nil {
			t.Errorf("accepted %s", relative)
		}
	}
	os.Symlink(".toggle", filepath.Join(r.root, "definitions"))
	if _, err := r.destination("definitions/other.md"); err == nil {
		t.Fatal("accepted alias into .toggle")
	}
	os.Symlink(t.TempDir(), filepath.Join(r.root, "outside"))
	if _, err := r.destination("outside/skill.md"); err == nil {
		t.Fatal("accepted escaping symlink")
	}
	os.Symlink("local", filepath.Join(r.root, "alias"))
	resolved, err := r.destination("alias/skill.md")
	if err != nil || resolved != filepath.Join(r.root, "local", "skill.md") {
		t.Fatal(resolved, err)
	}
	os.Symlink(filepath.Join(t.TempDir(), "outside.md"), filepath.Join(f.dir, "source", "escape"))
	if _, err := sourceFiles(f); err == nil {
		t.Fatal("accepted source escape")
	}
}

func TestInternalSourceSymlinks(t *testing.T) {
	f := testFeature(t, t.TempDir(), "review", "original.md", "skill")
	os.Symlink("original.md", filepath.Join(f.dir, "source", "alias.md"))
	files, err := sourceFiles(f)
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	os.Symlink(".", filepath.Join(f.dir, "source", "cycle"))
	if _, err := sourceFiles(f); err == nil {
		t.Fatal("accepted cycle")
	}
}

func TestSymlinkParentTraversalUsesResolvedDirectory(t *testing.T) {
	r := testRepo(t)
	if err := r.metadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(r.root, "real/inner"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real/inner", filepath.Join(r.root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("link/../skill.md", filepath.Join(r.root, "alias.md")); err != nil {
		t.Fatal(err)
	}
	resolved, err := r.destination("alias.md")
	if err != nil || resolved != filepath.Join(r.root, "real/skill.md") {
		t.Fatal("lexical cleanup changed symlink semantics", resolved, err)
	}
	f := testFeature(t, t.TempDir(), "review", "alias.md", "skill")
	applyFeature(t, r, f, "on")
	if mustRead(t, filepath.Join(r.root, "alias.md")) != "skill" {
		t.Fatal("declared alias does not reach installed file")
	}
	applyFeature(t, r, f, "off")
	assertAbsent(t, filepath.Join(r.root, "real/skill.md"))

	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "inner"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(r.root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "inner"), filepath.Join(r.root, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.destination("alias.md"); err == nil {
		t.Fatal("accepted destination escaping through symlink and ..")
	}
}
