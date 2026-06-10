// Package skill embeds the add-to-bench skill so `bench install` can write it
// into the user's global Claude Code skills directory.
package skill

import _ "embed"

//go:embed SKILL.md
var Markdown string

// Name is the skill directory/identifier.
const Name = "add-to-bench"
