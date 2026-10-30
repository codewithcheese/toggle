package toggle

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
)

func prepareDefinition(ctx context.Context, r *repo, action string, f feature, protected []string, view *comparisonView, status bool) (*plan, error) {
	files, err := sourceEntries(f)
	if err != nil {
		return nil, fmt.Errorf("%s: source %s: %w", f.label(), f.dir, err)
	}
	p := &plan{action: action, name: f.label(), repo: r, definition: f, protected: protected, oldExclude: bytes.Clone(view.exclusion), total: len(files)}
	seen := map[string]string{}
	var paths []string
	for _, source := range files {
		resolved, err := r.destination(source.relative)
		if err != nil {
			p.invalidPath = true
			p.conflict(source.relative, err.Error())
			continue
		}
		if err := checkDefinitionBoundary(r, source.relative, resolved, protected); err != nil {
			p.conflict(source.relative, err.Error())
		}
		for other, declared := range seen {
			if overlap(other, resolved) {
				p.conflict(source.relative, "overlapping resolved destination with "+declared+": "+resolved)
			}
		}
		seen[resolved] = source.relative
		relative, err := filepath.Rel(r.root, resolved)
		if err != nil {
			return nil, err
		}
		if _, err := exactExclusion(relative); err != nil {
			p.conflict(relative, err.Error())
		}
		paths = append(paths, relative)
		if tracked := view.trackedPath(filepath.Join(r.root, source.relative), resolved); tracked != "" {
			p.conflict(tracked, "tracked by Git; resolve its tracked state before changes")
		}
		observed, err := view.inspect(resolved)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.label(), err)
		}
		change := fileChange{file: filePaths{Relative: source.relative, Declared: source.relative, Resolved: relative}, before: observed.content, desired: source.content, source: source.path, sourceRoot: source.root}
		if observed.content.missing {
			p.missing++
			if action == "on" {
				change.desired, err = view.read(source.resolved)
			}
		} else {
			p.present++
			if observed.content.invalid {
				p.different++
				p.conflict(resolved, "not an ordinary file; source: "+source.path)
			} else if observed.content.exec != source.content.exec || observed.content.size != source.content.size {
				p.different++
				p.conflict(resolved, "content or executable mode differs from source: "+source.path)
			} else {
				change.before, err = view.read(resolved)
				if err == nil {
					change.desired, err = view.read(source.resolved)
				}
				if err == nil {
					if change.before.matches(change.desired) {
						p.matching++
					} else {
						p.different++
						p.conflict(resolved, "content or executable mode differs from source: "+source.path)
					}
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.label(), err)
		}
		if action == "on" && (change.desired.missing || change.desired.invalid) {
			return nil, fmt.Errorf("%s: source changed or became unavailable: %s", f.label(), source.path)
		}
		p.changes = append(p.changes, change)
	}
	if action == "off" && p.present != 0 && p.missing != 0 {
		for _, change := range p.changes {
			if change.before.missing {
				p.conflict(change.file.Declared, "partial presence: destination is absent")
			}
		}
	}
	// Status can omit absent definitions without reading source contents or
	// preparing an exclusion mutation; source/path enumeration was still checked.
	if status && p.present == 0 && !p.invalidPath {
		return p, nil
	}
	if action == "on" {
		blocked, err := r.allowExclusions(ctx, paths)
		if err != nil {
			return nil, err
		}
		for _, path := range sortedNames(blocked) {
			p.conflict(path, blocked[path])
		}
	}
	p.newExclude, err = updateExclusions(p.oldExclude, paths, action)
	if err != nil {
		p.conflict(r.excludePath, err.Error())
	}
	if action == "off" && p.err() == nil && p.present != 0 {
		if err := p.prepareDirectories(); err != nil {
			p.conflict(r.root, "directory cleanup: "+err.Error())
		}
	}
	p.noop = p.err() == nil && bytes.Equal(p.oldExclude, p.newExclude) && len(p.directories) == 0 && (action == "off" && p.present == 0 || action == "on" && p.missing == 0)
	return p, nil
}
