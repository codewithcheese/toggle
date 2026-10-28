package toggle

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

func exactExclusion(relative string) (string, error) {
	if strings.ContainsAny(relative, "\r\n") {
		return "", fmt.Errorf("path cannot be represented as an exact Git exclusion: %q", relative)
	}
	escape := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]", " ", "\\ ", "#", "\\#", "!", "\\!")
	return "/" + escape.Replace(filepath.ToSlash(relative)), nil
}

func blockMarker(name string) string {
	return "# toggle begin " + hex.EncodeToString([]byte(name)) + "\n"
}

func exclusionBlock(name string, paths []string) string {
	return "\n" + blockMarker(name) + strings.Join(paths, "\n") + "\n# toggle end " + hex.EncodeToString([]byte(name)) + "\n"
}

func addExclusion(old []byte, name, block string) ([]byte, error) {
	if bytes.Contains(old, []byte(blockMarker(name))) {
		return nil, fmt.Errorf("existing exclusion block for %s requires manual inspection", name)
	}
	return append(bytes.Clone(old), []byte(block)...), nil
}

func removeExclusion(old []byte, name, block string) ([]byte, error) {
	if !bytes.Contains(old, []byte(blockMarker(name))) {
		return bytes.Clone(old), nil
	}
	if block == "" || bytes.Count(old, []byte(block)) != 1 {
		return nil, fmt.Errorf("exclusion block for %s changed; resolve it before off", name)
	}
	return bytes.Replace(old, []byte(block), nil, 1), nil
}
