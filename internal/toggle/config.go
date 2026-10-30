package toggle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	Stores map[string]string `json:"stores"`
}

func loadConfig(path string) (config, error) {
	c := config{Stores: map[string]string{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("config %s: %w", path, err)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return c, fmt.Errorf("config %s: expected a JSON object", path)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("config %s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return c, fmt.Errorf("config %s: expected one JSON object", path)
	}
	if c.Stores == nil {
		return c, fmt.Errorf("config %s: stores must be an object", path)
	}
	for name, root := range c.Stores {
		if strings.TrimSpace(name) == "" || root == "" || !filepath.IsAbs(root) {
			return c, fmt.Errorf("config %s: store %q requires a nonempty name and absolute directory path: %q", path, name, root)
		}
		c.Stores[name] = filepath.Clean(root)
	}
	return c, nil
}
