package toggle

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedPathBlock(t *testing.T) {
	for _, user := range []string{"", "# user rules\n/user\n", "# user rules\n/user", "\n"} {
		paths := []string{"z.md", "a [b]*?# !.md", "back\\slash.md", "trailing "}
		on, err := updateExclusions([]byte(user), paths, "on")
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseExclusions(on)
		if err != nil || len(parsed.rules) != len(paths) {
			t.Fatal(parsed, err)
		}
		again, err := updateExclusions(on, paths, "on")
		if err != nil || !bytes.Equal(on, again) {
			t.Fatal("repeated union changed block", err)
		}
		off, err := updateExclusions(on, paths, "off")
		if err != nil || string(off) != user {
			t.Fatalf("outside text changed: %q want %q: %v", off, user, err)
		}
	}
}

func TestManagedPathSetHasNoOwnership(t *testing.T) {
	base := []byte("# user rule\n/skill.md\n")
	on, _ := updateExclusions(base, []string{"skill.md", "other.md"}, "on")
	again, _ := updateExclusions(on, []string{"skill.md"}, "on")
	if !bytes.Equal(on, again) || strings.Count(string(on), "/skill.md\n") != 2 {
		t.Fatal("path wasn't deduplicated independently of user rule", string(on))
	}
	off, err := updateExclusions(on, []string{"skill.md"}, "off")
	if err != nil || !bytes.HasPrefix(off, base) || !bytes.Contains(off, []byte("/other.md\n")) {
		t.Fatal("unrelated paths/rules changed", string(off), err)
	}
}

func TestMalformedPathBlock(t *testing.T) {
	for _, data := range []string{pathBlockStart, pathBlockEnd, pathBlockStart + "/file\n", pathBlockStart + "*.md\n" + pathBlockEnd, pathBlockStart + "/wild*.md\n" + pathBlockEnd, pathBlockStart + "/../outside\n" + pathBlockEnd, pathBlockStart + "/file\n" + pathBlockEnd + pathBlockStart + pathBlockEnd} {
		if _, err := updateExclusions([]byte(data), []string{"new.md"}, "on"); err == nil {
			t.Fatal("malformed block accepted", data)
		}
	}
	duplicates := pathBlockStart + "/file.md\n/file.md\n" + pathBlockEnd
	updated, err := updateExclusions([]byte(duplicates), []string{"file.md"}, "on")
	if err != nil || strings.Count(string(updated), "/file.md\n") != 1 {
		t.Fatal("duplicate rules not normalized", string(updated), err)
	}
}

func TestPathBlockUsesGitMetadataAndEffectiveIgnoring(t *testing.T) {
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
	paths := []string{"plain.md", "a [b]*?# !.md", "back\\slash.md", "trailing "}
	old, err := readOptional(r.excludePath)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := updateExclusions(old, paths, "on")
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(r.excludePath, updated); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		match, err := r.ignore(ctx, path)
		if err != nil || match.pattern == "" {
			t.Fatal("Git did not ignore literal path", path, match, err)
		}
	}
	put(t, filepath.Join(r.root, ".gitignore"), "!/plain.md\n", 0644)
	blocked, err := r.allowExclusions(ctx, paths)
	if err != nil || len(blocked) != 1 || blocked["plain.md"] == "" {
		t.Fatal("batched negation detection", blocked, err)
	}

	// Atomic replacement errors leave the destination directory intact.
	bad := filepath.Join(r.root, "not-a-document")
	put(t, filepath.Join(bad, "personal.md"), "user data", 0644)
	if err := atomicWrite(bad, []byte("replacement")); err == nil {
		t.Fatal("replaced a directory")
	}
	if data, err := os.ReadFile(filepath.Join(bad, "personal.md")); err != nil || string(data) != "user data" {
		t.Fatal("atomic error changed existing data", string(data), err)
	}
}
