# Publishing the google-tag-manager-cli plugin

## Before every release

1. `go generate ./...` — regenerate `internal/llmdocs/90-commands.md`, commit
   any diff.
2. `go test ./...` — includes `TestPluginSkills`, which enforces that
   `plugin.json.version` equals `PluginVersion` in `cmd/root.go` and that the
   SKILL.md frontmatter stays within the Agent Skills standard.
3. `claude plugin validate plugins/google-tag-manager-cli`.
4. Bump `PluginVersion` and `plugin.json.version` together, in the same commit
   as the release tag. The release workflow refuses a mismatched tag.

## Registering in the marketplace (first release only)

Add to `.claude-plugin/marketplace.json` in `ideamans/claude-public-plugins`:

```json
{
  "name": "google-tag-manager-cli",
  "source": {
    "source": "git-subdir",
    "url": "https://github.com/ideamans/google-tag-manager-cli.git",
    "path": "plugins/google-tag-manager-cli"
  }
}
```

## Verifying the published result

```
/plugin marketplace add ideamans/claude-public-plugins
/plugin install google-tag-manager-cli@ideamans-plugins
/gtmc-usage
```

Other hosts install the same files directly:

```bash
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-usage --agent copilot
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-log-audit --agent copilot
```
