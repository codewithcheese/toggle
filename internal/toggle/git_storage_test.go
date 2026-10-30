package toggle

import (
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
	view, err := newView(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	tracked := view.trackedPath(path)
	if err != nil || tracked != path {
		t.Fatal(tracked, err)
	}
	put(t, filepath.Join(r.root, ".gitignore"), "!/skill.md\n", 0644)
	if blocked, err := r.allowExclusions(ctx, []string{"skill.md"}); err != nil || blocked["skill.md"] == "" {
		t.Fatal("accepted overriding negation")
	}
	os.Remove(filepath.Join(r.root, ".gitignore"))
	put(t, r.excludePath, "!/skill.md\n", 0644)
	if blocked, err := r.allowExclusions(ctx, []string{"skill.md"}); err != nil || len(blocked) != 0 {
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
