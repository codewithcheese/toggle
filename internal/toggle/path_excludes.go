package toggle

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

const pathBlockStart = "\n# toggle paths begin\n"
const pathBlockEnd = "# toggle paths end\n"

type managedExclusions struct {
	prefix, suffix []byte
	rules          map[string]bool
}

func parseExclusions(data []byte) (managedExclusions, error) {
	block := managedExclusions{prefix: bytes.Clone(data), rules: map[string]bool{}}
	starts, ends := bytes.Count(data, []byte("# toggle paths begin")), bytes.Count(data, []byte("# toggle paths end"))
	if starts == 0 && ends == 0 {
		return block, nil
	}
	start := bytes.Index(data, []byte(pathBlockStart))
	end := bytes.Index(data, []byte(pathBlockEnd))
	if starts != 1 || ends != 1 || start < 0 || end < start+len(pathBlockStart) {
		return block, fmt.Errorf("malformed toggle path block; inspect local exclusions")
	}
	body := string(data[start+len(pathBlockStart) : end])
	if body != "" && !strings.HasSuffix(body, "\n") {
		return block, fmt.Errorf("malformed toggle path rule terminator")
	}
	for _, rule := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		if rule == "" && body == "" {
			continue
		}
		if err := validateExactRule(rule); err != nil {
			return block, err
		}
		block.rules[rule] = true
	}
	block.prefix = bytes.Clone(data[:start])
	block.suffix = bytes.Clone(data[end+len(pathBlockEnd):])
	return block, nil
}

func validateExactRule(rule string) error {
	if !strings.HasPrefix(rule, "/") {
		return fmt.Errorf("invalid exact rule in toggle path block: %q", rule)
	}
	var decoded strings.Builder
	for i := 1; i < len(rule); i++ {
		if rule[i] == '\\' {
			i++
			if i >= len(rule) {
				return fmt.Errorf("incomplete escape in toggle path rule: %q", rule)
			}
		}
		decoded.WriteByte(rule[i])
	}
	path := filepath.FromSlash(decoded.String())
	exact, err := exactExclusion(path)
	if err != nil || !relativeFile(path) || exact != rule {
		return fmt.Errorf("non-exact rule in toggle path block: %q", rule)
	}
	return nil
}

func updateExclusions(data []byte, paths []string, action string) ([]byte, error) {
	block, err := parseExclusions(data)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		rule, err := exactExclusion(path)
		if err != nil {
			return nil, err
		}
		if action == "on" {
			block.rules[rule] = true
		} else {
			delete(block.rules, rule)
		}
	}
	result := bytes.Clone(block.prefix)
	if len(block.rules) != 0 {
		result = append(result, pathBlockStart...)
		result = append(result, strings.Join(sortedNames(block.rules), "\n")...)
		result = append(result, '\n')
		result = append(result, pathBlockEnd...)
	}
	return append(result, block.suffix...), nil
}
