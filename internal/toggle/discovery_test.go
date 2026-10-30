package toggle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoLookup(t *testing.T) {
	r := testRepo(t)
	sub := filepath.Join(r.root, "docs")
	os.Mkdir(sub, 0755)
	for _, dir := range []string{r.root, sub} {
		found, err := findRepo(context.Background(), dir)
		if err != nil || found == nil || found.root != r.root {
			t.Fatalf("%s: %v %v", dir, found, err)
		}
	}
	if err := r.requireRoot(sub); err == nil {
		t.Fatal("accepted subdirectory")
	}
	dir := t.TempDir()
	found, err := findRepo(context.Background(), dir)
	entries, _ := os.ReadDir(dir)
	if err != nil || found != nil || len(entries) != 0 {
		t.Fatalf("outside: %v %v %v", found, err, entries)
	}
}
