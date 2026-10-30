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

func TestStatelessFilesAndGitIsolation(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", ".agents/skills/review/SKILL.md", "skill instructions")
	put(t, filepath.Join(f.dir, "README.md"), "metadata", 0644)
	raw := "{\n // keep comments and bytes\n invalid JSON\n}\n"
	put(t, filepath.Join(f.dir, "source/settings.jsonc"), raw, 0644)
	put(t, filepath.Join(f.dir, "source/settings.json"), raw, 0644)
	put(t, filepath.Join(f.dir, "source/run.sh"), "echo hi\n", 0751)
	put(t, filepath.Join(r.root, "settings.jsonc"), raw, 0644)
	beforeSource := snapshot(t, f.dir)
	existing, _ := os.Stat(filepath.Join(r.root, "settings.jsonc"))
	applyFeature(t, r, f, "on")
	after, _ := os.Stat(filepath.Join(r.root, "settings.jsonc"))
	if !os.SameFile(existing, after) {
		t.Fatal("matching file was replaced")
	}
	for _, name := range []string{"settings.json", "settings.jsonc"} {
		if mustRead(t, filepath.Join(r.root, name)) != raw {
			t.Fatal("JSON was parsed or merged")
		}
	}
	info, _ := os.Stat(filepath.Join(r.root, "run.sh"))
	if info.Mode().Perm()&0111 != 0111 {
		t.Fatal("executable bits lost")
	}
	assertAbsent(t, filepath.Join(r.root, "README.md"))
	assertAbsent(t, legacyPath(r))
	before := snapshot(t, r.root)
	applyFeature(t, r, f, "on")
	if !reflect.DeepEqual(before, snapshot(t, r.root)) {
		t.Fatal("repeated on changed files")
	}
	testGit(t, r.root, "add", ".")
	if status := testGit(t, r.root, "status", "--porcelain"); status != "" {
		t.Fatal("feature files staged", status)
	}
	applyFeature(t, r, f, "off")
	assertAbsent(t, filepath.Join(r.root, ".agents"))
	for _, name := range []string{"settings.json", "settings.jsonc", "run.sh"} {
		assertAbsent(t, filepath.Join(r.root, name))
	}
	before = snapshot(t, r.root)
	applyFeature(t, r, f, "off")
	if !reflect.DeepEqual(before, snapshot(t, r.root)) {
		t.Fatal("repeated off changed files")
	}
	if !reflect.DeepEqual(beforeSource, snapshot(t, f.dir)) {
		t.Fatal("source changed")
	}
}

func TestOffWithoutHistoryAndLocalFork(t *testing.T) {
	r := testRepo(t)
	store := t.TempDir()
	team := testFeature(t, store, "review", "instructions.md", "team version")
	put(t, filepath.Join(r.root, "instructions.md"), "local edited version", 0755)
	local := testFeature(t, filepath.Join(r.root, ".toggle"), "review", "instructions.md", "local edited version")
	os.Chmod(filepath.Join(local.dir, "source/instructions.md"), 0755)
	if err := r.metadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	put(t, legacyPath(r), "obsolete invalid activation data", 0600)
	if p := mustPlan(t, r, "off", team); p.err() == nil {
		t.Fatal("team mismatch accepted")
	}
	applyFeature(t, r, local, "on")
	applyFeature(t, r, local, "off")
	assertAbsent(t, filepath.Join(r.root, "instructions.md"))
	if mustRead(t, legacyPath(r)) != "obsolete invalid activation data" {
		t.Fatal("legacy record consulted or changed")
	}
	if mustRead(t, filepath.Join(team.dir, "source/instructions.md")) != "team version" || mustRead(t, filepath.Join(local.dir, "source/instructions.md")) != "local edited version" {
		t.Fatal("source modified")
	}
}

func TestCurrentSourceChangesAndSharedMatches(t *testing.T) {
	r := testRepo(t)
	root := t.TempDir()
	a := testFeature(t, root, "review", "instructions.md", "same")
	b := testFeature(t, root, "checks", "instructions.md", "same")
	applyFeature(t, r, a, "on")
	applyFeature(t, r, b, "on")
	block, _ := parseExclusions([]byte(mustRead(t, r.excludePath)))
	if len(block.rules) != 1 {
		t.Fatal("shared path duplicated or owned", block)
	}
	applyFeature(t, r, b, "off")
	assertAbsent(t, filepath.Join(r.root, "instructions.md"))
	applyFeature(t, r, a, "on")
	put(t, filepath.Join(a.dir, "source/instructions.md"), "updated", 0644)
	before := snapshot(t, r.root)
	for _, action := range []string{"on", "off"} {
		if err := mustPlan(t, r, action, a).apply(context.Background(), defaultOps()); err == nil {
			t.Fatal("source drift accepted", action)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root)) {
		t.Fatal("source drift changed repo")
	}
	os.Remove(filepath.Join(a.dir, "source/instructions.md"))
	put(t, filepath.Join(a.dir, "source/new.md"), "new", 0644)
	applyFeature(t, r, a, "off")
	if mustRead(t, filepath.Join(r.root, "instructions.md")) != "same" {
		t.Fatal("historical path removed")
	}
	if !strings.Contains(mustRead(t, r.excludePath), "/instructions.md") {
		t.Fatal("historical rule removed")
	}
}

func TestEveryWriteFailureStopsLaterEffects(t *testing.T) {
	for _, action := range []string{"on", "off"} {
		for failAt := 1; failAt <= 4; failAt++ {
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
						return errors.New("injected removal failure")
					}
					return os.Remove(path)
				}
				err := p.apply(context.Background(), ops)
				var partial *applicationError
				if !errors.As(err, &partial) || calls != failAt || len(partial.completed) != failAt-1 || len(partial.remaining) != 4-failAt {
					t.Fatal(calls, partial, err)
				}
				for _, value := range []string{f.name, partial.path, r.excludePath, "Completed effects:", "Remaining effects:"} {
					if !strings.Contains(err.Error(), value) {
						t.Fatal("incomplete report", err)
					}
				}
				if strings.Contains(err.Error(), "bookkeeping") {
					t.Fatal("reported removed state", err)
				}
				assertAbsent(t, legacyPath(r))
				if action == "on" && failAt == 3 {
					if mustRead(t, filepath.Join(r.root, "first.md")) != "first" || mustRead(t, filepath.Join(r.root, "second.md")) != "partial write" {
						t.Fatal("partial effects lost")
					}
					assertAbsent(t, filepath.Join(r.root, "third.md"))
				}
				if action == "off" && failAt == 2 {
					assertAbsent(t, filepath.Join(r.root, "first.md"))
					if mustRead(t, filepath.Join(r.root, "third.md")) != "third" || !bytes.Equal([]byte(mustRead(t, r.excludePath)), p.oldExclude) {
						t.Fatal("off continued")
					}
					before := snapshot(t, r.root)
					if err := mustPlan(t, r, "off", f).apply(context.Background(), defaultOps()); err == nil || !strings.Contains(err.Error(), "partial presence") {
						t.Fatal("partial off retried silently", err)
					}
					if !reflect.DeepEqual(before, snapshot(t, r.root)) {
						t.Fatal("partial retry changed files")
					}
					applyFeature(t, r, f, "on")
					applyFeature(t, r, f, "off")
				}
				if action == "off" && failAt == 4 {
					applyFeature(t, r, f, "off") // All files are absent; stale selected rules can be removed.
					if bytes.Contains([]byte(mustRead(t, r.excludePath)), []byte("/first.md")) {
						t.Fatal("absence did not subtract stale exclusions")
					}
				}
			})
		}
	}
}

func TestFreshChecksRejectChangedInputs(t *testing.T) {
	for _, kind := range []string{"target", "source", "source-list", "exclusions", "symlink", "tracked"} {
		t.Run(kind, func(t *testing.T) {
			r := testRepo(t)
			f := testFeature(t, t.TempDir(), "review", "instructions.md", "source")
			put(t, filepath.Join(r.root, "instructions.md"), "source", 0644)
			p := mustPlan(t, r, "off", f)
			switch kind {
			case "target":
				put(t, filepath.Join(r.root, "instructions.md"), "user edit", 0644)
			case "source":
				put(t, filepath.Join(f.dir, "source/instructions.md"), "source edit", 0644)
			case "source-list":
				put(t, filepath.Join(f.dir, "source/new.md"), "new source file", 0644)
			case "exclusions":
				put(t, r.excludePath, "new user rules", 0644)
			case "symlink":
				os.Rename(filepath.Join(r.root, "instructions.md"), filepath.Join(r.root, "original.md"))
				os.Symlink("other.md", filepath.Join(r.root, "instructions.md"))
			case "tracked":
				testGit(t, r.root, "add", "-f", "instructions.md")
			}
			before := snapshot(t, r.root, f.dir)
			if err := p.apply(context.Background(), defaultOps()); err == nil {
				t.Fatal("applied changed inputs", kind)
			}
			if !reflect.DeepEqual(before, snapshot(t, r.root, f.dir)) {
				t.Fatal("failed checks changed files")
			}
		})
	}
}

func TestIndexRefreshProtectsFilesStagedDuringOff(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "first.md", "first")
	put(t, filepath.Join(f.dir, "source/second.md"), "second", 0644)
	applyFeature(t, r, f, "on")
	p := mustPlan(t, r, "off", f)
	ops := defaultOps()
	ops.remove = func(path string) error {
		if err := os.Remove(path); err != nil {
			return err
		}
		testGit(t, r.root, "add", "-f", "second.md")
		return nil
	}
	err := p.apply(context.Background(), ops)
	if err == nil || !strings.Contains(err.Error(), "tracked by Git") {
		t.Fatal("staged file not protected", err)
	}
	if mustRead(t, filepath.Join(r.root, "second.md")) != "second" {
		t.Fatal("staged file removed")
	}
}

func TestWriteCommandsReuseStableIndexSnapshots(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "first.md", "first")
	put(t, filepath.Join(f.dir, "source/second.md"), "second", 0644)
	put(t, filepath.Join(f.dir, "source/third.md"), "third", 0644)
	calls := map[string]int{}
	r.query = func(ctx context.Context, cwd string, data []byte, args ...string) ([]byte, error) {
		calls[args[0]]++
		return git(ctx, cwd, data, args...)
	}
	applyFeature(t, r, f, "on")
	if calls["ls-files"] != 2 || calls["check-ignore"] != 2 {
		t.Fatal("queries were not batched per snapshot", calls)
	}
}

func TestMatchingDefinitionsCanShareDestinationAliases(t *testing.T) {
	r := testRepo(t)
	store := t.TempDir()
	direct := testFeature(t, store, "checks", "real/skill.md", "same")
	alias := testFeature(t, store, "review", "skills-link/skill.md", "same")
	if err := os.Symlink("real", filepath.Join(r.root, "skills-link")); err != nil {
		t.Fatal(err)
	}
	applyFeature(t, r, direct, "on")
	path := filepath.Join(r.root, "real/skill.md")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	applyFeature(t, r, alias, "on")
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("matching shared destination was rewritten", err)
	}
	block, err := parseExclusions([]byte(mustRead(t, r.excludePath)))
	if err != nil || len(block.rules) != 1 || !block.rules["/real/skill.md"] {
		t.Fatal("aliases did not share one resolved exclusion", block, err)
	}
	applyFeature(t, r, alias, "off")
	assertAbsent(t, path)
	if target, err := os.Readlink(filepath.Join(r.root, "skills-link")); err != nil || target != "real" {
		t.Fatal("destination symlink changed", target, err)
	}
}

func TestOffUsesCurrentAliasWithoutHistoricalDestination(t *testing.T) {
	r := testRepo(t)
	f := testFeature(t, t.TempDir(), "review", "skills/skill.md", "same")
	link := filepath.Join(r.root, "skills")
	if err := os.Symlink("original", link); err != nil {
		t.Fatal(err)
	}
	applyFeature(t, r, f, "on")
	put(t, filepath.Join(r.root, "current/skill.md"), "same", 0644)
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("current", link); err != nil {
		t.Fatal(err)
	}
	applyFeature(t, r, f, "off")
	assertAbsent(t, filepath.Join(r.root, "current/skill.md"))
	if mustRead(t, filepath.Join(r.root, "original/skill.md")) != "same" || !strings.Contains(mustRead(t, r.excludePath), "/original/skill.md") {
		t.Fatal("historical destination or exclusion was changed")
	}
	if target, err := os.Readlink(link); err != nil || target != "current" {
		t.Fatal("current symlink changed", target, err)
	}
}
