package toggle

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	c, err := loadConfig(path)
	if err != nil || len(c.Stores) != 0 {
		t.Fatal(c, err)
	}
	assertAbsent(t, path)
	for _, data := range []string{
		`{}`, `{"stores":{}}`,
		`{"stores":{"local":` + strconv.Quote(dir) + `,"Team Notes":` + strconv.Quote(dir) + `}}`,
	} {
		put(t, path, data, 0644)
		before := snapshot(t, dir)
		if _, err := loadConfig(path); err != nil {
			t.Fatal(data, err)
		}
		if !reflect.DeepEqual(before, snapshot(t, dir)) {
			t.Fatal("config read changed files")
		}
	}
	for _, data := range []string{`null`, `[]`, `true`, `{`, `{} {}`, `{"stores":null}`, `{"stores":[]}`, `{"stores":{"":"/tmp"}}`, `{"stores":{"team":false}}`, `{"stores":{"team":"relative/path"}}`, `{"stores":{"team":""}}`, `{"storres":{}}`} {
		put(t, path, data, 0644)
		if _, err := loadConfig(path); err == nil || !strings.Contains(err.Error(), path) {
			t.Fatal("invalid config accepted", data, err)
		}
	}
	os.Remove(path)
}
