package toggle

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testConfig(t *testing.T, stores map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	data, err := json.Marshal(config{Stores: stores})
	if err != nil {
		t.Fatal(err)
	}
	put(t, path, string(data), 0644)
	return path
}

func TestCatalogSelection(t *testing.T) {
	r := testRepo(t)
	team, namedLocal := t.TempDir(), t.TempDir()
	testFeature(t, team, "review", "skill.md", "team")
	testFeature(t, namedLocal, "review", "skill.md", "named local")
	testFeature(t, filepath.Join(team, "group"), "nested", "skill.md", "nested")
	local := testFeature(t, filepath.Join(r.root, ".toggle"), "review", "skill.md", "repo local")
	cfg := config{Stores: map[string]string{"team": team, "local": namedLocal}}
	before := snapshot(t, r.root, team, namedLocal)
	all, err := definitions(cfg, r, command{})
	if err != nil || len(all) != 3 {
		t.Fatal(all, err)
	}
	if _, err := selectFeature(all, "review"); err == nil || !strings.Contains(err.Error(), "--local") || !strings.Contains(err.Error(), "--store local") || !strings.Contains(err.Error(), "--store team") {
		t.Fatal("ambiguity not reported", err)
	}
	for _, selector := range []command{{local: true}, {store: "local"}, {store: "team"}} {
		choices, err := definitions(cfg, r, selector)
		if err != nil {
			t.Fatal(err)
		}
		f, err := selectFeature(choices, "review")
		if err != nil || f.local != selector.local || !f.local && f.store != selector.store {
			t.Fatal(f, err)
		}
		if selector.local && f.dir != local.dir {
			t.Fatal(f)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, r.root, team, namedLocal)) {
		t.Fatal("discovery changed sources")
	}
	delete(cfg.Stores, "local")
	os.RemoveAll(filepath.Join(r.root, ".toggle"))
	choices, err := definitions(cfg, r, command{})
	f, selected := selectFeature(choices, "review")
	if err != nil || selected != nil || f.store != "team" {
		t.Fatal(f, err, selected)
	}
	if _, err := selectFeature(choices, "unknown"); err == nil {
		t.Fatal("unknown feature accepted")
	}
	if _, err := definitions(cfg, r, command{store: "unknown"}); err == nil {
		t.Fatal("unknown store accepted")
	}
	// An unconfigured former global-shaped directory has no meaning.
	testFeature(t, t.TempDir(), "openspec", "skill.md", "old global")
	choices, err = definitions(config{Stores: map[string]string{}}, nil, command{})
	if err != nil || len(choices) != 0 {
		t.Fatal(choices, err)
	}
}

func TestExplicitSelectionAndUnavailableSources(t *testing.T) {
	r := testRepo(t)
	testFeature(t, filepath.Join(r.root, ".toggle"), "review", "skill.md", "local")
	cfg := config{Stores: map[string]string{"missing": filepath.Join(t.TempDir(), "missing")}}
	if _, err := definitions(cfg, r, command{local: true}); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []command{{}, {store: "missing"}} {
		if _, err := definitions(cfg, r, cmd); err == nil || !strings.Contains(err.Error(), cfg.Stores["missing"]) {
			t.Fatal("unavailable catalog accepted", err)
		}
	}
}

func TestListHelpAndSelectorArguments(t *testing.T) {
	r := testRepo(t)
	team := t.TempDir()
	testFeature(t, team, "Review Notes", "skill.md", "team")
	testFeature(t, filepath.Join(r.root, ".toggle"), "Review Notes", "skill.md", "local")
	path := testConfig(t, map[string]string{"Team Notes": team})
	var out, diagnostics bytes.Buffer
	a := app{cwd: r.root, configPath: path, out: &out, errOut: &diagnostics}
	if code := a.run(context.Background(), []string{"list"}); code != 0 || !strings.Contains(out.String(), "--store 'Team Notes'") || !strings.Contains(out.String(), "--local") {
		t.Fatal(code, out.String(), diagnostics.String())
	}
	if strings.Count(out.String(), "'Review Notes'") != 2 {
		t.Fatal("list collapsed definitions", out.String())
	}
	for _, action := range []string{"on", "off"} {
		diagnostics.Reset()
		if code := a.run(context.Background(), []string{action, "Review Notes"}); code == 0 || !strings.Contains(diagnostics.String(), "ambiguous") {
			t.Fatal(code, diagnostics.String())
		}
	}
	put(t, path, "invalid JSON", 0644)
	a.cwd = t.TempDir()
	out.Reset()
	if code := a.run(context.Background(), []string{"--help"}); code != 0 || !strings.Contains(out.String(), "config.json") || !strings.Contains(out.String(), "source/<repo-relative files>") {
		t.Fatal(code, out.String())
	}
	for _, args := range [][]string{{"on", "review", "--local"}, {"off", "review", "--store", "local"}, {"on", "--store", "--label", "review"}} {
		if _, err := parse(args); err != nil {
			t.Fatal(args, err)
		}
	}
	for _, args := range [][]string{{"on", "review", "--local", "--store", "team"}, {"off", "review", "--store"}, {"list", "--local"}, {"status", "--store", "team"}} {
		if _, err := parse(args); err == nil {
			t.Fatal("invalid selectors accepted", args)
		}
	}
}
