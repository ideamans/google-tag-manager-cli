# gtmc (google-tag-manager-cli) — Guide for LLMs

`gtmc` is a non-interactive CLI for Google Tag Manager (Tag Manager API v2). Every command
reads flags, environment variables and its config files only; nothing prompts, so it is safe to
run from scripts and agents. The single exception is `auth login`, which opens a browser and waits
for the user's consent — the user runs it, not you.

**Output contract.** Results go to stdout as **JSON by default** — a faithful copy of the API
response. List commands, `audit`, `workspace status` and `environment list` accept
`--format table` for humans. Errors go to stderr prefixed with `Error:`; the exit code is 0 on
success and 1 on any failure (including a guard such as a missing `--yes`, and a workspace that
does not compile in `workspace quick-preview`). Progress notes (e.g. "Default workspace moved")
also go to stderr, so stdout stays parseable.

This reference is embedded in the binary. `gtmc llm` always describes the exact version you
are running — prefer it over anything you remember about this tool. The generated command catalog
(every command and flag) is appended at the end.

## Ground rules

1. **Nothing you edit is live until a version is created and published.** Edit in a dedicated
   workspace, check it, and publish only with the user's explicit approval (procedure below).
2. **Never add `--yes` on your own initiative.** It is required for every `delete`,
   `workspace delete`, `workspace bulk-update` with deletions, `version publish`,
   `version delete`, `environment delete` and `environment reauthorize`. Approval to edit is not
   approval to publish.
3. **Address the container by `GTM-XXXX`** (from the site's HTML); no account ID is needed. The CLI
   never guesses a workspace: if the container has several, it stops and asks for `--workspace`.
4. **Don't write JSON bodies from memory.** Fetch an existing entity of the same `type` with
   `get` and edit it. Unknown fields are rejected, so a typo fails loudly instead of being dropped.
5. **Bodies go through a file or stdin** (`--file body.json` or `--file -`), never through a
   shell-quoted flag.
6. **IDs are per container.** A trigger ID, folder ID or `cvt_` template type copied from another
   container points at nothing here — see "Copying between containers".
7. **Every API request is logged locally** (see "Audit log"). When the user has limited you to
   certain accounts, finish by proving it: `log summary --since <session start> --allow-account <ids>`.
   Avoid `container list --all` in that situation — it reads every account you can see.

## The GTM model (read this first)

```
account ─ container (GTM-XXXX) ─ workspace ─ tags / triggers / variables / folders / templates
        │                      │            (server: clients / transformations; web: zones,
        │                      │             Google tag configs; built-in variables)
        │                      ├ versions (immutable snapshots; one of them is "live")
        │                      └ environments (Live, Latest, and user ones such as staging)
```

- **Every edit happens in a workspace and is invisible to the site** until a version is created
  from that workspace (`version create`) **and** that version is published (`version publish`).
- `version create` **consumes the workspace**: GTM deletes it. Only when it was the container's
  last workspace does GTM create a fresh one (`newWorkspacePath`). If it was the saved default, the
  CLI moves the default to the new workspace, or clears it when none was created — then run
  `workspace create --use` before editing again.
- `live` = the published version (what the site runs). `latest` = the newest version, possibly
  unpublished. `workspace` = current unsaved edits.
- Rollback = publish an older version ID again (`version list`, then `version publish <id> --yes`).
- Container kinds: **web** containers have tags/triggers/variables, zones (Tag Manager 360 only)
  and Google tag configs; **server** containers additionally have clients and transformations.

## Choosing the target

- `--container` accepts **`GTM-XXXX`** (looked up via `containers.lookup`, no account ID needed),
  `accounts/<a>/containers/<c>`, or a numeric container ID together with `--account`.
- Resolution order for each of account / container / workspace:
  **flag > env (`GTM_ACCOUNT_ID`, `GTM_CONTAINER_ID`, `GTM_WORKSPACE_ID`) > `credentials.json`**.
- `container use GTM-XXXX` saves the default container; `workspace use <id>` saves the workspace.
  A saved workspace is only applied to the saved container — pointing `--container` elsewhere
  never carries the old workspace over.
- Without a workspace, the CLI uses the container's workspace **only if there is exactly one**.
  With several it stops and lists them; it never guesses (someone else may be working in one).

## Safe change procedure (follow it for any write)

1. `export --from live -o before.json` — a snapshot to compare against and rebuild from.
2. `workspace create --name "<purpose>" --use` — work in a dedicated workspace, not someone's
   Default Workspace.
3. Make the edits (`tag create/update/pause/delete`, `workspace bulk-update`, ...).
4. `workspace status --format table` — confirm exactly the intended entities changed.
5. `workspace quick-preview` — compile without creating a version; exit 1 lists `compilerErrors`.
6. Show the user the change list and get approval.
7. `version create --name "<what>" --notes "<why>"` — nothing is live yet.
8. Optionally deploy it to a staging environment first: `environment update <env-id> --version <id>`.
9. **Ask the user before** `version publish <id> --yes`. Publishing changes every site using the
   container within minutes. Report the previous live version ID so it can be rolled back.

Prefer `tag pause` over `tag delete` when the effect is uncertain — pausing keeps the tag.

## Editing entities

The same subcommands exist for `tag`, `trigger`, `variable`, `folder`, `template`, `client`,
`transformation`, `zone` and `gtag-config`: `list`, `get`, `create --file`, `update`, `delete`,
and `revert` (except `gtag-config`, which has no revert in the API).

- `get` returns the API JSON; `update --file` **replaces the whole entity** with the file. The
  usual loop is `tag get 12 > t.json` → edit → `tag update 12 --file t.json`. A fingerprint in
  the file enables optimistic locking: if the entity changed meanwhile (or you reuse an old file
  after another update), the API answers `400 The provided entity fingerprint is not valid.` —
  `get` again and redo the edit.
- What the API validates on write: unknown trigger IDs (`400 Tag references an unknown trigger`),
  required parameters of template-based tags (`vendorTemplate.parameter.<key>: The value must not
  be empty`). What it does **not** validate on write: `{{Variable}}` references — a missing one
  only shows up as a compile error (see `workspace quick-preview`).
- `update <id> --name/--notes/--folder` changes only those fields (the CLI fetches, patches, saves).
  Kinds without a name (`gtag-config`) or folder (`folder`, `template`, `zone`) lack those flags.
- `revert <id>` discards the workspace's changes to that entity (back to the latest version).
- `create --file` ignores IDs, paths and fingerprints in the body.
- Typical shapes (copy real ones from `get` rather than these):

```json
{"name": "HTML - Example", "type": "html",
 "parameter": [{"type": "template", "key": "html", "value": "<script>/* ... */</script>"},
               {"type": "boolean", "key": "supportDocumentWrite", "value": "false"}],
 "firingTriggerId": ["2147479553"]}
```

```json
{"name": "CE - purchase", "type": "customEvent",
 "customEventFilter": [{"type": "equals", "parameter": [
   {"type": "template", "key": "arg0", "value": "{{_event}}"},
   {"type": "template", "key": "arg1", "value": "purchase"}]}]}
```

- Trigger ID **`2147479553` is the built-in "All Pages"** trigger. IDs above ~2147479000 are
  built-in triggers; they do not appear in `trigger list`.
- Built-in variables are enabled by API type (`pageUrl`, `clickElement`, `clickText`, ...),
  not by UI name: `builtin-variable enable clickText clickUrl`; `builtin-variable revert <type>`
  undoes the workspace's change.
- `folder entities <id>` lists what is in a folder; `folder move <id> --tags 1,2` moves things in.

### Bulk changes and copying between containers

`workspace bulk-update --file changes.json` applies many changes in one call:

```json
{"changes": [
  {"folder":  {"folderId": "new_1", "name": "Imported"}, "changeStatus": "added"},
  {"trigger": {"triggerId": "new_2", "name": "CE - purchase", "type": "customEvent",
               "parentFolderId": "new_1", "customEventFilter": [...]}, "changeStatus": "added"},
  {"tag":     {"tagId": "new_3", "name": "GA4 - purchase", "type": "gaawe",
               "parentFolderId": "new_1", "firingTriggerId": ["new_2"], "parameter": [...]},
   "changeStatus": "added"},
  {"tag":     {"tagId": "42", "name": "Old UA", ...}, "changeStatus": "deleted"}
]}
```

- New entities need IDs `new_1`, `new_2`, ... and may refer to each other by them. **This is how to
  copy a group of entities from another container's export with references intact**: give each
  copied entity a `new_N` ID and rewrite `firingTriggerId`, `blockingTriggerId`, `parentFolderId`,
  variables' `enablingTriggerId`/`disablingTriggerId` and trigger-group `triggerReference` values to
  match. Existing entities keep their real IDs (`updated` / `deleted`).
- `{{Variable Name}}` references are by name: create or keep a variable with that name.
- `cvt_<containerId>_<templateId>` types must exist in the target: import the gallery template or
  create the custom template first, then use its new type.
- With `create --file` one by one instead, nothing is remapped: create folders, templates,
  variables and triggers first, then tags with the new IDs.
- A bulk update containing `"changeStatus": "deleted"` requires `--yes`.

### Templates from the Community Template Gallery

`template import --owner <github-owner> --repository <repo> --acknowledge-permissions [--sha <commit>]`
imports a gallery template (owner/repository are on its gallery page; the gallery is backed by
GitHub). The template's declared permissions (URLs it may inject, cookies, globals) are granted on
import, so tell the user what they are before importing. Tags and variables use a template through
their `type`: **`cvt_<galleryTemplateId>`** for a gallery template (e.g. `cvt_MBTSV`, from
`galleryReference.galleryTemplateId`) and `cvt_<containerId>_<templateId>` for one written in the
container. `template list --format table` shows the right value in the TYPE column. Required
template parameters must be given on create (copy the keys from the template's
`___TEMPLATE_PARAMETERS___` in `template get`).

## Workspaces: conflicts and preview

- `workspace sync` brings changes from a version created meanwhile into the workspace. If the same
  entity changed on both sides, `workspace status` lists a `mergeConflict` and `version create`
  will fail until it is resolved:
  `workspace resolve-conflict --kind tag --id 12 --take workspace|base`, or `--file merged.json`
  with your own merge in the Entity shape (`{"tag": {...}}`).
- `workspace quick-preview` compiles without creating a version or consuming the workspace. It is
  not GTM's Preview mode (Tag Assistant), which exists only in the UI. The API usually sets
  `compilerError: true` with an **empty** `compilerErrors`, so the CLI adds `diagnostics`: entities
  referring to `{{variables}}` that are neither user variables nor enabled built-ins (the common
  cause; `_event`-style internal names are allowed).

## Versions and environments

- `version list [--include-deleted]`, `version get live|latest|<id>`.
- `version update <id> --name/--notes` renames a version (contents are immutable).
- `version delete <id> --yes` hides a version from the history (and from rollback);
  `version undelete <id>` restores it. While deleted, `version get <id>` answers 404; only
  `version list --include-deleted` shows it. The CLI refuses to delete the live version.
- Rollback was verified end to end: `version publish latest --yes`, then
  `version publish <previous live id> --yes` puts the old version back immediately.
- `version set-latest <id>` changes the base workspaces sync with; it publishes nothing.
- `environment list` shows Live, Latest and user environments with the deployed version and a
  `previewQuery` (`gtm_auth=...&gtm_preview=env-N&gtm_cookies_win=x`) to add to the gtm.js URL on a
  staging site. `environment create --name staging --url https://stg.example.com` creates one;
  `environment update <id> --version <vid>` deploys a version to it without touching the live site.
  `environment reauthorize <id> --yes` issues a new code and breaks every link using the old one.
- `container snippet` prints the install code for the site.

## Common tag types

`html` Custom HTML · `img` Custom Image · `googtag` Google Tag · `gaawe` GA4 Event ·
`gaawc` GA4 Configuration (legacy) · `ua` Universal Analytics (dead) · `awct` Google Ads
conversion · `sp` Google Ads remarketing · `gclidw` Conversion Linker · `flc`/`fls` Floodlight ·
`cvt_<container>_<templateId>` custom template (gallery templates too).

## Export and audit

- `export --from live -o live.json` writes `{exportFormatVersion: 2, exportTime, containerVersion}`
  — the GTM UI's Export Container format; it can be imported in the UI (Admin > Import Container,
  merge or overwrite). There is **no import API**; to rebuild through the API use
  `workspace bulk-update` as described above.
- `--from` is `live`, `latest`, `workspace` (current edits, including clients/transformations on
  server containers and zones/Google tag configs on web containers) or a version number.
- `audit` (default `--from live`) never writes. `audit --file export.json` works offline, with no
  credentials, on an export from any source (including one a client sent).
- Audit finding codes (facts, not verdicts — confirm with the user before deleting):

| code | severity | meaning |
| --- | --- | --- |
| `PAUSED_TAG` | info | paused tag; never fires — deletion candidate |
| `TAG_WITHOUT_FIRING_TRIGGER` | warn | not paused, no firing trigger, not a setup/teardown tag — never fires |
| `UNUSED_TRIGGER` | info | referenced by no tag, variable or trigger group |
| `UNUSED_VARIABLE` | info | `{{name}}` appears in no tag, trigger, other variable, client, transformation, zone or Google tag config |
| `UNIVERSAL_ANALYTICS` | warn | `ua` tag; UA stopped processing data, so it only costs load time |
| `DUPLICATE_CUSTOM_HTML` | warn | identical custom HTML in several tags (double-firing risk) |
| `DOCUMENT_WRITE` | warn | custom HTML uses `document.write` (render-blocking) |
| `DUPLICATE_TAG_NAME` | info | several tags share a name |
| `BROKEN_VARIABLE_REFERENCE` | warn | refers to a `{{name}}` that is neither a variable nor an enabled built-in variable — the container will not compile |

- `UNUSED_VARIABLE` is text matching: a variable used only from a custom template's code or from
  outside GTM is still reported. Check before deleting.
- `customHtml` lists custom HTML tags largest first with `allPages` and `documentWrite` flags — the
  starting point for estimating a container's page-speed cost.

## Auth & configuration

```sh
gtmc auth login [--readonly]   # OAuth2 in the browser (the user must run it; it waits for consent)
gtmc auth status               # resolved configuration and default target
gtmc auth list-accounts        # connectivity check
```

Config: `~/.config/google-tag-manager/oauth2-client.json` (Desktop OAuth client of a Cloud project with the
**Tag Manager API enabled**) and `credentials.json` (refresh token, scopes, default target).
`GTM_HOME` overrides the directory. `--readonly` requests only `tagmanager.readonly`; the
default login adds edit, version, publish and delete scopes.

## Audit log

Every command that calls the API appends to a local audit log, so it can be shown afterwards which
accounts and containers were touched. It is not optional: if the log cannot be opened, the CLI
refuses to call the API.

- Location: `$GTM_LOG_DIR`, else `$XDG_STATE_HOME/google-tag-manager/logs`, else
  `~/.local/state/google-tag-manager/logs` (`log dir` prints the bare path; `log path` adds the files as JSON). One JSON Lines file per day,
  `audit-YYYY-MM-DD.jsonl`; files older than `$GTM_LOG_RETENTION_DAYS` days (default 90)
  are deleted when the CLI next opens the log.
- Records (all have `time` and `invocation`, which groups one CLI run):
  - `{"event":"command","phase":"start","args":[...],"target":{"account","container","workspace"}}`
  - `{"event":"api","method","path","query","account","container","workspace","write","request",
    "status","resultPath","error","durationMs"}` — one per HTTP attempt (retries appear separately).
    `account`/`container` come from the request path, or from the response's `path` for
    `containers.lookup`. `write` is true for anything but GET.
  - `{"event":"command","phase":"end","ok","error","durationMs"}`
- Strings longer than 256 characters and arrays longer than 50 items in request bodies are
  abbreviated (`…(N chars)`). The OAuth token and `--client-secret` are never written.
- Read it with `log show [--since 24h] [--account ID] [--container-id ID] [--writes] [--failed]
  [--invocation ID] [--limit N]` (JSON Lines). `--failed` keeps API requests with an error or
  status >= 400 and commands that ended `ok:false`; `--invocation` keeps one CLI run, whose start
  record holds the command line — the way to name the command behind a write.
- `log summary [--since ...] [--allow-account ID,...] [--allow-container ID,...]` counts reads and
  writes per account and container, `failedRequests` and `writesByMethod`, and **exits 1 listing
  violations** if any request touched an account (or numeric container) outside the lists.
  Requests with no account in the path (`accounts.list`) are counted as `unscoped`; requests in an
  allowed account but outside any container (`containers.list`) do not violate `--allow-container`.
- `write` is true for any non-GET. Two POSTs only read: `:quick_preview` and `folders/N:entities`.
- The log covers this machine and OS user only; GTM UI changes are not in it (`version list` is the
  container's own history). It is a local file, not a tamper-proof record.

## Failure modes

| Symptom | Cause | Fix |
| --- | --- | --- |
| `Error: credentials are missing` / no refresh token | not logged in | ask the user to run `gtmc auth login` |
| `[403]` with a scope hint | token issued with `--readonly` | the user re-runs `auth login` without it |
| `[403]` otherwise | no GTM permission on the container, or Tag Manager API disabled in the OAuth client's project | ask the user |
| `[403]` on `zone` | zones are a Tag Manager 360 feature | nothing to do on free accounts |
| `[404]` on `versions.live` | the container was never published | use `--from workspace` or `--from latest` |
| `[400] The provided entity fingerprint is not valid.` (or `[409]`) | changed since read | `get` again and redo the edit |
| `[400] ... references an unknown trigger` | trigger ID not in this container | use this container's IDs (`trigger list`) |
| `[400] vendorTemplate.key: Unknown entity type` | wrong `cvt_` type | use the TYPE shown by `template list --format table` |
| `[429]` | per-user rate limit (low) | the CLI already retries with backoff; avoid loops over many containers |
| `[5xx]` on a create, `version create`, `publish` or `bulk-update` | server error; it may or may not have been applied | **not retried automatically** (it could run twice). `list` / `workspace status` / `version list` to see whether it happened before retrying by hand |
| `[500]` on `gtag-config create` | seen with a minimal `{"type":"googtag",...}` body | copy a real Google tag config from `gtag-config get` of a container that has one |
| `the container has N workspaces` | ambiguous target | pass `--workspace` or ask the user which one |
| `Re-run with --yes to confirm` | guarded operation | get the user's explicit approval first |
| `version create` / `quick-preview` reports `compilerError` | the workspace does not compile | read `diagnostics` from `quick-preview`, fix, repeat until clean |
| `version create` fails on merge conflicts | sync left conflicts | `workspace resolve-conflict` each one |
| created tag never fires | trigger IDs copied from another container | use this container's IDs, or `bulk-update` with `new_N` |
| `invalid JSON body: unknown field` | typo or a field from another entity kind | compare with `get` output |
| `cannot open the audit log` / `refusing to call the API` | log directory not writable | ask the user; `GTM_LOG_DIR` points it elsewhere |
| `log summary` exits 1 with violations | a request touched an account outside `--allow-account` | report it to the user with the listed records |

## What this CLI will not do

- **Admin operations are out of scope**: user permissions (`user_permissions`), account settings,
  creating/renaming/deleting containers, combining containers, moving a Google tag ID out of a
  container, and linking Google tag destinations. Do them in the GTM UI. They need scopes
  (`manage.users`, `manage.accounts`) this CLI never requests.
- **No API exists** for: importing a container file (use the UI, or rebuild with `bulk-update`),
  GTM Preview mode / Tag Assistant debugging, approval workflows, and provisioning a server
  container's hosting (Cloud Run / App Engine).
- It does not decide what to delete: `audit` reports facts; the user decides.
