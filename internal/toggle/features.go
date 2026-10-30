package toggle

import (
	"sort"
)

type feature struct {
	name, dir string
	store     string
	local     bool
}

func sortedNames[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
