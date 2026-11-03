# gtmc — Google Tag Manager CLI

[![test](https://github.com/ideamans/google-tag-manager-cli/actions/workflows/test.yml/badge.svg)](https://github.com/ideamans/google-tag-manager-cli/actions/workflows/test.yml)
[![release](https://img.shields.io/github/v/release/ideamans/google-tag-manager-cli)](https://github.com/ideamans/google-tag-manager-cli/releases)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

English | [日本語](README.ja.md)

A command line client for the [Google Tag Manager API v2](https://developers.google.com/tag-platform/tag-manager/api/v2),
built for both people and AI agents. It reads, audits and exports containers; edits tags, triggers,
variables and the rest in a workspace (including bulk edits and Community Template Gallery
imports); creates, stages and publishes versions; and keeps a local audit log of every API request.

- Every non-admin method of the Tag Manager API v2 is covered (96 of 110; the rest are user
  permissions, account settings, container creation/deletion and similar admin operations).
- Hard-to-undo operations require `--yes`. Non-idempotent requests are never retried
  automatically, so a flaky network cannot create or publish twice.
- `gtmc llm` prints a complete reference for AI agents; Claude Code skills are included.
- Built on Google's official Go client, [`google.golang.org/api/tagmanager/v2`](https://pkg.go.dev/google.golang.org/api/tagmanager/v2).

```sh
gtmc auth login
gtmc container use GTM-XXXX
gtmc audit --format table                      # what is in the live container, and what looks wrong
gtmc workspace create --name "Remove UA" --use
gtmc tag pause 42
gtmc workspace quick-preview                   # compile check
gtmc version create --name "Remove UA"
gtmc version publish 15 --yes                  # only this step changes the site
```

> **Status:** v0.x. Web containers are tested against the live API. Creating clients and
> transformations in server containers, and importing `gtmc export` output into the GTM UI, have
> not been verified yet — reports are welcome.

## Install

```sh
go install github.com/ideamans/google-tag-manager-cli/cmd/gtmc@latest
```

This needs Go 1.26 or later. Alternatively, download `google-tag-manager_<version>_<os>_<arch>.tar.gz` from
[Releases](https://github.com/ideamans/google-tag-manager-cli/releases) and put the `gtmc` binary on
your PATH.

## Setup

### 1. Create an OAuth client

`gtmc` ships no OAuth client of its own; you use one from your own Google Cloud project, so quota
and the consent screen stay under your control.

1. In the Google Cloud Console, enable the **Tag Manager API** for your project.
2. Create an OAuth client of type **Desktop app** and download its JSON.
3. Save it, unchanged (`{"installed": {...}}`), as:

```
~/.config/google-tag-manager/oauth2-client.json
```

`--client-id` / `--client-secret` or `GTM_CLIENT_ID` / `GTM_CLIENT_SECRET` work too.

### 2. Log in

```sh
gtmc auth login              # read, edit workspaces, create versions, publish, delete workspaces
gtmc auth login --readonly   # read only — for containers you must not change
```

The refresh token is stored in `~/.config/google-tag-manager/credentials.json` (mode 0600).

### 3. Check and pick a default container

```sh
gtmc auth status
gtmc auth list-accounts
gtmc container use GTM-XXXX   # --container can be omitted from now on
```

## Choosing the target

GTM is a hierarchy: **account → container → workspace → tags / triggers / variables**.

| Value | Flag | Environment | File |
|---|---|---|---|
| Account | `--account` | `GTM_ACCOUNT_ID` | `credentials.json` |
| Container | `--container` | `GTM_CONTAINER_ID` | `credentials.json` (`container use`) |
| Workspace | `--workspace` | `GTM_WORKSPACE_ID` | `credentials.json` (`workspace use`) |

- `--container` accepts a **`GTM-XXXX`** public ID (no account ID needed),
  `accounts/<a>/containers/<c>`, or a numeric ID together with `--account`.
- Without `--workspace`, the container's workspace is used **only if there is exactly one**.
  With several, `gtmc` refuses to guess.
- A saved workspace applies only to the saved container.
- `GTM_HOME` overrides the configuration directory.

## Read and audit

```sh
gtmc container list --all --format table      # every container you can see
gtmc container info GTM-XXXX
gtmc container snippet                        # the install snippet for the site
gtmc tag list --format table                  # likewise trigger / variable / folder / template ...
gtmc tag get 42
gtmc version list --format table
gtmc version get live                         # the whole published version

gtmc export --from live -o live.json          # GTM UI import format
gtmc audit --format table                     # inventory and findings (changes nothing)
gtmc audit --file live.json                   # audit an export offline
```

`--from` is `live` (published), `latest` (newest version), `workspace` (unsaved edits) or a
version number.

### Entities

| Command | Entity | Notes |
|---|---|---|
| `tag` / `trigger` / `variable` | tags, triggers, user-defined variables | `tag pause` / `unpause` |
| `folder` | folders | `move` (put entities in), `entities` (list contents) |
| `template` | custom templates | `import` from the Community Template Gallery |
| `client` / `transformation` | clients, transformations | server containers only |
| `zone` | zones | web containers, Tag Manager 360 only |
| `gtag-config` | Google tag configs | web containers; the API has no revert |
| `builtin-variable` | built-in variables | `enable` / `disable` / `revert` |

Each has `list` / `get` / `create --file` / `update` / `delete` / `revert`.

`audit` reports paused tags, tags without firing triggers, triggers and variables nothing
references, references to variables that do not exist (the workspace will not compile), Universal
Analytics tags, identical Custom HTML tags, Custom HTML that uses `document.write`, and duplicate
tag names. Custom HTML tags are listed largest first, as a rough guide to their page-speed cost.

## Change things

**Edits happen in a workspace and do not reach the site until a version is created and published.**

```sh
gtmc workspace create --name "Remove UA" --use

gtmc tag pause 42                              # unpause reverses it
gtmc tag update 42 --name "GA4 - page_view" --folder 7
gtmc tag get 42 > t.json && vi t.json && gtmc tag update 42 --file t.json
gtmc trigger create --file trigger.json        # - reads stdin
gtmc variable delete 13 --yes
gtmc tag revert 42                             # undo the workspace change
gtmc folder move 7 --tags 42,43
gtmc builtin-variable enable clickText clickUrl
gtmc workspace bulk-update --file changes.json # many changes in one call; new_1 etc. cross-reference
gtmc template import --owner <owner> --repository <repo> --acknowledge-permissions

gtmc workspace status --format table           # what changed
gtmc workspace quick-preview                   # compile without creating a version (exit 1 on errors)
gtmc version create --name "Remove UA" --notes "UA stopped collecting in 2024"
gtmc environment update 3 --version 15         # deploy to a staging environment (live is unchanged)
gtmc version publish 15 --yes                  # publish — the site changes
```

- `update --file` **replaces** the entity. Edit the JSON from `get` and pass it back.
- `create --file` ignores IDs, paths and fingerprints in the body, so entities can be copied from
  another container's export. **References (trigger IDs, folder IDs, `{{variable}}` names, `cvt_`
  template types) are not rewritten.** To copy entities together with their references, use
  `workspace bulk-update` with `new_N` placeholder IDs.
- When `workspace sync` reports conflicts: `workspace resolve-conflict --kind tag --id 12 --take workspace|base`.
- `version create` **consumes the workspace** (GTM's behaviour). GTM creates a new one only when it
  was the last workspace; the saved default follows it, or is cleared when none was created.
- Tags from gallery templates have the type `cvt_<galleryId>` (the TYPE column of
  `template list --format table`).
- When `workspace quick-preview` fails, the API rarely says why; `gtmc` adds `diagnostics` naming
  broken `{{variable}}` references.
- `--yes` is required for: every entity `delete`, `workspace delete`, `workspace bulk-update` with
  deletions, `version publish`, `version delete`, `environment delete`, `environment reauthorize`.
- Roll back by publishing an earlier version: `version list`, then `version publish <id> --yes`.
- Version housekeeping: `version update` (name, notes), `version delete --yes` / `undelete`,
  `version set-latest`. Deleting the live version is refused.

### Staging environments

```sh
gtmc environment create --name staging --url https://stg.example.com
gtmc environment list        # append previewQuery to the gtm.js URL to load that environment
gtmc environment update 3 --version 15
```

### Out of scope

User permissions, account settings, creating/deleting/combining containers, moving Google tag IDs
and destinations are left to the GTM web UI — they are admin operations and would need broader
scopes. Importing a container file, preview mode (Tag Assistant), approval workflows and
provisioning a server-side tagging runtime have no API.

## Audit log

Every command that calls the API records **what was done to which target** in a local log, so you
can show afterwards that no unintended account or container was touched. If the log cannot be
opened, `gtmc` refuses to call the API.

- Location: `~/.local/state/google-tag-manager/logs/audit-YYYY-MM-DD.jsonl` (honours
  `$XDG_STATE_HOME`; override with `$GTM_LOG_DIR`). One JSON Lines file per day.
- Rotation: files older than 90 days are deleted on the next run (`$GTM_LOG_RETENTION_DAYS`).
- Contents: command start (arguments, target) and end (success), and one line per API request with
  method, path, account/container/workspace, status and body. Strings over 256 characters and
  arrays over 50 items are abbreviated. The OAuth token and `--client-secret` are never written.

```sh
gtmc log dir                                          # the directory, as a bare path
open "$(gtmc log dir)"                                # open it in Finder (macOS)
gtmc log path                                         # directory, files and retention (JSON)
gtmc log show --since 2h --writes                     # requests that change something
gtmc log show --since 24h --failed                    # failed requests and commands
gtmc log show --invocation <id>                       # one run; its start line has the command line
gtmc log summary --since 2026-11-01 --allow-account 1234567890   # exit 1 if another account was touched
gtmc log summary --since 8h --allow-container 98765432          # the same per numeric container ID
```

`write` is true for every non-GET request; `:quick_preview` and `folders/N:entities` are POSTs that
only read. The log covers this machine and OS user only — changes made in the GTM UI are not in it.
To have an agent audit a session, use the `gtmc-log-audit` skill.

## For AI agents

```sh
gtmc llm               # the GTM model, safe change procedure, JSON shapes and every command
gtmc llm --format json
```

In Claude Code, install the plugin from the ideamans marketplace. It adds `/gtmc-usage` (how to
work with GTM safely, including confirmation steps), `/gtmc-log-audit` (audit what was done) and
`/gtmc-install`.

```
/plugin marketplace add ideamans/claude-public-plugins
/plugin install google-tag-manager-cli@ideamans-plugins
```

For Copilot, Cursor, Gemini CLI and others, install the same skills with `gh skill`:

```sh
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-usage --agent copilot
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-log-audit --agent copilot
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-install --agent copilot
```

## Development

```sh
go build -o gtmc ./cmd/gtmc
go generate ./...   # regenerate the embedded command catalogue
go test ./...
```

- **test** (`.github/workflows/test.yml`): `go mod tidy` diff → vet → build → catalogue diff → `go test -race`
- **release** (`.github/workflows/release.yml`): GoReleaser on a `v*` tag; checks the tag matches `PluginVersion`

Release steps (version bumps, marketplace registration) are in
[`plugins/google-tag-manager-cli/PUBLISH.md`](plugins/google-tag-manager-cli/PUBLISH.md).

## License

MIT — see [LICENSE](LICENSE). Google Tag Manager is a trademark of Google LLC. This project is not
affiliated with or endorsed by Google.
