package toggle

import (
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
