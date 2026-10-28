package toggle

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReservedGitTargets(t *testing.T) {
	for _, name := range []string{".gitignore", "config/.gitignore", "vendor/.git", "vendor/.git/config", "ignore-alias", "git-alias/config", "git-pointer-alias", "declared-ignore-alias"} {
		t.Run(name, func(t *testing.T) {
			r := testRepo(t)
			global := t.TempDir()
			relative := name
			want := ".git"
			if strings.Contains(name, "ignore") {
				want = ".gitignore"
			}
			if name == "declared-ignore-alias" {
				relative = ".gitignore"
				put(t, filepath.Join(r.root, "ordinary.ignore"), "same", 0644)
				if err := os.Symlink("ordinary.ignore", filepath.Join(r.root, relative)); err != nil {
					t.Fatal(err)
				}
			}
			f := testFeature(t, global, "review", relative, "same")
			put(t, filepath.Join(f.dir, "source/instructions.md"), "ordinary skill", 0644)
			switch name {
			case ".gitignore":
				put(t, filepath.Join(f.dir, "source/.gitignore"), "!/instructions.md\n", 0644)
			case "ignore-alias":
				put(t, filepath.Join(r.root, "config/.gitignore"), "same", 0644)
				if err := os.Symlink("config/.gitignore", filepath.Join(r.root, name)); err != nil {
					t.Fatal(err)
				}
			case "git-alias/config":
				put(t, filepath.Join(r.root, "vendor/.git/config"), "same", 0644)
				if err := os.Symlink("vendor/.git", filepath.Join(r.root, "git-alias")); err != nil {
					t.Fatal(err)
				}
			case "git-pointer-alias":
				put(t, filepath.Join(r.root, "vendor/.git"), "same", 0644)
				if err := os.Symlink("vendor/.git", filepath.Join(r.root, name)); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, r.root, global)
			for _, args := range [][]string{{"on", "review"}, {"on", "review", "--dry-run"}} {
				code, _, diagnostics := runCLI(t, r.root, global, args...)
				if code == 0 || !strings.Contains(diagnostics, "reserved "+want+" destination") {
					t.Fatal(code, diagnostics)
				}
				if !reflect.DeepEqual(before, snapshot(t, r.root, global)) {
					t.Fatal("reserved-path conflict changed files")
				}
			}
		})
	}
}

func TestTopLevelIgnoreMetadataIsNotInstalled(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "instructions.md", "ordinary skill")
	put(t, filepath.Join(f.dir, ".gitignore"), "!/instructions.md\n", 0644)
	before := snapshot(t, f.dir)
	applyFeature(t, r, f, "on")
	assertAbsent(t, filepath.Join(r.root, ".gitignore"))
	if !reflect.DeepEqual(before, snapshot(t, f.dir)) {
		t.Fatal("top-level feature metadata changed")
	}
}

func TestFilesystemCaseAliasesCannotBypassReservedPaths(t *testing.T) {
	r := testRepo(t)
	for _, relative := range []string{".GIT/config", "nested/.GIT/config", "nested/.GITIGNORE", ".TOGGLE/other/source/file.md"} {
		resolved, err := r.destination(relative)
		if r.reservedName(".GIT", ".git") && err == nil {
			t.Fatalf("accepted case alias %s -> %s", relative, resolved)
		}
	}
	put(t, filepath.Join(r.root, "MixedCase.md"), "existing file", 0644)
	actual, err := resolvePath(filepath.Join(r.root, "MixedCase.md"))
	if err != nil {
		t.Fatal(err)
	}
	aliasInfo, err := os.Stat(filepath.Join(r.root, "mixedcase.md"))
	if os.IsNotExist(err) {
		return // This filesystem keeps these names distinct.
	}
	if err != nil || aliasInfo.IsDir() {
		t.Fatal(err)
	}
	alias, err := resolvePath(filepath.Join(r.root, "mixedcase.md"))
	if err != nil || alias != actual {
		t.Fatal("filesystem alias did not share destination identity", alias, actual, err)
	}
}
