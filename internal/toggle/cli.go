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

const usage = `Toggle lets you bring skills, agent instructions, configuration, and
research files into a project for a task, then remove them when you're done.

For code reviews, for example, you could keep a review skill and its
instructions together as a feature named review. Keep your features in
a directory called a store. Turn review on to copy its files from the
store into your project, then off to remove the project copies. The
originals stay in the store for the next time you need them.

The project copies are excluded from normal Git staging through Git's
local exclude file. Your .gitignore stays unchanged.

Usage:
  toggle list                     List features with stores and source paths.
  toggle status                   Show features ready for removal and conflicts.
  toggle on <feature> [--store <name>] [--dry-run]
  toggle off <feature> [--store <name>] [--dry-run]
  toggle --help

Run on/off, including previews, from your project's Git repo root.
Status works anywhere within that repo. List also works outside Git.
A completed status scan exits with code 0 even when it reports conflicts.

Set up a store:
  Create or edit ~/.config/toggle/config.json to name your store:

    {"stores":{"personal":"/absolute/path/to/features"}}

  Paths must be absolute. You can use an ordinary directory, a Git checkout,
  or a worktree. Toggle reads the files as they are on disk; edits do not
  need to be committed.

  All configured stores are visible in every project. If a feature name
  appears in several stores, use --store <name> to choose one.
  With no configured stores, toggle list is empty.

  Keeping stores outside your project keeps inactive feature files out
  of project searches. Configured stores inside a project also work.

Add a feature:
  In the personal store above, arrange the review files like this:

    /absolute/path/to/features/
      review/
        source/
          .agents/skills/review/SKILL.md

  The review directory names the feature. Files under source/ are copied
  to the same paths in your project: this skill becomes
  .agents/skills/review/SKILL.md. No manifest is needed, and files outside
  source/ are not copied.

Try it from your project's Git repo root:
  toggle on review --store personal --dry-run
  toggle on review --store personal
  toggle status
  toggle off review --store personal

  --dry-run checks and shows the proposed changes without writing anything.

Matching and protection:
  Existing project files must match the source contents and executable
  permissions. On fills in missing files and leaves matching files in place.
  Off removes the project copies only if all match the current source.
  If all are already absent, off just clears their Toggle exclusions.
  A mix of present and missing files, or any differing file, blocks off.

  If you've edited project files, save any work you want to keep and resolve
  differences with the store before using off. Toggle does not save edits
  back to the store for you. If the store has changed since you copied a
  feature, off compares your project files with that updated source.

  Feature files cannot be installed or removed at Git-tracked paths,
  .gitignore files, .git entries and descendants, Git metadata, or configured
  feature directories.

  Commands check for conflicts before writing. An unexpected failure while
  applying changes can leave partial results.

Project-specific features:
  For research or experiments tied to one project, you can keep a store
  in an external Git worktree of an orphan branch. This gives the feature
  files independent history in the same repository. Configure the worktree
  directory like any other store; manage worktrees and sharing with Git.
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
