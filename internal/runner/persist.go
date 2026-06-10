package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// persist writes the run result as run.json plus a human-readable diff/rationale
// file per (eval x model) under the run directory.
func persist(runDir string, r *RunResult) error {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	return os.WriteFile(filepath.Join(runDir, "run.json"), b, 0o644)
}

// LoadRun reads a persisted run.json.
func LoadRun(path string) (*RunResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r RunResult
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
