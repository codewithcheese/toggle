package toggle

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// File operations are injectable so failure tests can exercise real partial
// effects without relying on platform-dependent permission failures.
type fileOps struct {
	metadata func(string, []byte) error
	write    func(string, []byte, os.FileMode) error
	remove   func(string) error
	rmdir    func(string) error
}

func defaultOps() fileOps {
	return fileOps{metadata: atomicWrite, write: writeInstalled, remove: os.Remove, rmdir: syscall.Rmdir}
}

func writeInstalled(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (p *plan) recheckFile(ctx context.Context, change fileChange, view *comparisonView) error {
	resolved, err := p.repo.destination(change.file.Declared)
	if err != nil {
		return err
	}
	want := filepath.Join(p.repo.root, change.file.Resolved)
	if resolved != want {
		return fmt.Errorf("destination redirected: %s; planned %s; current %s", change.file.Declared, want, resolved)
	}
	if err := checkDefinitionBoundary(p.repo, change.file.Declared, resolved, p.protected); err != nil {
		return err
	}
	if err := view.refreshIndex(ctx); err != nil {
		return err
	}
	tracked := view.trackedPath(filepath.Join(p.repo.root, change.file.Declared), resolved)
	if tracked != "" {
		return fmt.Errorf("tracked by Git: %s", tracked)
	}
	current, err := readContent(resolved)
	if err != nil {
		return err
	}
	if !current.matches(change.before) {
		return fmt.Errorf("file changed after preparation: %s", resolved)
	}
	if change.source != "" && !(p.action == "off" && change.before.missing) {
		resolvedSource, err := resolvePath(change.source)
		if err != nil {
			return err
		}
		if !within(change.sourceRoot, resolvedSource) {
			return fmt.Errorf("source escaped source/: %s -> %s", change.source, resolvedSource)
		}
		source, err := readContent(resolvedSource)
		if err != nil {
			return err
		}
		if !source.matches(change.desired) {
			return fmt.Errorf("source changed after preparation: %s", change.source)
		}
	}
	return nil
}

func (p *plan) recheck(ctx context.Context) (*comparisonView, error) {
	view, err := newView(ctx, p.repo)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(view.exclusion, p.oldExclude) {
		return nil, fmt.Errorf("exclusions changed after preparation: %s", p.repo.excludePath)
	}
	fresh, err := prepareDefinition(ctx, p.repo, p.action, p.definition, p.protected, view, false)
	if err != nil {
		return nil, err
	}
	if err := fresh.err(); err != nil {
		return nil, err
	}
	if len(p.changes) != len(fresh.changes) {
		return nil, fmt.Errorf("source file list changed after preparation: %s", p.definition.dir)
	}
	for i, change := range p.changes {
		other := fresh.changes[i]
		if change.file != other.file || change.sourceRoot != other.sourceRoot || !change.before.matches(other.before) || !change.desired.matches(other.desired) {
			return nil, fmt.Errorf("file or source changed after preparation: %s", change.file.Declared)
		}
	}
	return view, nil
}

type effect struct {
	label, path string
	run         func() error
}

type applicationError struct {
	feature, failed, path, excludes string
	cause                           error
	completed, remaining            []string
}

func (e *applicationError) Error() string {
	return fmt.Sprintf("%s: application failed: %s: %s: %v\nCompleted effects: %s\nFailed effect may have partial effects: %s\nRemaining effects: %s\nInspect exclusions: %s\nCompleted effects remain; manual cleanup may be required.",
		e.feature, e.failed, e.path, e.cause, effectsText(e.completed), e.failed, effectsText(e.remaining), e.excludes)
}

func effectsText(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	return strings.Join(items, "; ")
}

func (p *plan) apply(ctx context.Context, ops fileOps) error {
	if err := p.err(); err != nil {
		return err
	}
	if p.noop {
		return nil
	}
	view, err := p.recheck(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", p.name, err)
	}
	var effects []effect
	exclusion := effect{label: "update exclusions", path: p.repo.excludePath, run: func() error { return ops.metadata(p.repo.excludePath, p.newExclude) }}
	if p.action == "on" && !bytes.Equal(p.oldExclude, p.newExclude) {
		effects = append(effects, exclusion)
	}
	for _, change := range p.changes {
		if p.action == "on" && !change.before.missing || p.action == "off" && change.before.missing {
			continue
		}
		path := filepath.Join(p.repo.root, change.file.Resolved)
		label := "install " + change.file.Declared
		if p.action == "off" {
			label = "remove " + change.file.Declared
		}
		effects = append(effects, effect{label: label, path: path, run: func() error {
			if err := p.recheckFile(ctx, change, view); err != nil {
				return err
			}
			if p.action == "off" {
				return ops.remove(path)
			}
			return ops.write(path, change.desired.data, 0644|os.FileMode(change.desired.exec))
		}})
	}
	if p.action == "off" {
		for _, dir := range p.directories {
			effects = append(effects, effect{label: "prune empty directory " + dir, path: dir, run: func() error {
				return removeEmptyDirectory(p.repo.root, dir, ops.rmdir)
			}})
		}
		if !bytes.Equal(p.oldExclude, p.newExclude) {
			effects = append(effects, exclusion)
		}
	}
	var completed []string
	for i, effect := range effects {
		if err := effect.run(); err != nil {
			var remaining []string
			for _, pending := range effects[i+1:] {
				remaining = append(remaining, pending.label+" ("+pending.path+")")
			}
			return &applicationError{feature: p.name, failed: effect.label, path: effect.path, cause: err, completed: completed, remaining: remaining, excludes: p.repo.excludePath}
		}
		completed = append(completed, effect.label+" ("+effect.path+")")
	}
	return nil
}
