---
name: gtmc-install
description: Make the gtmc command (google-tag-manager-cli) available, installing it only if it is missing. Use when another skill reports that `gtmc` is not on PATH, or when the user asks to install, update or upgrade the ideamans Google Tag Manager (GTM) CLI. Prefers an already-installed binary, then the latest GitHub release, then a build from source with go install.
license: MIT
compatibility: Requires curl and tar to install from a release, or a Go toolchain for the source fallback. Standalone — does not need gtmc to be present already. Installs from the public repository github.com/ideamans/google-tag-manager-cli, so no GitHub authentication is needed.
allowed-tools: Bash(curl:*) Bash(go:*) Bash(tar:*) Bash(unzip:*) Bash(uname:*) Bash(command:*) Bash(which:*) Bash(mkdir:*) Bash(mv:*) Bash(cp:*) Bash(rm:*) Bash(chmod:*) Bash(ls:*) Bash(test:*) Bash(echo:*) Read
---

# gtmc-install

Make the `gtmc` command usable, doing the least work that achieves it.

## Route 1 — an existing installation on PATH

```bash
command -v gtmc && gtmc --version
```

If that resolves, **use it and stop here.** Do not check for a newer release —
it costs an API call and the user did not ask for an upgrade.

Two checks before trusting the hit:

- **It is the right tool.** Short names collide; note that `gtm` (without the
  `c`) is a different Tag Manager CLI. `gtmc llm | head -1` must read
  `# gtmc (google-tag-manager-cli) — Guide for LLMs`. If something else owns the
  name, tell the user and install this one under an explicit path rather than
  shadowing theirs.
- **It is recent enough.** If `gtmc llm` is not a known command, it is either
  another tool or a build that predates the embedded reference. Say so and
  continue to route 2.

Continue past this section only when the command is missing, is the wrong tool,
is too old, or the user explicitly asked to update.

## Route 2 — the latest GitHub release

The repository is public, so no authentication is needed.

```bash
VERSION=$(curl -fsSL https://api.github.com/repos/ideamans/google-tag-manager-cli/releases/latest \
  | grep '"tag_name"' | head -1 | cut -d'"' -f4)   # e.g. v0.1.0
```

**The archive is named after the goreleaser project, not the repository** —
`google-tag-manager`, without the `-cli` suffix — and the binary inside is `gtmc`:

```
google-tag-manager_<version-without-v>_<os>_<arch>.tar.gz
```

`<os>` is `darwin`, `linux` or `windows` (lowercase); `<arch>` is `amd64` or
`arm64`, so `uname -m` reporting `x86_64` maps to `amd64`. Windows ships a
`.zip`.

```bash
OS=$(uname -s | tr '[:upper:]' '[:lower:]')            # darwin | linux
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64  # amd64 | arm64
curl -fsSL -o /tmp/gtmc.tar.gz \
  "https://github.com/ideamans/google-tag-manager-cli/releases/download/${VERSION}/google-tag-manager_${VERSION#v}_${OS}_${ARCH}.tar.gz"
```

If the download 404s, list the actual assets on the release page rather than
retrying variations.

### Install onto PATH

```bash
tar -xzf /tmp/gtmc.tar.gz -C /tmp gtmc
mkdir -p ~/.local/bin && mv /tmp/gtmc ~/.local/bin/ && chmod +x ~/.local/bin/gtmc
```

Prefer the first writable directory already on PATH — `~/.local/bin`, then
`/usr/local/bin`. Two things not to do on your own initiative:

- If nothing on PATH is writable, leave the binary in `/tmp`, print the exact
  `sudo mv` command and let the user run it. Do not run `sudo` yourself.
- If `~/.local/bin` is not on PATH, give the user the line to add to their shell
  profile. Do not edit the profile for them.

## Route 3 — build from source

Needs a Go toolchain and compiles rather than downloads, so it is the last
resort — but it covers platforms the release assets miss. Note the `/cmd/gtmc`
suffix; installing the module root would not build anything.

```bash
go install github.com/ideamans/google-tag-manager-cli/cmd/gtmc@latest
```

The binary lands in `$(go env GOPATH)/bin` and is named `gtmc`.

## Verify

```bash
gtmc --version
gtmc llm | head -5
gtmc auth status
```

Report which route was taken, the version and the install path.

Then say what is still needed. `gtmc` brings no OAuth client of its own — the
user creates one:

1. In a Google Cloud project, enable the **Tag Manager API**.
2. Create an OAuth client of type **Desktop app** and download its JSON.
3. Save it as `~/.config/google-tag-manager/oauth2-client.json` (or set
   `GTM_CLIENT_ID` / `GTM_CLIENT_SECRET`).
4. **The user runs `gtmc auth login` themselves** — it opens a browser and waits
   for their approval. Add `--readonly` for containers that must not be changed.

Never ask the user to paste a client secret or token into the conversation.
