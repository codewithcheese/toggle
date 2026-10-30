package toggle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (f feature) selector() string {
	if f.local {
		return "--local"
	}
	return "--store " + shellQuote(f.store)
}

func (f feature) label() string { return shellQuote(f.name) + " " + f.selector() }

func shellQuote(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_-./", r))
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func discoverRoot(root, store string, local, optional bool) ([]feature, error) {
	entries, err := os.ReadDir(root)
	if optional && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("source %s (%s): %w", store, root, err)
	}
	var features []feature
	for _, entry := range entries {
		dir := filepath.Join(root, entry.Name())
		info, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("source %s: %w", dir, err)
		}
		if !info.IsDir() {
			continue
		}
		source, err := os.Stat(filepath.Join(dir, "source"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("source %s: %w", dir, err)
		}
		if source.IsDir() {
			features = append(features, feature{name: entry.Name(), dir: dir, store: store, local: local})
		}
	}
	return features, nil
}

func definitions(c config, r *repo, cmd command) ([]feature, error) {
	if cmd.local {
		if r == nil {
			return nil, fmt.Errorf("--local requires a Git repo")
		}
		return discoverRoot(filepath.Join(r.root, ".toggle"), "local", true, true)
	}
	if cmd.store != "" {
		root, ok := c.Stores[cmd.store]
		if !ok {
			return nil, fmt.Errorf("unknown store %q in config", cmd.store)
		}
		return discoverRoot(root, cmd.store, false, false)
	}
	var all []feature
	if r != nil {
		local, err := discoverRoot(filepath.Join(r.root, ".toggle"), "local", true, true)
		if err != nil {
			return nil, err
		}
		all = append(all, local...)
	}
	for _, name := range sortedNames(c.Stores) {
		features, err := discoverRoot(c.Stores[name], name, false, false)
		if err != nil {
			return nil, err
		}
		all = append(all, features...)
	}
	return all, nil
}

func selectFeature(features []feature, name string) (feature, error) {
	var matches []feature
	for _, f := range features {
		if f.name == name {
			matches = append(matches, f)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return feature{}, fmt.Errorf("feature %q is unavailable in the selected sources", name)
	}
	var choices []string
	for _, f := range matches {
		choices = append(choices, f.selector()+" ("+f.dir+")")
	}
	return feature{}, fmt.Errorf("feature %q is ambiguous; select %s", name, strings.Join(choices, " or "))
}
