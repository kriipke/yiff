package http

import (
	"encoding/json"
	"net/http"

	"github.com/kriipke/driftmap/pkg/differ"
)

type diffRequest struct {
	A string `json:"a"` // YAML document as string
	B string `json:"b"`
}

func DiffHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req diffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	yamlA, err := differ.LoadYAMLMap([]byte(req.A))
	if err != nil {
		http.Error(w, "Invalid YAML for 'a': "+err.Error(), http.StatusBadRequest)
		return
	}
	yamlB, err := differ.LoadYAMLMap([]byte(req.B))
	if err != nil {
		http.Error(w, "Invalid YAML for 'b': "+err.Error(), http.StatusBadRequest)
		return
	}
	diffs := differ.Diff(yamlA, yamlB)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"variables": diffs,
	})
}
