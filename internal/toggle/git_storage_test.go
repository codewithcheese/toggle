package toggle

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGitTrackingAndExclusionPreflight(t *testing.T) {
	ctx := context.Background()
	r := testRepo(t)
	r.metadata(ctx)
	path := filepath.Join(r.root, "skill.md")
	put(t, path, "skill", 0644)
	testGit(t, r.root, "add", "skill.md")
	tracked, err := r.tracked(ctx, path)
	if err != nil || tracked != path {
		t.Fatal(tracked, err)
	}
	put(t, filepath.Join(r.root, ".gitignore"), "!/skill.md\n", 0644)
	if err := r.exclusionAllowed(ctx, "skill.md"); err == nil {
		t.Fatal("accepted overriding negation")
	}
	os.Remove(filepath.Join(r.root, ".gitignore"))
	put(t, r.excludePath, "!/skill.md\n", 0644)
	if err := r.exclusionAllowed(ctx, "skill.md"); err != nil {
		t.Fatalf("new appended rule should override old info/exclude: %v", err)
	}

	for _, name := range []string{"plain.md", "a [b]*?# !.md", "back\\slash.md", "trailing "} {
		pattern, err := exactExclusion(name)
		if err != nil {
			t.Fatal(err)
		}
		put(t, r.excludePath, pattern+"\n", 0644)
		match, err := r.ignore(ctx, name)
		if err != nil || match.pattern != pattern {
			t.Fatalf("exact exclusion %q: %+v %v", name, match, err)
		}
		other, err := r.ignore(ctx, "other-"+name)
		if err != nil || other.pattern != "" {
			t.Fatalf("overbroad exclusion %q: %+v %v", name, other, err)
		}
	}
}

func TestSeparateGitDirectoryAndRecords(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	metadata := filepath.Join(t.TempDir(), "gitdata")
	testGit(t, dir, "init", "-q", "--separate-git-dir", metadata)
	r, err := findRepo(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.metadata(ctx); err != nil {
		t.Fatal(err)
	}
	want, _ := resolvePath(metadata)
	actual, _ := resolvePath(filepath.Dir(r.recordPath))
	if actual != want {
		t.Fatal(actual, want)
	}
	state, original, err := loadRecords(r.recordPath)
	if err != nil || original != nil || len(state.Features) != 0 {
		t.Fatal(state, err)
	}
	if _, err := os.Stat(r.recordPath); !os.IsNotExist(err) {
		t.Fatal("read created record")
	}
	state.Features["review"] = installation{Source: "example", Files: []installedFile{{Relative: "skill.md", Declared: "skill.md", Resolved: "skill.md", Hash: "hash", Executable: 0111}}}
	data, _ := recordBytes(state)
	if err := atomicWrite(r.recordPath, data); err != nil {
		t.Fatal(err)
	}
	loaded, data2, err := loadRecords(r.recordPath)
	if err != nil || !bytes.Equal(data, data2) || loaded.Features["review"].Files[0].Executable != 0111 {
		t.Fatal(loaded, err)
	}
	other := testRepo(t)
	other.metadata(ctx)
	separate, _, _ := loadRecords(other.recordPath)
	if len(separate.Features) != 0 {
		t.Fatal("records leaked between repos")
	}
}

func TestExclusionOwnership(t *testing.T) {
	user := []byte("# user rules\nuser-file")
	other := exclusionBlock("checks", []string{"/checks.md"})
	review := exclusionBlock("Review Notes", []string{"/review.md"})
	before := append(bytes.Clone(user), other...)
	after, err := addExclusion(before, "Review Notes", review)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := removeExclusion(after, "Review Notes", review)
	if err != nil || !bytes.Equal(restored, before) {
		t.Fatalf("rules changed: %q %v", restored, err)
	}
	if _, err := removeExclusion(bytes.ReplaceAll(after, []byte("/review.md"), []byte("/user-edit.md")), "Review Notes", review); err == nil {
		t.Fatal("removed modified block")
	}
}
