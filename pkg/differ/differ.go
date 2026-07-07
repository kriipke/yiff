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

// Optionally: Add a LoadYAML helper if you want to share file loading (or move to adapters/io)
func LoadYAMLMap(data []byte) (map[string]interface{}, error) {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	m := convertToStringMap(raw)
	return m.(map[string]interface{}), nil
}
