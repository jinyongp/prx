// Package gate is the module root. It embeds the bundled agent skill so the
// binary can materialise it for manual installation or debugging; the canonical
// copy under skills/gate/ is what skills.sh and apm install.
package gate

import _ "embed"

// SkillMD is the embedded agentskills.io SKILL.md.
//
//go:embed skills/gate/SKILL.md
var SkillMD []byte
