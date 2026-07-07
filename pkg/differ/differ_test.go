package differ

import (
	"testing"

	"github.com/kriipke/driftmap/internal/core"
)

func TestLoadYAMLMap(t *testing.T) {
	t.Run("valid mapping", func(t *testing.T) {
		m, err := LoadYAMLMap([]byte("a: 1\nb: two\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["a"] != 1 {
			t.Errorf("a = %v, want 1", m["a"])
		}
		if m["b"] != "two" {
			t.Errorf("b = %v, want two", m["b"])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		m, err := LoadYAMLMap([]byte(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("null document", func(t *testing.T) {
		m, err := LoadYAMLMap([]byte("null\n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("top-level list is an error, not a panic", func(t *testing.T) {
		if _, err := LoadYAMLMap([]byte("- a\n- b\n")); err == nil {
			t.Fatal("expected error for top-level list, got nil")
		}
	})

	t.Run("top-level scalar is an error, not a panic", func(t *testing.T) {
		if _, err := LoadYAMLMap([]byte("hello\n")); err == nil {
			t.Fatal("expected error for top-level scalar, got nil")
		}
	})

	t.Run("invalid YAML is an error", func(t *testing.T) {
		if _, err := LoadYAMLMap([]byte("a: [1, 2\n")); err == nil {
			t.Fatal("expected error for malformed YAML, got nil")
		}
	})
}

// byName indexes a diff slice by variable name for readable assertions.
func byName(diffs []core.VariableDiff) map[string]core.VariableDiff {
	m := make(map[string]core.VariableDiff, len(diffs))
	for _, d := range diffs {
		m[d.Name] = d
	}
	return m
}

func mustLoad(t *testing.T, s string) map[string]interface{} {
	t.Helper()
	m, err := LoadYAMLMap([]byte(s))
	if err != nil {
		t.Fatalf("LoadYAMLMap(%q): %v", s, err)
	}
	return m
}

func TestDiff(t *testing.T) {
	a := mustLoad(t, `
same: 1
changedKey: old
removedKey: gone
featureFlags:
  enableBeta: false
list:
  - name: alpha
`)
	b := mustLoad(t, `
same: 1
changedKey: new
addedKey: fresh
featureFlags:
  enableBeta: true
list:
  - name: beta
`)

	diffs := byName(Diff(a, b))

	// Unchanged keys must not appear.
	if _, ok := diffs["same"]; ok {
		t.Errorf("unchanged key 'same' should not be in diff")
	}

	if d, ok := diffs["changedKey"]; !ok {
		t.Errorf("changedKey missing from diff")
	} else {
		if d.Status != "changed" {
			t.Errorf("changedKey status = %q, want changed", d.Status)
		}
		if d.Default != "old" || d.Value != "new" {
			t.Errorf("changedKey = %v -> %v, want old -> new", d.Default, d.Value)
		}
	}

	if d, ok := diffs["addedKey"]; !ok {
		t.Errorf("addedKey missing from diff")
	} else {
		if d.Status != "added" {
			t.Errorf("addedKey status = %q, want added", d.Status)
		}
		if d.Default != nil || d.Value != "fresh" {
			t.Errorf("addedKey = %v -> %v, want <nil> -> fresh", d.Default, d.Value)
		}
	}

	if d, ok := diffs["removedKey"]; !ok {
		t.Errorf("removedKey missing from diff")
	} else {
		if d.Status != "removed" {
			t.Errorf("removedKey status = %q, want removed", d.Status)
		}
		if d.Default != "gone" || d.Value != nil {
			t.Errorf("removedKey = %v -> %v, want gone -> <nil>", d.Default, d.Value)
		}
	}

	// Nested map keys are flattened with dotted paths.
	if d, ok := diffs["featureFlags.enableBeta"]; !ok {
		t.Errorf("nested key featureFlags.enableBeta missing from diff")
	} else if d.Status != "changed" || d.Default != false || d.Value != true {
		t.Errorf("featureFlags.enableBeta = %v -> %v (%s), want false -> true (changed)", d.Default, d.Value, d.Status)
	}

	// List elements are flattened with index keys.
	if d, ok := diffs["list[0].name"]; !ok {
		t.Errorf("list key list[0].name missing from diff")
	} else if d.Status != "changed" || d.Default != "alpha" || d.Value != "beta" {
		t.Errorf("list[0].name = %v -> %v (%s), want alpha -> beta (changed)", d.Default, d.Value, d.Status)
	}
}

func TestDiffEmpty(t *testing.T) {
	diffs := Diff(map[string]interface{}{}, map[string]interface{}{})
	if len(diffs) != 0 {
		t.Errorf("expected no diffs for two empty maps, got %v", diffs)
	}
}

func TestFlattenYAML(t *testing.T) {
	in := map[string]interface{}{
		"a": map[string]interface{}{
			"b": 1,
		},
		"list": []interface{}{"x", "y"},
	}
	out := map[string]interface{}{}
	flattenYAML("", in, out)

	if out["a.b"] != 1 {
		t.Errorf("a.b = %v, want 1", out["a.b"])
	}
	if out["list[0]"] != "x" {
		t.Errorf("list[0] = %v, want x", out["list[0]"])
	}
	if out["list[1]"] != "y" {
		t.Errorf("list[1] = %v, want y", out["list[1]"])
	}
}
