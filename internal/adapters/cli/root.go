package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kriipke/driftmap/pkg/differ"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var (
	fromRef      string
	toRef        string
	diffPath     string
	outputFormat string
)

var rootCmd = &cobra.Command{
	Use:   "driftmap [A] [B]",
	Short: "Detect configuration drift between two sets of YAML",
	Long: `DriftMap reports the drift between two sets of YAML configuration.

It works on:
  * Helm chart values -- compare two values.yaml files
    (e.g. values-dev.yaml vs values-prod.yaml).
  * Vault secrets -- compare two directories of secrets exported as YAML
    files with vaultsync (github.com/kriipke/vaultsync), e.g. a before/
    and after/ snapshot.

A and B may both be files (single-file diff) or both be directories
(recursive per-file diff). Alternatively, use --from/--to/--path to compare
all YAML files under a path between two git refs.`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Directory diff mode (git refs)
		if fromRef != "" && toRef != "" && diffPath != "" {
			return runGitRefDirDiff(fromRef, toRef, diffPath, outputFormat)
		}

		if len(args) == 2 {
			aIsDir, err := isDir(args[0])
			if err != nil {
				return fmt.Errorf("Failed to stat %s: %w", args[0], err)
			}
			bIsDir, err := isDir(args[1])
			if err != nil {
				return fmt.Errorf("Failed to stat %s: %w", args[1], err)
			}
			switch {
			case aIsDir && bIsDir:
				// Directory diff mode (local dirs, e.g. vaultsync output)
				return runDirDiff(args[0], args[1], outputFormat)
			case !aIsDir && !bIsDir:
				// Single file diff mode
				return runFileDiff(args[0], args[1], outputFormat)
			default:
				return fmt.Errorf("both arguments must be files or both directories (%s and %s differ)", args[0], args[1])
			}
		}

		// Print usage if not enough arguments
		cmd.Usage()
		return fmt.Errorf("invalid arguments")
	},
}

func init() {
	rootCmd.Flags().StringVar(&fromRef, "from", "", "Git ref/tag/commit for the base comparison (used with --to and --path)")
	rootCmd.Flags().StringVar(&toRef, "to", "", "Git ref/tag/commit for the target comparison (used with --from and --path)")
	rootCmd.Flags().StringVar(&diffPath, "path", "", "Compare all yaml files under this path between --from and --to refs")
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "shell", "Output format: 'shell' (default), 'yaml', or 'columns'")
}

// isDir reports whether path exists and is a directory.
func isDir(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

// Entrypoint for Cobra
func Execute() error {
	return rootCmd.Execute()
}

const (
	ColorReset  = "\x1b[0m"
	ColorGreen  = "\x1b[32;1m"
	ColorYellow = "\x1b[33;1m"
	ColorRed    = "\x1b[31;1m"
	ColorBold   = "\x1b[1m"
)

// Update runFileDiff signature!
func runFileDiff(fileA, fileB, outputFormat string) error {
	dataA, err := os.ReadFile(fileA)
	if err != nil {
		return fmt.Errorf("Failed to load %s: %w", fileA, err)
	}
	dataB, err := os.ReadFile(fileB)
	if err != nil {
		return fmt.Errorf("Failed to load %s: %w", fileB, err)
	}
	yamlA, err := differ.LoadYAMLMap(dataA)
	if err != nil {
		return fmt.Errorf("Failed to parse %s: %w", fileA, err)
	}
	yamlB, err := differ.LoadYAMLMap(dataB)
	if err != nil {
		return fmt.Errorf("Failed to parse %s: %w", fileB, err)
	}
	diffs := differ.Diff(yamlA, yamlB)
	return printDiffs(diffs, outputFormat)
}

// perFileDiff holds the drift found for a single file present in both sides.
type perFileDiff struct {
	File  string
	Diffs []differ.VariableDiff
}

// isYAMLFile reports whether a filename looks like a YAML file.
func isYAMLFile(name string) bool {
	return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

// --- Directory diff between two local directories (e.g. vaultsync output) ---
func runDirDiff(dirA, dirB, outputFormat string) error {
	filesA, err := listDirYAMLFiles(dirA)
	if err != nil {
		return fmt.Errorf("Failed to list files under %s: %w", dirA, err)
	}
	filesB, err := listDirYAMLFiles(dirB)
	if err != nil {
		return fmt.Errorf("Failed to list files under %s: %w", dirB, err)
	}

	changed, added, removed, err := diffFileSets(
		filesA, filesB,
		func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join(dirA, rel)) },
		func(rel string) ([]byte, error) { return os.ReadFile(filepath.Join(dirB, rel)) },
	)
	if err != nil {
		return err
	}
	return printDirSummary(changed, added, removed, outputFormat)
}

// --- Directory diff between git refs ---
func runGitRefDirDiff(fromRef, toRef, relPath, outputFormat string) error {
	filesA, err := listGitFiles(fromRef, relPath)
	if err != nil {
		return fmt.Errorf("Failed to list files for ref %s: %w", fromRef, err)
	}
	filesB, err := listGitFiles(toRef, relPath)
	if err != nil {
		return fmt.Errorf("Failed to list files for ref %s: %w", toRef, err)
	}

	changed, added, removed, err := diffFileSets(
		filesA, filesB,
		func(f string) ([]byte, error) { return gitShowFile(fromRef, f) },
		func(f string) ([]byte, error) { return gitShowFile(toRef, f) },
	)
	if err != nil {
		return err
	}
	return printDirSummary(changed, added, removed, outputFormat)
}

// diffFileSets compares two sets of YAML file keys, loading each side's contents
// via the provided readers, and classifies them into changed/added/removed.
func diffFileSets(filesA, filesB []string, loadA, loadB func(string) ([]byte, error)) (changed []perFileDiff, added, removed []string, err error) {
	setA, setB := map[string]struct{}{}, map[string]struct{}{}
	for _, f := range filesA {
		setA[f] = struct{}{}
	}
	for _, f := range filesB {
		setB[f] = struct{}{}
	}

	allFiles := map[string]struct{}{}
	for f := range setA {
		allFiles[f] = struct{}{}
	}
	for f := range setB {
		allFiles[f] = struct{}{}
	}

	for file := range allFiles {
		_, inA := setA[file]
		_, inB := setB[file]
		switch {
		case inA && inB:
			dataA, errA := loadA(file)
			dataB, errB := loadB(file)
			if errA != nil || errB != nil {
				continue // skip files that cannot be read on either side
			}
			yamlA, e := differ.LoadYAMLMap(dataA)
			if e != nil {
				continue
			}
			yamlB, e := differ.LoadYAMLMap(dataB)
			if e != nil {
				continue
			}
			diffs := differ.Diff(yamlA, yamlB)
			if len(diffs) > 0 {
				changed = append(changed, perFileDiff{File: file, Diffs: diffs})
			}
		case inA && !inB:
			removed = append(removed, file)
		case !inA && inB:
			added = append(added, file)
		}
	}

	sort.Slice(changed, func(i, j int) bool { return changed[i].File < changed[j].File })
	sort.Strings(added)
	sort.Strings(removed)
	return changed, added, removed, nil
}

// printDirSummary renders a per-file directory diff summary.
func printDirSummary(changed []perFileDiff, added, removed []string, outputFormat string) error {
	if len(changed) == 0 && len(added) == 0 && len(removed) == 0 {
		fmt.Println("No differences found.")
		return nil
	}

	if len(changed) > 0 {
		fmt.Println("Changed files:")
		for _, c := range changed {
			fmt.Printf("\n# %s\n", c.File)
			if err := printDiffs(c.Diffs, outputFormat); err != nil {
				return err
			}
		}
	}
	if len(added) > 0 {
		fmt.Println("\nAdded files:")
		for _, f := range added {
			fmt.Printf("  %s\n", f)
		}
	}
	if len(removed) > 0 {
		fmt.Println("\nRemoved files:")
		for _, f := range removed {
			fmt.Printf("  %s\n", f)
		}
	}
	return nil
}

// listDirYAMLFiles walks dir recursively and returns YAML file paths relative to dir.
func listDirYAMLFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !isYAMLFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// List all *.yaml and *.yml files in a given path at a specific git ref
func listGitFiles(ref, relPath string) ([]string, error) {
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", ref, relPath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, f := range lines {
		if isYAMLFile(f) {
			files = append(files, f)
		}
	}
	return files, nil
}

// Get the contents of a file at a specific git ref
func gitShowFile(ref, file string) ([]byte, error) {
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", ref, file))
	return cmd.Output()
}

// formatValuePlain returns string values as-is (no quotes).
func formatValuePlain(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatShellValue(val interface{}) string {
	if val == nil {
		return "NaN"
	}
	return fmt.Sprintf("%v", val)
}

// printDiffs outputs diff results in columns, shell, or yaml formats.
func printDiffs(diffs []differ.VariableDiff, outputFormat string) error {
	switch outputFormat {
	case "columns":
		type row struct {
			Key   string
			Old   string
			Arrow string
			New   string
		}
		rows := make([]row, 0, len(diffs))
		for _, d := range diffs {
			rows = append(rows, row{
				Key:   d.Name + ":",
				Old:   formatValuePlain(d.Default),
				Arrow: "->",
				New:   formatValuePlain(d.Value),
			})
		}
		// Find max width for each column
		maxKey, maxOld, maxArrow := 0, 0, 2
		for _, r := range rows {
			if l := len(r.Key); l > maxKey {
				maxKey = l
			}
			if l := len(r.Old); l > maxOld {
				maxOld = l
			}
		}
		// Print aligned
		for _, r := range rows {
			fmt.Printf("%-*s  %-*s  %-*s  %s\n",
				maxKey, r.Key, maxOld, r.Old, maxArrow, r.Arrow, r.New)
		}
		return nil

	case "yaml":
		out := map[string]interface{}{
			"variables": diffs,
		}
		yamlBytes, err := yaml.Marshal(out)
		if err != nil {
			return fmt.Errorf("Error marshaling YAML: %w", err)
		}
		fmt.Print(string(yamlBytes))
		return nil

	case "shell", "":
		for _, d := range diffs {
			left := formatShellValue(d.Default)
			right := formatShellValue(d.Value)
			var color string
			switch d.Status {
			case "added":
				color = ColorGreen
			case "removed":
				color = ColorRed
			case "changed":
				color = ColorYellow
			default:
				color = ""
			}
			// Bold the var name, color only the new value
			fmt.Printf("%s%s%s: %s → %s%s%s\n",
				ColorBold, d.Name, ColorReset,
				left,
				color, right, ColorReset,
			)
		}
		return nil
	default:
		return fmt.Errorf("Unknown output format: %s. Supported: 'shell', 'yaml', 'columns'", outputFormat)
	}
}
