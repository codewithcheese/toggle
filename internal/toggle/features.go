package toggle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type feature struct {
	name, dir string
}

func discover(global string, r *repo) (map[string]feature, error) {
	features := map[string]feature{}
	roots := []string{global}
	if r != nil {
		roots = append(roots, filepath.Join(r.root, ".toggle"))
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("discover %s: %w", root, err)
		}
		for _, entry := range entries {
			dir := filepath.Join(root, entry.Name())
			info, err := os.Stat(dir)
			if err != nil {
				return nil, fmt.Errorf("discover %s: %w", dir, err)
			}
			if !info.IsDir() {
				continue
			}
			source, err := os.Stat(filepath.Join(dir, "source"))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("discover %s: %w", dir, err)
			}
			if source.IsDir() {
				features[entry.Name()] = feature{name: entry.Name(), dir: dir}
			}
		}
	}
	return features, nil
}

func sortedNames[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
