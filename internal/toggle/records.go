package toggle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type installedFile struct {
	Relative   string `json:"source_relative"`
	Declared   string `json:"declared"`
	Resolved   string `json:"resolved"`
	Hash       string `json:"hash"`
	Executable uint32 `json:"executable"`
}

type installation struct {
	Source    string          `json:"source"`
	Files     []installedFile `json:"files"`
	Exclusion string          `json:"exclusion_block"`
}

type records struct {
	Features map[string]installation `json:"features"`
}

func loadRecords(path string) (records, []byte, error) {
	r := records{Features: map[string]installation{}}
	data, err := readOptional(path)
	if err != nil {
		return r, nil, fmt.Errorf("read bookkeeping %s: %w", path, err)
	}
	if data == nil {
		return r, nil, nil
	}
	if err := json.Unmarshal(data, &r); err != nil || r.Features == nil {
		return r, data, fmt.Errorf("invalid bookkeeping at %s; inspect before proceeding: %v", path, err)
	}
	return r, data, nil
}

func recordBytes(r records) ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	return append(data, '\n'), err
}

// atomicWrite changes one document, without promising a multi-file transaction.
func atomicWrite(path string, data []byte) error {
	resolved, err := resolvePath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0755); err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(resolved); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(resolved), ".toggle-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), resolved)
}
