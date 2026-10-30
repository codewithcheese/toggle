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
	root, gitDir, commonDir, excludePath, indexPath string
	query                                           func(context.Context, string, []byte, ...string) ([]byte, error)
}

func (r *repo) command(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	if r.query != nil {
		return r.query(ctx, r.root, input, args...)
	}
	return git(ctx, r.root, input, args...)
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
		{[]string{"--path-format=absolute", "--git-path", "index"}, &r.indexPath},
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

func (r *repo) index(ctx context.Context) (map[string]bool, error) {
	out, err := r.command(ctx, nil, "ls-files", "--cached", "-z")
	if err != nil {
		return nil, err
	}
	index := map[string]bool{}
	for _, path := range bytes.Split(out, []byte{0}) {
		if len(path) != 0 {
			index[string(path)] = true
		}
	}
	return index, nil
}

type ignoreMatch struct {
	source, line, pattern string
}

func (r *repo) ignore(ctx context.Context, relative string) (ignoreMatch, error) {
	matches, err := r.ignoreMany(ctx, append([]byte(filepath.ToSlash(relative)), 0))
	return matches[filepath.ToSlash(relative)], err
}

func (r *repo) ignoreMany(ctx context.Context, input []byte) (map[string]ignoreMatch, error) {
	matches := map[string]ignoreMatch{}
	out, err := r.command(ctx, input, "check-ignore", "--no-index", "-v", "-z", "--stdin")
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return matches, nil
		}
		return nil, err
	}
	parts := bytes.Split(out, []byte{0})
	if (len(parts)-1)%4 != 0 {
		return nil, fmt.Errorf("invalid Git ignore response")
	}
	for i := 0; i+3 < len(parts); i += 4 {
		matches[string(parts[i+3])] = ignoreMatch{source: string(parts[i]), line: string(parts[i+1]), pattern: string(parts[i+2])}
	}
	return matches, nil
}

func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}
