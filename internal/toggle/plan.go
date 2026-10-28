package toggle

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

type fileChange struct {
	file               installedFile
	before, desired    content
	source, sourceRoot string
}

type plan struct {
	action, name           string
	repo                   *repo
	noop                   bool
	changes                []fileChange
	conflicts, modified    []string
	oldRecord, newRecord   []byte
	oldExclude, newExclude []byte
	patterns               []string
}

func (p *plan) conflict(path, reason string) {
	p.conflicts = append(p.conflicts, fmt.Sprintf("%s: %s: %s", p.name, path, reason))
}

func (p *plan) err() error {
	if len(p.conflicts) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(p.conflicts, "\n"))
}

func relativeFile(path string) bool {
	return path != "" && path != "." && !filepath.IsAbs(path) && filepath.Clean(path) == path && path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func prepare(ctx context.Context, r *repo, action, name string, features map[string]feature) (*plan, error) {
	state, original, err := loadRecords(r.recordPath)
	if err != nil {
		return nil, err
	}
	p := &plan{action: action, name: name, repo: r, oldRecord: original}
	installed, on := state.Features[name]
	f, available := features[name]
	if action == "on" && on || action == "off" && !on && available {
		p.noop = true
		return p, nil
	}
	if !available {
		reason := "feature definition is unavailable"
		path := name
		if on {
			path = installed.Source
			var paths []string
			for _, file := range installed.Files {
				paths = append(paths, file.Declared)
			}
			reason += "; manual cleanup required; installed paths: " + strings.Join(paths, ", ")
		}
		p.conflict(path, reason)
		return p, nil
	}
	for owner, installation := range state.Features {
		for _, file := range installation.Files {
			if !relativeFile(file.Declared) || !relativeFile(file.Resolved) || !relativeFile(file.Relative) {
				return nil, fmt.Errorf("invalid installed path for %s in %s; inspect bookkeeping", owner, r.recordPath)
			}
		}
	}
	p.oldExclude, err = readOptional(r.excludePath)
	if err != nil {
		return nil, fmt.Errorf("read exclusions %s: %w", r.excludePath, err)
	}
	if action == "on" {
		err = p.prepareOn(ctx, f, state)
	} else {
		err = p.prepareOff(ctx, f, installed, state)
	}
	return p, err
}

func (p *plan) checkTracked(ctx context.Context, declared, resolved string) {
	tracked, err := p.repo.tracked(ctx, filepath.Join(p.repo.root, declared), resolved)
	if err != nil {
		p.conflict(declared, err.Error())
	} else if tracked != "" {
		p.conflict(tracked, "tracked by Git; resolve its tracked state before changing files or exclusions")
	}
}

func overlap(a, b string) bool { return within(a, b) || within(b, a) }

func (p *plan) prepareOn(ctx context.Context, f feature, state records) error {
	files, err := sourceFiles(f)
	if err != nil {
		p.conflict(f.dir, err.Error())
		return nil
	}
	root, err := resolvePath(filepath.Join(f.dir, "source"))
	if err != nil {
		return err
	}
	installation := installation{Source: f.dir}
	seen := map[string]string{}
	for _, source := range files {
		resolved, err := p.repo.destination(source.relative)
		if err != nil {
			p.conflict(source.relative, err.Error())
			continue
		}
		for path, declared := range seen {
			if overlap(path, resolved) {
				p.conflict(source.relative, fmt.Sprintf("overlapping resolved destination %s with %s", resolved, declared))
			}
		}
		seen[resolved] = source.relative
		for owner, other := range state.Features {
			for _, file := range other.Files {
				if overlap(filepath.Join(p.repo.root, file.Resolved), resolved) {
					p.conflict(source.relative, fmt.Sprintf("destination %s is installed for feature %s", resolved, owner))
				}
			}
		}
		p.checkTracked(ctx, source.relative, resolved)
		before, err := readContent(resolved)
		if err != nil {
			p.conflict(resolved, err.Error())
			continue
		}
		if !before.missing && !before.matches(source.content) {
			p.conflict(resolved, "existing content or executable mode differs from source: "+source.path)
		}
		rel, err := filepath.Rel(p.repo.root, resolved)
		if err != nil {
			return err
		}
		pattern, err := exactExclusion(rel)
		if err != nil {
			p.conflict(resolved, err.Error())
		} else {
			p.patterns = append(p.patterns, pattern)
		}
		if err := p.repo.exclusionAllowed(ctx, rel); err != nil {
			p.conflict(resolved, err.Error())
		}
		file := installedFile{Relative: source.relative, Declared: source.relative, Resolved: rel, Hash: source.content.hash, Executable: source.content.exec}
		installation.Files = append(installation.Files, file)
		p.changes = append(p.changes, fileChange{file: file, before: before, desired: source.content, source: source.path, sourceRoot: root})
	}
	installation.Exclusion = exclusionBlock(p.name, p.patterns)
	p.newExclude, err = addExclusion(p.oldExclude, p.name, installation.Exclusion)
	if err != nil {
		p.conflict(p.repo.excludePath, err.Error())
	}
	state.Features[p.name] = installation
	p.newRecord, err = recordBytes(state)
	return err
}

func (p *plan) prepareOff(ctx context.Context, f feature, installation installation, state records) error {
	for _, file := range installation.Files {
		resolved, err := p.repo.destination(file.Declared)
		if err != nil {
			p.conflict(file.Declared, err.Error())
			continue
		}
		recorded := filepath.Join(p.repo.root, file.Resolved)
		if resolved != recorded {
			p.conflict(file.Declared, fmt.Sprintf("destination redirected: recorded %s, current %s", recorded, resolved))
			continue
		}
		p.checkTracked(ctx, file.Declared, resolved)
		before, err := readContent(resolved)
		if err != nil {
			p.conflict(resolved, err.Error())
			continue
		}
		change := fileChange{file: file, before: before}
		baseline := content{hash: file.Hash, exec: file.Executable}
		if !before.matches(baseline) {
			p.modified = append(p.modified, file.Declared)
			if !before.missing {
				source := filepath.Join(f.dir, "source", file.Relative)
				root, rootErr := resolvePath(filepath.Join(f.dir, "source"))
				path, pathErr := resolvePath(source)
				if rootErr != nil || pathErr != nil || !within(root, path) {
					p.conflict(resolved, "edited file source is outside source/ or unavailable: "+source)
				} else {
					saved, err := readContent(path)
					if err != nil || !before.matches(saved) {
						p.conflict(resolved, "edit is not preserved in selected source: "+source)
					} else {
						change.source, change.sourceRoot, change.desired = source, root, saved
					}
				}
			}
		}
		p.changes = append(p.changes, change)
	}
	var err error
	p.newExclude, err = removeExclusion(p.oldExclude, p.name, installation.Exclusion)
	if err != nil {
		p.conflict(p.repo.excludePath, err.Error())
	}
	delete(state.Features, p.name)
	p.newRecord, err = recordBytes(state)
	return err
}
