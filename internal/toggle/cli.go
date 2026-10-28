// Package toggle implements file-based features for Git working trees.
package toggle

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usage = `Usage:
  toggle list
  toggle status
  toggle on <feature> [--dry-run]
  toggle off <feature> [--dry-run]

On/off and previews require the Git repo root. Status works throughout a repo.
`

type command struct {
	action, name string
	dryRun       bool
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
	for _, arg := range args[1:] {
		switch {
		case !literal && arg == "--":
			literal = true
		case !literal && arg == "--dry-run":
			c.dryRun = true
		case !literal && strings.HasPrefix(arg, "-"):
			return c, fmt.Errorf("unknown option %q", arg)
		case c.name != "":
			return c, fmt.Errorf("%s requires exactly one feature name", c.action)
		default:
			c.name = arg
		}
	}
	if c.name == "" || c.name == "." || c.name == ".." || filepath.Base(c.name) != c.name {
		return c, fmt.Errorf("%s requires one feature directory name", c.action)
	}
	return c, nil
}

type app struct {
	cwd, globalRoot string
	out, errOut     io.Writer
	ops             *fileOps
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
	a := app{cwd: cwd, globalRoot: filepath.Join(home, ".local", "share", "toggle"), out: out, errOut: errOut}
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
	if c.action == "list" {
		features, err := discover(a.globalRoot, r)
		if err != nil {
			return err
		}
		for _, name := range sortedNames(features) {
			fmt.Fprintf(a.out, "%s\t%s\n", name, features[name].dir)
		}
		return nil
	}
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
	features, err := discover(a.globalRoot, r)
	if err != nil {
		return err
	}
	if c.action == "status" {
		return a.status(ctx, r, features)
	}
	p, err := prepare(ctx, r, c.action, c.name, features)
	if err != nil {
		return err
	}
	if err := p.err(); err != nil {
		return err
	}
	if p.noop {
		fmt.Fprintf(a.out, "%s is already %s\n", c.name, c.action)
		return nil
	}
	if c.dryRun {
		a.preview(p)
		return nil
	}
	ops := defaultOps()
	if a.ops != nil {
		ops = *a.ops
	}
	if err := p.apply(ctx, ops); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s is %s\n", c.name, c.action)
	return nil
}

func (a *app) preview(p *plan) {
	fmt.Fprintf(a.out, "%s %s (dry run)\n", p.action, p.name)
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
		fmt.Fprintf(a.out, "%s exclusion\t%s\n", map[bool]string{true: "add", false: "remove"}[p.action == "on"], pattern)
	}
	fmt.Fprintf(a.out, "update exclusions\t%s\nupdate bookkeeping\t%s\n", p.repo.excludePath, p.repo.recordPath)
}

func (a *app) status(ctx context.Context, r *repo, features map[string]feature) error {
	state, _, err := loadRecords(r.recordPath)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for name := range features {
		names[name] = true
	}
	for name := range state.Features {
		names[name] = true
	}
	for _, name := range sortedNames(names) {
		_, installed := state.Features[name]
		next := "on"
		label := "off"
		if installed {
			next, label = "off", "on"
		}
		p, err := prepare(ctx, r, next, name, features)
		if err != nil {
			return err
		}
		if len(p.conflicts) != 0 {
			fmt.Fprintf(a.out, "%s\tconflicted\t%s blocked\n", name, next)
			for _, reason := range p.conflicts {
				fmt.Fprintf(a.out, "  %s\n", reason)
			}
			continue
		}
		if installed && len(p.modified) != 0 {
			label = "modified"
		}
		fmt.Fprintf(a.out, "%s\t%s\n", name, label)
		for _, path := range p.modified {
			fmt.Fprintf(a.out, "  %s\n", path)
		}
	}
	return nil
}
