package toggle

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNamedStoreCLILifecycle(t *testing.T) {
	r := testRepo(t)
	team, personal := t.TempDir(), t.TempDir()
	rel := ".agents/skills/review/SKILL.md"
	testFeature(t, team, "review", rel, "team")
	testFeature(t, personal, "review", rel, "personal")
	local := testFeature(t, filepath.Join(r.root, ".toggle"), "review", rel, "local edit")
	put(t, filepath.Join(local.dir, "source/.agents/skills/review/extra.md"), "local companion", 0644)
	os.Chmod(filepath.Join(local.dir, "source", rel), 0755)
	cfg := testConfig(t, map[string]string{"team": team, "personal": personal})
	// The owner chooses to ignore local definitions.
	put(t, filepath.Join(r.root, ".git/info/exclude"), "# user rules\n/.toggle/\n", 0644)
	beforeSources := snapshot(t, team, personal, local.dir)
	for _, action := range []string{"on", "off"} {
		before := snapshot(t, r.root, team, personal)
		if code, _, diagnostics := runConfigured(t, r.root, cfg, action, "review"); code == 0 || !strings.Contains(diagnostics, "ambiguous") {
			t.Fatal(code, diagnostics)
		}
		if !reflect.DeepEqual(before, snapshot(t, r.root, team, personal)) {
			t.Fatal("ambiguity changed files")
		}
	}
	_, out, _ := runConfigured(t, r.root, cfg, "list")
	if strings.Count(out, "review\t") != 3 {
		t.Fatal("list collapsed sources", out)
	}
	target := filepath.Join(r.root, rel)
	put(t, target, "team", 0644) // A matching pre-existing copy needs no ownership.
	before := snapshot(t, r.root, team, personal)
	if code, _, diagnostics := runConfigured(t, r.root, cfg, "on", "review", "--store", "team", "--dry-run"); code != 0 {
		t.Fatal(diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, team, personal)) {
		t.Fatal("preview wrote files")
	}
	if code, _, diagnostics := runConfigured(t, r.root, cfg, "on", "review", "--store", "team"); code != 0 {
		t.Fatal(diagnostics)
	}
	testGit(t, r.root, "add", ".")
	if status := testGit(t, r.root, "status", "--porcelain"); status != "" {
		t.Fatal(status)
	}
	if code, _, diagnostics := runConfigured(t, r.root, cfg, "on", "review", "--store", "personal"); code == 0 || !strings.Contains(diagnostics, "differs") {
		t.Fatal(code, diagnostics)
	}
	put(t, target, "local edit", 0755)
	// The saved local feature is selected directly, without a team off.
	if code, _, diagnostics := runConfigured(t, r.root, cfg, "on", "review", "--local"); code != 0 {
		t.Fatal(diagnostics)
	}
	if mustRead(t, filepath.Join(r.root, ".agents/skills/review/extra.md")) != "local companion" {
		t.Fatal("partial on did not complete the local definition")
	}
	_, out, _ = runConfigured(t, r.root, cfg, "status")
	if !strings.Contains(out, "review\t--local\tready for off") || !strings.Contains(out, "review\t--store team\tblocked") {
		t.Fatal(out)
	}
	before = snapshot(t, r.root, team, personal)
	if code, out, diagnostics := runConfigured(t, r.root, cfg, "off", "review", "--local", "--dry-run"); code != 0 || !strings.Contains(out, "remove empty directory\t.agents") {
		t.Fatal(code, out, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, team, personal)) {
		t.Fatal("off preview changed files")
	}
	os.Remove(filepath.Join(r.root, ".agents/skills/review/extra.md"))
	before = snapshot(t, r.root, team, personal)
	if code, _, diagnostics := runConfigured(t, r.root, cfg, "off", "review", "--local"); code == 0 || !strings.Contains(diagnostics, "partial presence") {
		t.Fatal(code, diagnostics)
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, team, personal)) {
		t.Fatal("partial off changed files")
	}
	for _, action := range []string{"on", "off", "off"} {
		if code, _, diagnostics := runConfigured(t, r.root, cfg, action, "review", "--local"); code != 0 {
			t.Fatal(diagnostics)
		}
	}
	assertAbsent(t, filepath.Join(r.root, ".agents"))
	if !reflect.DeepEqual(beforeSources, snapshot(t, team, personal, local.dir)) {
		t.Fatal("source definitions changed")
	}
	if mustRead(t, filepath.Join(r.root, ".git/info/exclude")) != "# user rules\n/.toggle/\n" {
		t.Fatal("outside exclusion bytes changed")
	}
	if status := testGit(t, r.root, "status", "--porcelain"); status != "" {
		t.Fatal(status)
	}
}

func TestSourceCheckoutGitStateIsReadOnly(t *testing.T) {
	r := testRepo(t)
	store := t.TempDir()
	testGit(t, store, "init", "-q")
	testFeature(t, store, "review", "instructions.md", "uncommitted source")
	testGit(t, store, "add", "review")
	testGit(t, store, "config", "remote.origin.url", "https://example.invalid/feature-repo")
	before := snapshot(t, store)
	cfg := testConfig(t, map[string]string{"team": store})
	for _, args := range [][]string{{"list"}, {"on", "review", "--store", "team"}, {"status"}, {"off", "review", "--store", "team"}} {
		if code, _, diagnostics := runConfigured(t, r.root, cfg, args...); code != 0 {
			t.Fatal(args, diagnostics)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, store)) {
		t.Fatal("source checkout files, index, branches, or config changed")
	}
}

func TestManualLegacyTransitionKeepsMatchingCopies(t *testing.T) {
	r := testRepo(t)
	if err := r.metadata(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := t.TempDir()
	f := testFeature(t, store, "review", "instructions.md", "matching contents")
	target := filepath.Join(r.root, "instructions.md")
	put(t, target, "matching contents", 0644)
	original, _ := os.Stat(target)
	legacyBlock := "\n# toggle begin 726576696577\n/instructions.md\n# toggle end 726576696577\n"
	outside := "# user rule\n/personal\n"
	put(t, filepath.Join(r.root, ".git/info/exclude"), outside+legacyBlock, 0644)
	put(t, filepath.Join(r.root, ".git/toggle.json"), `{"features":{"review":{"source":"/old/global/review"}}}`, 0600)
	cfg := testConfig(t, map[string]string{"team": store})
	if code, _, diagnostics := runConfigured(t, r.root, cfg, "on", "review", "--store", "team"); code != 0 {
		t.Fatal(diagnostics)
	}
	if mustRead(t, legacyPath(r)) != `{"features":{"review":{"source":"/old/global/review"}}}` {
		t.Fatal("automatic legacy-record migration")
	}
	raw := mustRead(t, filepath.Join(r.root, ".git/info/exclude"))
	if !strings.Contains(raw, legacyBlock) {
		t.Fatal("automatic legacy-block migration")
	}
	after, _ := os.Stat(target)
	if !os.SameFile(original, after) {
		t.Fatal("matching copy was reinstalled")
	}
	// One-off agent reconciliation, not a production migration command.
	put(t, filepath.Join(r.root, ".git/info/exclude"), strings.Replace(raw, legacyBlock, "", 1), 0644)
	os.Remove(legacyPath(r))
	beforeSource := snapshot(t, f.dir)
	for _, action := range []string{"on", "off"} {
		if code, _, diagnostics := runConfigured(t, r.root, cfg, action, "review", "--store", "team"); code != 0 {
			t.Fatal(diagnostics)
		}
	}
	assertAbsent(t, target)
	assertAbsent(t, legacyPath(r))
	if mustRead(t, filepath.Join(r.root, ".git/info/exclude")) != outside {
		t.Fatal("migration lost user rules")
	}
	if !reflect.DeepEqual(beforeSource, snapshot(t, f.dir)) {
		t.Fatal("source changed during transition")
	}
}
