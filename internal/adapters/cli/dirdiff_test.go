package cli

import (
	"fmt"
	"testing"
)

// mapLoader returns a loader func backed by an in-memory filename->contents map.
func mapLoader(files map[string]string) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		data, ok := files[name]
		if !ok {
			return nil, fmt.Errorf("no such file: %s", name)
		}
		return []byte(data), nil
	}
}

func TestDiffFileSets(t *testing.T) {
	sideA := map[string]string{
		"changed.yaml": "key: old\n",
		"removed.yaml": "gone: yes\n",
	}
	sideB := map[string]string{
		"changed.yaml": "key: new\n",
		"added.yaml":   "brand: new\n",
	}

	// Pass inputs unsorted to exercise deterministic sorting.
	filesA := []string{"removed.yaml", "changed.yaml"}
	filesB := []string{"added.yaml", "changed.yaml"}

	changed, added, removed, err := diffFileSets(
		filesA, filesB,
		mapLoader(sideA), mapLoader(sideB),
	)
	if err != nil {
		t.Fatalf("diffFileSets returned error: %v", err)
	}

	// One changed file, with the expected variable diff.
	if len(changed) != 1 {
		t.Fatalf("changed = %v, want exactly one file", changed)
	}
	if changed[0].File != "changed.yaml" {
		t.Errorf("changed file = %q, want changed.yaml", changed[0].File)
	}
	if len(changed[0].Diffs) != 1 {
		t.Fatalf("changed.yaml diffs = %v, want exactly one", changed[0].Diffs)
	}
	d := changed[0].Diffs[0]
	if d.Name != "key" || d.Status != "changed" || d.Default != "old" || d.Value != "new" {
		t.Errorf("diff = %+v, want key old->new changed", d)
	}

	// One added file.
	if len(added) != 1 || added[0] != "added.yaml" {
		t.Errorf("added = %v, want [added.yaml]", added)
	}

	// One removed file.
	if len(removed) != 1 || removed[0] != "removed.yaml" {
		t.Errorf("removed = %v, want [removed.yaml]", removed)
	}
}

func TestDiffFileSetsSortsDeterministically(t *testing.T) {
	sideA := map[string]string{
		"c.yaml": "k: 1\n",
		"a.yaml": "k: 1\n",
		"b.yaml": "k: 1\n",
	}
	sideB := map[string]string{} // everything removed

	filesA := []string{"c.yaml", "a.yaml", "b.yaml"}

	_, added, removed, err := diffFileSets(
		filesA, nil,
		mapLoader(sideA), mapLoader(sideB),
	)
	if err != nil {
		t.Fatalf("diffFileSets returned error: %v", err)
	}
	if len(added) != 0 {
		t.Errorf("added = %v, want none", added)
	}
	want := []string{"a.yaml", "b.yaml", "c.yaml"}
	if len(removed) != len(want) {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	for i := range want {
		if removed[i] != want[i] {
			t.Errorf("removed[%d] = %q, want %q (removed=%v)", i, removed[i], want[i], removed)
		}
	}
}
