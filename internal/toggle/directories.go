package toggle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// cleanupLimit preserves directory symlinks and the directory they point at.
// Only their installed descendants are candidates for removal.
func (p *plan) cleanupLimit(change fileChange) (string, error) {
	for dir := filepath.Dir(filepath.Join(p.repo.root, change.file.Declared)); dir != p.repo.root && within(p.repo.root, dir); dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return resolvePath(dir)
		}
	}
	return p.repo.root, nil
}

func (p *plan) prepareDirectories() error {
	removed := map[string]bool{}
	candidates := map[string]bool{}
	protected := map[string]bool{p.repo.root: true}
	for _, change := range p.changes {
		file := filepath.Join(p.repo.root, change.file.Resolved)
		removed[file] = true
		limit, err := p.cleanupLimit(change)
		if err != nil {
			return err
		}
		if !within(limit, file) {
			return fmt.Errorf("directory mapping changed: %s", change.file.Declared)
		}
		protected[limit] = true
		for dir := filepath.Dir(file); dir != limit && dir != p.repo.root && within(p.repo.root, dir); dir = filepath.Dir(dir) {
			info, err := os.Lstat(dir)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				break
			}
			candidates[dir] = true
		}
	}
	dirs := sortedNames(candidates)
	sort.Slice(dirs, func(i, j int) bool {
		a, b := strings.Count(dirs[i], string(filepath.Separator)), strings.Count(dirs[j], string(filepath.Separator))
		if a == b {
			return dirs[i] < dirs[j]
		}
		return a > b
	})
	for _, dir := range dirs {
		if protected[dir] {
			continue
		}
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		empty := true
		for _, entry := range entries {
			if !removed[filepath.Join(dir, entry.Name())] {
				empty = false
				break
			}
		}
		if empty {
			p.directories = append(p.directories, dir)
			removed[dir] = true
		}
	}
	return nil
}

func removeEmptyDirectory(root, dir string, remove func(string) error) error {
	if dir == root || !within(root, dir) {
		return fmt.Errorf("directory cleanup cannot remove repo root or escape it: %s", dir)
	}
	resolved, err := resolvePath(dir)
	if err != nil {
		return err
	}
	if resolved != dir {
		return fmt.Errorf("directory redirected after preparation: %s -> %s", dir, resolved)
	}
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("directory changed after preparation: %s", dir)
	}
	// Rmdir only removes empty directories, preserving anything that appeared
	// after preparation. It cannot unlink a replacement file or symlink.
	err = remove(dir)
	if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) || os.IsNotExist(err) {
		return nil
	}
	return err
}
