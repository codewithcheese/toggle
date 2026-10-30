package toggle

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type content struct {
	data    []byte
	hash    string
	exec    uint32
	missing bool
	size    int64
	invalid bool
}

func readContent(path string) (content, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return content{missing: true}, nil
	}
	if err != nil {
		return content{}, err
	}
	if !info.Mode().IsRegular() {
		return content{invalid: true}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return content{}, err
	}
	return fingerprint(data, info), nil
}

func fingerprint(data []byte, info os.FileInfo) content {
	return content{data: data, hash: fmt.Sprintf("%x", sha256.Sum256(data)), exec: uint32(info.Mode().Perm() & 0111), size: info.Size()}
}

func (c content) matches(other content) bool {
	return !c.invalid && !other.invalid && c.missing == other.missing && c.hash == other.hash && c.exec == other.exec
}

// resolvePath follows existing symlinks, including dangling final symlinks,
// while allowing the remaining destination components to be absent.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parts := strings.Split(abs, string(filepath.Separator))
	current := string(filepath.Separator)
	links := 0
	for len(parts) != 0 {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			current = filepath.Dir(current)
			continue
		}
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if errors.Is(err, os.ErrNotExist) {
			// A missing component cannot be traversed by .. on the actual
			// filesystem. Do not guess a different, lexically cleaned target.
			for _, pending := range parts {
				if pending == ".." {
					return "", fmt.Errorf("cannot resolve .. through absent directory: %s", next)
				}
			}
			return filepath.Join(append([]string{next}, parts...)...), nil
		}
		if err != nil {
			return "", err
		}
		// Lstat accepts case aliases on some filesystems. Use the actual
		// directory entry so boundaries and destination ownership share one
		// identity without conflating distinct names on case-sensitive disks.
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		exact := false
		for _, entry := range entries {
			if entry.Name() == part {
				exact = true
				break
			}
		}
		if !exact {
			for _, entry := range entries {
				if !strings.EqualFold(entry.Name(), part) {
					continue
				}
				actual, err := entry.Info()
				if err != nil {
					return "", err
				}
				if os.SameFile(info, actual) {
					next = filepath.Join(current, entry.Name())
					break
				}
			}
		}
		if info.Mode()&os.ModeSymlink == 0 {
			current = next
			continue
		}
		links++
		if links > 256 {
			return "", fmt.Errorf("symlink loop at %s", path)
		}
		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			current = string(filepath.Separator)
		}
		// Resolve the target's components in order: cleaning link/../file
		// before following link would change its filesystem meaning.
		parts = append(strings.Split(target, string(filepath.Separator)), parts...)
	}
	return current, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (r *repo) destination(declared string) (string, error) {
	if filepath.IsAbs(declared) || filepath.Clean(declared) == "." {
		return "", fmt.Errorf("destination must be a repo-relative file: %s", declared)
	}
	path := filepath.Join(r.root, declared)
	resolved, err := resolvePath(path)
	if err != nil {
		return "", err
	}
	if !within(r.root, path) || !within(r.root, resolved) {
		return "", fmt.Errorf("destination escapes repo: %s -> %s", path, resolved)
	}
	for _, target := range []string{path, resolved} {
		relative, err := filepath.Rel(r.root, target)
		if err != nil {
			return "", err
		}
		for _, component := range strings.Split(relative, string(filepath.Separator)) {
			if r.reservedName(component, ".git") {
				return "", fmt.Errorf("reserved .git destination: %s -> %s", path, resolved)
			}
		}
		if r.reservedName(filepath.Base(target), ".gitignore") {
			return "", fmt.Errorf("reserved .gitignore destination: %s -> %s", path, resolved)
		}
	}
	for _, reserved := range []string{filepath.Join(r.root, ".git"), r.gitDir, r.commonDir} {
		if reserved == "" {
			continue
		}
		canonical, err := resolvePath(reserved)
		if err != nil {
			return "", err
		}
		if within(reserved, path) || within(reserved, resolved) || within(canonical, resolved) {
			return "", fmt.Errorf("reserved destination: %s -> %s (%s)", path, resolved, reserved)
		}
	}
	return resolved, nil
}

func (r *repo) reservedName(name, reserved string) bool {
	if name == reserved {
		return true
	}
	if !strings.EqualFold(name, reserved) {
		return false
	}
	// The repo's .git entry also identifies case aliases for not-yet-existing
	// control files, where there is no destination entry to canonicalize yet.
	exact, err := os.Lstat(filepath.Join(r.root, ".git"))
	if err != nil {
		return false
	}
	alias, err := os.Lstat(filepath.Join(r.root, ".GIT"))
	return err == nil && os.SameFile(exact, alias)
}

type sourceFile struct {
	relative, path string
	resolved, root string
	content        content
}

// sourceEntries enumerates and validates paths without reading file contents.
func sourceEntries(f feature) ([]sourceFile, error) {
	root, err := resolvePath(filepath.Join(f.dir, "source"))
	if err != nil {
		return nil, err
	}
	var files []sourceFile
	ancestors := map[string]bool{}
	var walk func(string) error
	walk = func(relative string) error {
		path := filepath.Join(f.dir, "source", relative)
		resolved, err := resolvePath(path)
		if err != nil {
			return err
		}
		if !within(root, resolved) {
			return fmt.Errorf("source escapes source/: %s -> %s", path, resolved)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return fmt.Errorf("source %s: %w", path, err)
		}
		if info.IsDir() {
			if ancestors[resolved] {
				return fmt.Errorf("source directory symlink cycle: %s", path)
			}
			ancestors[resolved] = true
			defer delete(ancestors, resolved)
			entries, err := os.ReadDir(resolved)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := walk(filepath.Join(relative, entry.Name())); err != nil {
					return err
				}
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("source is not an ordinary file: %s", path)
		}
		files = append(files, sourceFile{relative: relative, path: path, resolved: resolved, root: root, content: content{exec: uint32(info.Mode().Perm() & 0111), size: info.Size()}})
		return nil
	}
	err = walk("")
	return files, err
}
