package toggle

import "testing"

func TestParse(t *testing.T) {
	for _, args := range [][]string{{"list"}, {"status"}, {"on", "Review"}, {"off", "review", "--dry-run"}, {"on", "--dry-run", "review"}, {"on", "--", "-review"}, {"--help"}} {
		if _, err := parse(args); err != nil {
			t.Errorf("%v: %v", args, err)
		}
	}
	for _, args := range [][]string{nil, {"unknown"}, {"list", "review"}, {"status", "--dry-run"}, {"on"}, {"off", "a", "b"}, {"on", "../review"}, {"on", "."}, {"off", "--bad", "review"}} {
		if _, err := parse(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
