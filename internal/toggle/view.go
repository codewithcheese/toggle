package toggle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type observedFile struct {
	content content
	info    os.FileInfo
	loaded  bool
}

// A view is a read-only snapshot/cache for one preparation or status scan.
// It never survives the command; apply takes fresh inputs before effects.
type comparisonView struct {
	repo      *repo
	tracked   map[string]bool
	files     map[string]*observedFile
	readBytes func(string) ([]byte, error)
	exclusion []byte
	indexInfo os.FileInfo
}

func newView(ctx context.Context, r *repo) (*comparisonView, error) {
	tracked, indexInfo, err := indexSnapshot(ctx, r)
	if err != nil {
		return nil, err
	}
	exclusion, err := readOptional(r.excludePath)
	if err != nil {
		return nil, fmt.Errorf("read exclusions %s: %w", r.excludePath, err)
	}
	return &comparisonView{repo: r, tracked: tracked, indexInfo: indexInfo, files: map[string]*observedFile{}, readBytes: os.ReadFile, exclusion: exclusion}, nil
}

func indexFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}

func sameIndex(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime() == b.ModTime()
}

func indexSnapshot(ctx context.Context, r *repo) (map[string]bool, os.FileInfo, error) {
	for attempt := 0; attempt < 3; attempt++ {
		before, err := indexFileInfo(r.indexPath)
		if err != nil {
			return nil, nil, err
		}
		entries, err := r.index(ctx)
		if err != nil {
			return nil, nil, err
		}
		after, err := indexFileInfo(r.indexPath)
		if err != nil {
			return nil, nil, err
		}
		if sameIndex(before, after) {
			return entries, after, nil
		}
	}
	return nil, nil, fmt.Errorf("Git index changed during validation; retry with stable inputs")
}

func (v *comparisonView) refreshIndex(ctx context.Context) error {
	current, err := indexFileInfo(v.repo.indexPath)
	if err != nil {
		return err
	}
	if sameIndex(current, v.indexInfo) {
		return nil
	}
	v.tracked, v.indexInfo, err = indexSnapshot(ctx, v.repo)
	return err
}

func (v *comparisonView) inspect(path string) (*observedFile, error) {
	if entry, ok := v.files[path]; ok {
		return entry, nil
	}
	info, err := os.Stat(path)
	entry := &observedFile{info: info}
	if os.IsNotExist(err) {
		entry.content.missing = true
	} else if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", path, err)
	} else if !info.Mode().IsRegular() {
		entry.content.invalid = true
	} else {
		entry.content.exec = uint32(info.Mode().Perm() & 0111)
		entry.content.size = info.Size()
	}
	v.files[path] = entry
	return entry, nil
}

func (v *comparisonView) read(path string) (content, error) {
	entry, err := v.inspect(path)
	if err != nil {
		return content{}, err
	}
	if !entry.loaded && !entry.content.missing && !entry.content.invalid {
		data, err := v.readBytes(path)
		if err != nil {
			return content{}, fmt.Errorf("read %s: %w", path, err)
		}
		entry.content = fingerprint(data, entry.info)
		entry.loaded = true
	}
	return entry.content, nil
}

func (v *comparisonView) trackedPath(paths ...string) string {
	for _, path := range paths {
		relative, err := filepath.Rel(v.repo.root, path)
		if err == nil && v.tracked[filepath.ToSlash(relative)] {
			return path
		}
	}
	return ""
}

// Only directory metadata is read for protection. An unrelated unavailable
// store cannot prevent explicit selection of an available named store.
func protectedDefinitions(c config, r *repo, selected []feature) ([]string, error) {
	var roots []string
	for _, name := range sortedNames(c.Stores) {
		roots = append(roots, c.Stores[name])
	}
	protected := map[string]bool{}
	add := func(path string) error {
		resolved, err := resolvePath(path)
		if err != nil {
			return err
		}
		if overlap(r.root, resolved) {
			protected[resolved] = true
		}
		return nil
	}
	for _, f := range selected {
		if err := add(f.dir); err != nil {
			return nil, err
		}
		if err := add(filepath.Join(f.dir, "source")); err != nil {
			return nil, err
		}
	}
	for _, root := range roots {
		canonical, err := resolvePath(root)
		if err != nil {
			if overlap(root, r.root) {
				return nil, fmt.Errorf("source boundary %s: %w", root, err)
			}
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			if overlap(canonical, r.root) && !os.IsNotExist(err) {
				return nil, fmt.Errorf("source boundary %s: %w", root, err)
			}
			continue
		}
		for _, entry := range entries {
			dir := filepath.Join(root, entry.Name())
			resolved, err := resolvePath(dir)
			if err != nil {
				continue
			}
			info, err := os.Stat(dir)
			if err != nil || !info.IsDir() {
				continue
			}
			source, err := os.Stat(filepath.Join(dir, "source"))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				if overlap(r.root, resolved) {
					return nil, fmt.Errorf("source boundary %s: %w", dir, err)
				}
				continue
			}
			if source.IsDir() {
				if overlap(r.root, resolved) {
					protected[resolved] = true
				}
				if err := add(filepath.Join(dir, "source")); err != nil && overlap(r.root, resolved) {
					return nil, err
				}
			}
		}
	}
	return sortedNames(protected), nil
}

func checkDefinitionBoundary(r *repo, declared, resolved string, protected []string) error {
	for _, source := range protected {
		if within(source, filepath.Join(r.root, declared)) || within(source, resolved) {
			return fmt.Errorf("destination overlaps protected feature definition %s: %s -> %s", source, declared, resolved)
		}
	}
	return nil
}

func (r *repo) allowExclusions(ctx context.Context, relatives []string) (map[string]string, error) {
	conflicts := map[string]string{}
	if len(relatives) == 0 {
		return conflicts, nil
	}
	input := []byte(strings.Join(relatives, "\x00") + "\x00")
	matches, err := r.ignoreMany(ctx, input)
	if err != nil {
		return nil, err
	}
	for path, match := range matches {
		if strings.HasPrefix(match.pattern, "!") && !filepath.IsAbs(match.source) && filepath.Base(match.source) == ".gitignore" {
			dir := filepath.Dir(match.source)
			if dir == "." || within(filepath.Join(r.root, dir), filepath.Join(r.root, path)) {
				conflicts[path] = fmt.Sprintf("local exclusion is overridden by %s:%s (%s)", match.source, match.line, match.pattern)
			}
		}
	}
	return conflicts, nil
}
