// Package snapshot reads and writes evals: a single markdown file with YAML
// frontmatter (the machine-readable metadata) and a body holding the user
// prompts. One file == one eval. A bench run is a set of these.
package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/abdul/bench/internal/config"
	"gopkg.in/yaml.v3"
)

// Gates are the deterministic shell commands run against an agent's resulting
// worktree. Any may be empty (skipped). They are auto-detected by the skill and
// hand-editable afterwards.
type Gates struct {
	Build string `yaml:"build,omitempty"`
	Test  string `yaml:"test,omitempty"`
	Lint  string `yaml:"lint,omitempty"`
}

// meta is the YAML frontmatter block.
type meta struct {
	Title    string            `yaml:"title"`
	Repo     string            `yaml:"repo,omitempty"`   // optional: empty => scratch eval
	Commit   string            `yaml:"commit,omitempty"` // optional: required only if Repo is set
	Created  string            `yaml:"created"`
	Feedback string            `yaml:"feedback,omitempty"`
	Replay   config.ReplayMode `yaml:"replay,omitempty"`
	Gates    Gates             `yaml:"gates,omitempty"`
}

// Snapshot is a parsed eval. Path is set on load and not serialized.
type Snapshot struct {
	meta    `yaml:",inline"`
	Prompts []string `yaml:"-"`
	Path    string   `yaml:"-"`
}

// promptDelim separates prompts in the body. Rendered invisibly in markdown.
const promptDelim = "<!-- prompt -->"

var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n?(.*)$`)

// Parse decodes a snapshot from raw markdown bytes.
func Parse(raw []byte) (*Snapshot, error) {
	m := frontmatterRe.FindSubmatch(raw)
	if m == nil {
		return nil, fmt.Errorf("missing YAML frontmatter")
	}
	var s Snapshot
	if err := yaml.Unmarshal(m[1], &s.meta); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}
	s.Prompts = parsePrompts(string(m[2]))
	if s.Replay == "" {
		s.Replay = config.ReplayOneShot
	}
	return &s, nil
}

// parsePrompts pulls each delimited block out of the body, trimming the
// "## Prompts" heading and surrounding whitespace.
func parsePrompts(body string) []string {
	idx := strings.Index(body, promptDelim)
	if idx < 0 {
		return nil
	}
	parts := strings.Split(body[idx:], promptDelim)
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Load reads and parses a snapshot file.
func Load(path string) (*Snapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	s.Path = path
	return s, nil
}

// LoadAll reads every *.md snapshot in the snapshots dir, sorted by title.
func LoadAll() ([]*Snapshot, error) {
	dir := config.SnapshotsDir()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Snapshot
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		s, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out, nil
}

// Render serializes a snapshot back to markdown bytes.
func (s *Snapshot) Render() ([]byte, error) {
	fm, err := yaml.Marshal(s.meta)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(fm)
	b.WriteString("---\n\n## Prompts\n\n")
	for _, p := range s.Prompts {
		b.WriteString(promptDelim)
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(p))
		b.WriteString("\n\n")
	}
	return []byte(b.String()), nil
}

// Save writes the snapshot to the snapshots dir under <title>.md.
func (s *Snapshot) Save() error {
	if err := config.EnsureDirs(); err != nil {
		return err
	}
	if s.Path == "" {
		s.Path = filepath.Join(config.SnapshotsDir(), Slug(s.Title)+".md")
	}
	b, err := s.Render()
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, b, 0o644)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug turns a title into a filesystem-safe kebab-case identifier.
func Slug(title string) string {
	s := slugRe.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(s, "-")
}

// IsScratch reports whether this eval has no repo anchor. Scratch evals run in
// a fresh empty workspace — for sessions captured outside a git repo (Claude
// Desktop, ChatGPT desktop, Cowork, etc.) or for from-scratch tasks.
func (s *Snapshot) IsScratch() bool { return strings.TrimSpace(s.Repo) == "" }
