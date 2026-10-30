package toggle

import (
	"fmt"
	"path/filepath"
	"strings"
)

type filePaths struct{ Relative, Declared, Resolved string }
type fileChange struct {
	file               filePaths
	before, desired    content
	source, sourceRoot string
}

type plan struct {
	action, name                                 string
	repo                                         *repo
	definition                                   feature
	protected                                    []string
	changes                                      []fileChange
	directories, conflicts                       []string
	oldExclude, newExclude                       []byte
	total, matching, missing, different, present int
	invalidPath, noop                            bool
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
func overlap(a, b string) bool { return within(a, b) || within(b, a) }
