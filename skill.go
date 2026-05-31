// Package prx is the module root. It embeds the bundled agent skill so the
// binary can materialise it for manual installation or debugging; the canonical
// copy under skills/prx/ is what skills.sh and apm install.
package prx

import _ "embed"

// SkillMD is the embedded agentskills.io SKILL.md.
//
//go:embed skills/prx/SKILL.md
var SkillMD []byte
