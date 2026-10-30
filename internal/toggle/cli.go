// Package toggle implements file-based features for Git working trees.
package toggle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usage = `Toggle lets you add optional skills, agent instructions, configuration,
and research files to a project when needed, then remove them when finished.

Group files you want to add and remove together into a named feature.
A store is a directory that holds these features. Each feature has a
source/ directory containing the files to copy, arranged as they should
appear in the project.

Turning a feature on copies missing files into the project and leaves
existing matching files in place. Their paths are added to Git's local
exclude file, keeping them out of normal staging without changing .gitignore.
Turning it off removes its project files only when all match the current
source, then removes Toggle's exclusions for those paths.

The source files stay in the store for reuse. Toggle does not save project
edits back to the store. With stores outside the project, inactive source
files stay out of ordinary project searches.

Usage:
  toggle list                     List features with stores and source paths.
  toggle status                   Show matches ready for off and blockers.
  toggle on <feature> [--store <name>] [--dry-run]
  toggle off <feature> [--store <name>] [--dry-run]
  toggle --help

Run on/off, including previews, from the target Git repo root.
Status works throughout that repo. List also works outside Git.
--dry-run validates and previews changes without writing.
Status returns success for a completed scan, even with blockers.

Stores:
  Configure named stores in ~/.config/toggle/config.json:

    {"stores":{"personal":"/absolute/path/to/features"}}

  Store paths must be absolute. This config applies to every project.
  With no configured stores, toggle list is empty.
  Use --store <name> when a feature name occurs in multiple stores.
  Toggle reads current source files, including uncommitted changes.

Feature layout:
  <store-path>/<feature>/source/<project-relative files>

  Turning on review copies review/source/.agents/skills/review/SKILL.md
  from the store to .agents/skills/review/SKILL.md in the project.

  A feature's name is its directory name; no manifest is needed.
  Only source/ contents are copied.

Matching and protection:
  Existing files at selected paths must match source contents and
  executable bits. Off requires all selected files to match or all to
  be absent; partial presence blocks off. Comparisons use current source
  files, without installation history.

  On/off validate before writing. A failure while applying changes can
  leave partial results.

  Tracked destinations, .gitignore files, .git entries and descendants,
  Git metadata, and configured feature directories are protected.

Repo-specific features:
  You can keep these in an external Git worktree of an orphan branch
  with independent history. Configure its directory like any other store.
  Manage worktrees, commits, and sharing with ordinary Git.

Example:
  toggle on review --store personal --dry-run
`

type command struct {
	action, name string
	dryRun       bool
	store        string
}

func parse(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, fmt.Errorf("a command is required\n%s", usage)
	}
	c := command{action: args[0]}
	if c.action == "--help" || c.action == "-h" {
		if len(args) != 1 {
			return c, fmt.Errorf("help takes no arguments")
		}
		return command{action: "help"}, nil
	}
	if c.action == "list" || c.action == "status" {
		if len(args) != 1 {
			return c, fmt.Errorf("%s takes no arguments", c.action)
		}
		return c, nil
	}
	if c.action != "on" && c.action != "off" {
		return c, fmt.Errorf("unknown command %q\n%s", c.action, usage)
	}
	literal := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case !literal && arg == "--":
			literal = true
		case !literal && arg == "--dry-run":
			c.dryRun = true
		case !literal && arg == "--store":
			if i+1 >= len(args) || args[i+1] == "" || c.store != "" {
				return c, fmt.Errorf("--store requires one store name")
			}
			i++
			c.store = args[i]
		case !literal && strings.HasPrefix(arg, "-"):
			return c, fmt.Errorf("unknown option %q", arg)
		case c.name != "":
			return c, fmt.Errorf("%s requires exactly one feature name", c.action)
		default:
			if arg == "" {
				return c, fmt.Errorf("%s requires one nonempty feature name", c.action)
			}
			c.name = arg
		}
	}
	if c.name == "" || c.name == "." || c.name == ".." || filepath.Base(c.name) != c.name {
		return c, fmt.Errorf("%s requires one feature directory name", c.action)
	}
	return c, nil
}

type app struct {
	cwd         string
	configPath  string
	out, errOut io.Writer
	ops         *fileOps
}

// Run executes the CLI and returns a process exit code.
func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	a := app{cwd: cwd, configPath: filepath.Join(home, ".config", "toggle", "config.json"), out: out, errOut: errOut}
	return a.run(ctx, args)
}

func (a *app) run(ctx context.Context, args []string) int {
	c, err := parse(args)
	if err == nil && c.action == "help" {
		fmt.Fprint(a.out, usage)
		return 0
	}
	if err == nil {
		err = a.execute(ctx, c)
	}
	if err != nil {
		fmt.Fprintln(a.errOut, err)
		return 1
	}
	return 0
}

func (a *app) execute(ctx context.Context, c command) error {
	r, err := findRepo(ctx, a.cwd)
	if err != nil {
		return err
	}
	if c.action != "list" {
		if r == nil {
			if c.action == "status" {
				return fmt.Errorf("status requires a Git repo")
			}
			return fmt.Errorf("on/off and previews require the Git repo root")
		}
		if c.action != "status" {
			if err := r.requireRoot(a.cwd); err != nil {
				return err
			}
		}
		if err := r.metadata(ctx); err != nil {
			return err
		}
	}
	cfg, err := loadConfig(a.configPath)
	if err != nil {
		return err
	}
	features, err := definitions(cfg, c)
	if err != nil {
		return err
	}
	if c.action == "list" {
		for _, f := range features {
			fmt.Fprintf(a.out, "%s\t%s\t%s\n", shellQuote(f.name), f.selector(), f.dir)
		}
		return nil
	}
	if c.action == "status" {
		return a.status(ctx, r, features, cfg)
	}
	f, err := selectFeature(features, c.name)
	if err != nil {
		return err
	}
	protected, err := protectedDefinitions(cfg, r, []feature{f})
	if err != nil {
		return err
	}
	view, err := newView(ctx, r)
	if err != nil {
		return err
	}
	p, err := prepareDefinition(ctx, r, c.action, f, protected, view, false)
	if err != nil {
		return err
	}
	if err := p.err(); err != nil {
		return err
	}
	if c.dryRun {
		a.preview(p)
		return nil
	}
	if p.noop {
		fmt.Fprintf(a.out, "%s is already %s\n", f.label(), c.action)
		return nil
	}
	ops := defaultOps()
	if a.ops != nil {
		ops = *a.ops
	}
	if err := p.apply(ctx, ops); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s is %s\n", f.label(), c.action)
	return nil
}

func (a *app) preview(p *plan) {
	fmt.Fprintf(a.out, "%s %s (dry run)\n", p.action, p.name)
	if p.noop {
		fmt.Fprintln(a.out, "No changes.")
		return
	}
	for _, change := range p.changes {
		verb := "remove"
		if p.action == "on" {
			verb = "install"
			if !change.before.missing {
				verb = "accept matching file"
			}
		} else if change.before.missing {
			verb = "already absent"
		}
		fmt.Fprintf(a.out, "%s\t%s\t%s\n", verb, change.file.Declared, filepath.Join(p.repo.root, change.file.Resolved))
		pattern, _ := exactExclusion(change.file.Resolved)
		action := "remove"
		if p.action == "on" {
			action = "add"
		}
		fmt.Fprintf(a.out, "%s exclusion\t%s\n", action, pattern)
	}
	for _, dir := range p.directories {
		relative, _ := filepath.Rel(p.repo.root, dir)
		fmt.Fprintf(a.out, "remove empty directory\t%s\n", relative)
	}
	if !bytes.Equal(p.oldExclude, p.newExclude) {
		fmt.Fprintf(a.out, "update exclusions\t%s\n", p.repo.excludePath)
	}
}

func (a *app) status(ctx context.Context, r *repo, features []feature, cfg config) error {
	protected, err := protectedDefinitions(cfg, r, features)
	if err != nil {
		return err
	}
	view, err := newView(ctx, r)
	if err != nil {
		return err
	}
	rows := 0
	for _, f := range features {
		p, err := prepareDefinition(ctx, r, "off", f, protected, view, true)
		if err != nil {
			return err
		}
		if p.present == 0 && !p.invalidPath {
			continue
		}
		label := "ready for off"
		if p.err() != nil {
			label = "blocked"
		}
		fmt.Fprintf(a.out, "%s\t%s\t%s\t%d matching, %d missing, %d different\t%s\n", shellQuote(f.name), f.selector(), label, p.matching, p.missing, p.different, f.dir)
		for _, reason := range p.conflicts {
			fmt.Fprintf(a.out, "  %s\n", reason)
		}
		rows++
	}
	if rows == 0 {
		fmt.Fprintln(a.out, "No defined destination paths are present; other setup may be active.")
	}
	return nil
}
