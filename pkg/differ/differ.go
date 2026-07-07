package differ

import (
	"fmt"
	"reflect"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/kriipke/driftmap/internal/core"
)

// Re-export for convenience, so cli can use differ.VariableDiff
type VariableDiff = core.VariableDiff

func flattenYAML(prefix string, v interface{}, out map[string]interface{}) {
	switch node := v.(type) {
	case map[string]interface{}:
		for k, val := range node {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenYAML(key, val, out)
		}
	case []interface{}:
		for i, val := range node {
			key := fmt.Sprintf("%s[%d]", prefix, i)
			flattenYAML(key, val, out)
		}
	default:
		out[prefix] = node
	}
}

func convertToStringMap(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[fmt.Sprint(k)] = convertToStringMap(v)
		}
		return m2
	case []interface{}:
		for i2, v := range x {
			x[i2] = convertToStringMap(v)
		}
	}
	return i
}

// Accepts two YAML documents (arbitrary structure, already parsed), returns diff
func Diff(a, b map[string]interface{}) []core.VariableDiff {
	flatA := map[string]interface{}{}
	flatB := map[string]interface{}{}
	flattenYAML("", a, flatA)
	flattenYAML("", b, flatB)

	keys := map[string]struct{}{}
	for k := range flatA {
		keys[k] = struct{}{}
	}
	for k := range flatB {
		keys[k] = struct{}{}
	}
	var allKeys []string
	for k := range keys {
		allKeys = append(allKeys, k)
	}
	sort.Strings(allKeys)

	var diffs []core.VariableDiff

	for _, k := range allKeys {
		va, oka := flatA[k]
		vb, okb := flatB[k]
		switch {
		case oka && okb && !reflect.DeepEqual(va, vb):
			diffs = append(diffs, core.VariableDiff{
				Name:    k,
				Default: va,
				Value:   vb,
				Status:  "changed",
			})
		case oka && !okb:
			diffs = append(diffs, core.VariableDiff{
				Name:    k,
				Default: va,
				Value:   nil,
				Status:  "removed",
			})
		case !oka && okb:
			diffs = append(diffs, core.VariableDiff{
				Name:    k,
				Default: nil,
				Value:   vb,
				Status:  "added",
			})
		}
	}
	return diffs
}

// LoadYAMLMap parses a YAML document into a string-keyed map. The document's
// top level must be a mapping (as Helm values files and vaultsync secret files
// are); an empty document is treated as an empty mapping. A non-mapping root
// (list or scalar) returns an error rather than panicking.
func LoadYAMLMap(data []byte) (map[string]interface{}, error) {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		// Empty or explicitly-null document: treat as an empty mapping.
		return map[string]interface{}{}, nil
	}
	m, ok := convertToStringMap(raw).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("top-level YAML must be a mapping, got %T", raw)
	}
	return m, nil
}
