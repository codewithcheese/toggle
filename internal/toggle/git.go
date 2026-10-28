package toggle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type repo struct {
	root, gitDir, commonDir, excludePath, recordPath string
}

func git(ctx context.Context, cwd string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return output, fmt.Errorf("git %s: %s: %w", args[0], strings.TrimSpace(stderr.String()), err)
	}
	return output, nil
}

func findRepo(ctx context.Context, cwd string) (*repo, error) {
	output, err := git(ctx, cwd, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 128 {
			return nil, nil
		}
		return nil, err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSuffix(string(output), "\n"))
	if err != nil {
		return nil, err
	}
	return &repo{root: root}, nil
}

func (r *repo) metadata(ctx context.Context) error {
	queries := []struct {
		args []string
		to   *string
	}{
		{[]string{"--absolute-git-dir"}, &r.gitDir},
		{[]string{"--path-format=absolute", "--git-common-dir"}, &r.commonDir},
		{[]string{"--path-format=absolute", "--git-path", "info/exclude"}, &r.excludePath},
		{[]string{"--path-format=absolute", "--git-path", "toggle.json"}, &r.recordPath},
	}
	for _, q := range queries {
		out, err := git(ctx, r.root, nil, append([]string{"rev-parse"}, q.args...)...)
		if err != nil {
			return err
		}
		*q.to = filepath.Clean(strings.TrimSuffix(string(out), "\n"))
	}
	return nil
}

func (r *repo) requireRoot(cwd string) error {
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return err
	}
	if resolved != r.root {
		return fmt.Errorf("run on/off and previews from the Git repo root: %s", r.root)
	}
	return nil
}

func (r *repo) tracked(ctx context.Context, paths ...string) (string, error) {
	out, err := git(ctx, r.root, nil, "ls-files", "--cached", "-z")
	if err != nil {
		return "", err
	}
	for _, tracked := range bytes.Split(out, []byte{0}) {
		for _, path := range paths {
			rel, err := filepath.Rel(r.root, path)
			if err != nil {
				return "", err
			}
			if string(tracked) == filepath.ToSlash(rel) {
				return path, nil
			}
		}
	}
	return "", nil
}

type ignoreMatch struct {
	source, line, pattern string
}

func (r *repo) ignore(ctx context.Context, relative string) (ignoreMatch, error) {
	out, err := git(ctx, r.root, append([]byte(filepath.ToSlash(relative)), 0), "check-ignore", "--no-index", "-v", "-z", "--stdin")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return ignoreMatch{}, nil
		}
		return ignoreMatch{}, err
	}
	parts := bytes.Split(out, []byte{0})
	if len(parts) < 4 {
		return ignoreMatch{}, fmt.Errorf("invalid Git ignore response for %s", relative)
	}
	return ignoreMatch{source: string(parts[0]), line: string(parts[1]), pattern: string(parts[2])}, nil
}

// A new exact info/exclude entry follows earlier info/exclude entries and
// outranks global excludes. Only a per-directory .gitignore negation that
// currently wins can override it. Git supplies the winning rule, including
// the parent-directory traversal semantics, without changing any ignore file.
func (r *repo) exclusionAllowed(ctx context.Context, relative string) error {
	match, err := r.ignore(ctx, relative)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(match.pattern, "!") || filepath.IsAbs(match.source) || filepath.Base(match.source) != ".gitignore" {
		return nil
	}
	dir := filepath.Dir(match.source)
	if dir == "." || within(filepath.Join(r.root, dir), filepath.Join(r.root, relative)) {
		return fmt.Errorf("local exclusion is overridden by %s:%s (%s)", match.source, match.line, match.pattern)
	}
	return nil
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}
