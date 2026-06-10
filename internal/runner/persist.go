package runner

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/abdul/bench/internal/snapshot"
)

// persist writes the aggregate run result as run.json. Per-job diff/output
// artifacts are written separately by saveArtifacts as each job completes.
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

// saveArtifacts persists a job's diff and the agent's written answer under
// runDir/jobs/<eval>__<model>/, so results stay inspectable after the working
// trees are cleaned up. Best-effort: scoring proceeds regardless.
func saveArtifacts(runDir, eval, model, diff, output string) {
	dir := filepath.Join(runDir, "jobs", snapshot.Slug(eval)+"__"+snapshot.Slug(model))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	if diff != "" {
		_ = os.WriteFile(filepath.Join(dir, "diff.patch"), []byte(diff), 0o644)
	}
	if output != "" {
		_ = os.WriteFile(filepath.Join(dir, "output.txt"), []byte(output), 0o644)
	}
}
