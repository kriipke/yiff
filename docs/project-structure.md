## Project Structure

| Legacy Functionality                    | New File/Location                                   | Notes                                                    |
|-----------------------------------------|-----------------------------------------------------|----------------------------------------------------------|
| `VariableDiff` struct                   | `internal/core/types.go`                            | Central domain type                                      |
| flattening/conversion/format utils      | `pkg/differ/differ.go` (and/or `pkg/utils/sort.go`) | Helpers for YAML, string formatting, etc                 |
| YAML diff logic (diff calculation)      | `pkg/differ/differ.go`                              | Business logic; returns list of `VariableDiff`           |
| CLI flags, file IO, color printing      | `internal/adapters/cli/root.go`                 | CLI-specific, uses differ package                        |
| CLI entrypoint (main)                   | `cmd/cli/main.go`                                   | Only calls into `internal/adapters/cli`                  |
| Output formatting (color, YAML output)  | `internal/adapters/cli/root.go` and/or `pkg/differ/differ.go` | If common, can live in differ; else put in CLI           |
| YAML marshaling                         | Use `yaml.v3` in appropriate locations              | Both CLI and web handlers can marshal output as needed   |
| Web handler (new)                       | `internal/adapters/http/handlers.go`                | Accepts POST, calls core differ, returns JSON or YAML    |
