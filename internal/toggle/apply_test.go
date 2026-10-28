package toggle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestApplyWholeFilesAndGitIsolation(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", ".agents/skills/review/SKILL.md", "skill instructions")
	put(t, filepath.Join(f.dir, "README.md"), "metadata must stay out", 0644)
	json := "{\n  // copy this comment exactly\n  \"array\": [1, 2]\n}\n"
	put(t, filepath.Join(f.dir, "source", "settings.jsonc"), json, 0644)
	put(t, filepath.Join(f.dir, "source", "settings.json"), "{whole bytes without parsing", 0644)
	put(t, filepath.Join(f.dir, "source", "run.sh"), "echo hi\n", 0751)
	put(t, filepath.Join(r.root, ".agents/skills/personal/SKILL.md"), "unrelated", 0644)
	put(t, filepath.Join(r.root, ".gitignore"), "/unrelated-ignored\n", 0644)
	// An already present matching file is accepted and later removed by off.
	put(t, filepath.Join(r.root, "settings.jsonc"), json, 0644)
	sourceBefore := snapshot(t, f.dir)
	applyFeature(t, r, f, "on")
	if mustRead(t, filepath.Join(r.root, "settings.jsonc")) != json {
		t.Fatal("JSONC was edited instead of copied")
	}
	if mustRead(t, filepath.Join(r.root, "settings.json")) != "{whole bytes without parsing" {
		t.Fatal("JSON was parsed instead of copied")
	}
	info, _ := os.Stat(filepath.Join(r.root, "run.sh"))
	if info.Mode().Perm()&0111 != 0111 {
		t.Fatal("executable mode not preserved")
	}
	assertAbsent(t, filepath.Join(r.root, "README.md"))
	match, err := r.ignore(context.Background(), ".agents/skills/review/SKILL.md")
	if err != nil || match.pattern == "" {
		t.Fatal(match, err)
	}
	other, _ := r.ignore(context.Background(), ".agents/skills/personal/SKILL.md")
	if other.pattern != "" {
		t.Fatal("unrelated skill was hidden")
	}
	before := snapshot(t, r.root)
	if p := applyFeature(t, r, f, "on"); !p.noop || !reflect.DeepEqual(before, snapshot(t, r.root)) {
		t.Fatal("repeated on changed the repo")
	}
	applyFeature(t, r, f, "off")
	for _, path := range []string{"settings.json", "settings.jsonc", "run.sh", ".agents/skills/review/SKILL.md"} {
		assertAbsent(t, filepath.Join(r.root, path))
	}
	if mustRead(t, filepath.Join(r.root, ".agents/skills/personal/SKILL.md")) != "unrelated" {
		t.Fatal("unrelated skill changed")
	}
	if !reflect.DeepEqual(sourceBefore, snapshot(t, f.dir)) {
		t.Fatal("toggle changed source definitions")
	}
	if mustRead(t, filepath.Join(r.root, ".gitignore")) != "/unrelated-ignored\n" {
		t.Fatal("versioned ignore file changed")
	}
}

func TestNormalStagingStaysClean(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "instructions.md", "instructions")
	applyFeature(t, r, f, "on")
	testGit(t, r.root, "add", ".")
	if status := testGit(t, r.root, "status", "--porcelain"); status != "" {
		t.Fatal(status)
	}
}

func TestApplyExplicitPreservationAndMissingFiles(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "instructions.md", "source")
	applyFeature(t, r, f, "on")
	target := filepath.Join(r.root, "instructions.md")
	put(t, target, "explicitly saved edit", 0755)
	before := snapshot(t, r.root)
	if err := mustPlan(t, r, "off", f).apply(context.Background(), defaultOps()); err == nil {
		t.Fatal("removed unpreserved edit")
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root)) {
		t.Fatal("conflicting off changed files")
	}
	put(t, filepath.Join(f.dir, "source", "instructions.md"), "explicitly saved edit", 0755)
	applyFeature(t, r, f, "off")
	assertAbsent(t, target)
	applyFeature(t, r, f, "on")
	if mustRead(t, target) != "explicitly saved edit" {
		t.Fatal("did not install saved source")
	}
	os.Remove(target)
	applyFeature(t, r, f, "off")
	state, _, err := loadRecords(r.recordPath)
	if err != nil || len(state.Features) != 0 {
		t.Fatal(state, err)
	}
}

func TestApplyPreservesDestinationSymlink(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "alias/instructions.md", "skill")
	os.Symlink("local", filepath.Join(r.root, "alias"))
	applyFeature(t, r, f, "on")
	if mustRead(t, filepath.Join(r.root, "local/instructions.md")) != "skill" {
		t.Fatal("wrong installed file")
	}
	applyFeature(t, r, f, "off")
	assertAbsent(t, filepath.Join(r.root, "local/instructions.md"))
	if link, err := os.Readlink(filepath.Join(r.root, "alias")); err != nil || link != "local" {
		t.Fatal("symlink changed", link, err)
	}
}

func TestMetadataFailureBeforeTargetWrites(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "instructions.md", "skill")
	p := mustPlan(t, r, "on", f)
	ops := defaultOps()
	ops.metadata = func(path string, data []byte) error {
		if path == r.recordPath {
			return errors.New("injected record failure")
		}
		return atomicWrite(path, data)
	}
	err := p.apply(context.Background(), ops)
	if err == nil || !strings.Contains(err.Error(), r.recordPath) {
		t.Fatal(err)
	}
	assertAbsent(t, filepath.Join(r.root, "instructions.md"))
	assertAbsent(t, r.recordPath)
	if !bytes.Equal([]byte(mustRead(t, r.excludePath)), p.newExclude) {
		t.Fatal("successful exclusion effect was rolled back")
	}
}

func TestFailureStopsEveryLaterEffect(t *testing.T) {
	for _, action := range []string{"on", "off"} {
		for failAt := 1; failAt <= 5; failAt++ {
			t.Run(fmt.Sprintf("%s/%d", action, failAt), func(t *testing.T) {
				r := testRepo(t)
				f := testFeature(t, t.TempDir(), "review", "first.md", "first")
				put(t, filepath.Join(f.dir, "source/second.md"), "second", 0644)
				put(t, filepath.Join(f.dir, "source/third.md"), "third", 0644)
				if action == "off" {
					applyFeature(t, r, f, "on")
				}
				p := mustPlan(t, r, action, f)
				ops := defaultOps()
				calls := 0
				fail := func() bool { calls++; return calls == failAt }
				ops.metadata = func(path string, data []byte) error {
					if fail() {
						return errors.New("injected metadata failure")
					}
					return atomicWrite(path, data)
				}
				ops.write = func(path string, data []byte, mode os.FileMode) error {
					if fail() {
						put(t, path, "partial write", mode)
						return errors.New("injected partial write")
					}
					return writeInstalled(path, data, mode)
				}
				ops.remove = func(path string) error {
					if fail() {
						return errors.New("injected remove failure")
					}
					return os.Remove(path)
				}
				err := p.apply(context.Background(), ops)
				var partial *applicationError
				if !errors.As(err, &partial) || calls != failAt || len(partial.completed) != failAt-1 || len(partial.remaining) != 5-failAt {
					t.Fatalf("effects: %d %#v %v", calls, partial, err)
				}
				for _, value := range []string{"review", partial.path, r.recordPath, r.excludePath, "Completed effects:", "Remaining effects:"} {
					if !strings.Contains(err.Error(), value) {
						t.Fatal("incomplete report", err)
					}
				}
				if action == "on" && failAt == 4 {
					if mustRead(t, filepath.Join(r.root, "first.md")) != "first" || mustRead(t, filepath.Join(r.root, "second.md")) != "partial write" {
						t.Fatal("successful/failed effects not retained")
					}
					assertAbsent(t, filepath.Join(r.root, "third.md"))
				}
				if action == "off" && failAt == 2 {
					assertAbsent(t, filepath.Join(r.root, "first.md"))
					if mustRead(t, filepath.Join(r.root, "third.md")) != "third" || mustRead(t, r.excludePath) != string(p.oldExclude) || mustRead(t, r.recordPath) != string(p.oldRecord) {
						t.Fatal("off continued or rolled back")
					}
				}
			})
		}
	}
}

func TestRecheckPreventsChangedFilesAndSources(t *testing.T) {
	for _, what := range []string{"target", "source", "exclusions", "symlink"} {
		t.Run(what, func(t *testing.T) {
			r := testRepo(t)
			f := testFeature(t, t.TempDir(), "review", "instructions.md", "source")
			p := mustPlan(t, r, "on", f)
			switch what {
			case "target":
				put(t, filepath.Join(r.root, "instructions.md"), "new user edit", 0644)
			case "source":
				put(t, filepath.Join(f.dir, "source/instructions.md"), "new source", 0644)
			case "exclusions":
				put(t, r.excludePath, "new user rules", 0644)
			case "symlink":
				os.Symlink("other.md", filepath.Join(r.root, "instructions.md"))
			}
			before := snapshot(t, r.root, f.dir)
			if err := p.apply(context.Background(), defaultOps()); err == nil {
				t.Fatal("applied stale plan")
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
				t.Fatal("stale-plan check changed files")
			}
		})
	}
}

func TestOffUsesRecordedFilesAndPreservesOtherInstallations(t *testing.T) {
	r := testRepo(t)
	global := t.TempDir()
	f := testFeature(t, global, "review", "old.md", "old source")
	other := testFeature(t, global, "checks", "checks.md", "other feature")
	applyFeature(t, r, f, "on")
	applyFeature(t, r, other, "on")
	os.Remove(filepath.Join(f.dir, "source/old.md"))
	put(t, filepath.Join(f.dir, "source/new.md"), "new source", 0644)
	put(t, filepath.Join(r.root, "new.md"), "unrelated existing file", 0644)
	applyFeature(t, r, f, "off")
	assertAbsent(t, filepath.Join(r.root, "old.md"))
	if mustRead(t, filepath.Join(r.root, "new.md")) != "unrelated existing file" || mustRead(t, filepath.Join(r.root, "checks.md")) != "other feature" {
		t.Fatal("off changed unrelated files")
	}
	state, _, err := loadRecords(r.recordPath)
	if err != nil || len(state.Features) != 1 || state.Features["checks"].Source != other.dir {
		t.Fatal("other installation not preserved", state, err)
	}
	match, err := r.ignore(context.Background(), "checks.md")
	if err != nil || match.pattern == "" {
		t.Fatal("other exclusion not preserved", match, err)
	}
}
