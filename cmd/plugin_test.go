package cmd

import (
	"testing"

	"github.com/ideamans/go-llm-cli-kit/skillcheck"
)

// TestPluginSkills validates the distributed Claude Code plugin: the manifest
// version must track PluginVersion, the SKILL.md frontmatter must stay within
// the Agent Skills standard (the same files are installed into Copilot, Cursor
// and Gemini CLI via `gh skill`), and the descriptions must still carry the
// terms a user says when they need this tool.
func TestPluginSkills(t *testing.T) {
	report := skillcheck.CheckDir("../plugins/google-tag-manager-cli", skillcheck.Options{
		Version:             PluginVersion,
		Keywords:            []string{"tag manager", "gtm", "install"},
		RequireInstallSkill: true,
	})
	for _, problem := range report.Problems {
		t.Error(problem)
	}
}
