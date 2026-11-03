---
name: gtmc-usage
description: Work with Google Tag Manager from the command line with gtmc — look up a container by its GTM-XXXX id, audit it for paused, orphaned, duplicated or Universal Analytics tags and unused triggers or variables, export it in the GTM import format, edit tags, triggers, variables, templates and server-side clients in a workspace (also in bulk, and by copying from another container), import Community Template Gallery templates, check that a workspace compiles, and create, stage and publish versions. Use when the user asks about a site's GTM container, tag cleanup, what a GTM tag does, or wants a change made in Tag Manager.
license: MIT
compatibility: Requires the `gtmc` binary on PATH — run the gtmc-install skill if it is missing. Needs OAuth credentials (`gtmc auth login`, which the user runs) for a Cloud project with the Tag Manager API enabled, and access to the container in GTM. Publishing changes live sites and always needs the user's explicit approval.
allowed-tools: Bash(gtmc:*) Bash(jq:*) Bash(command:*) Read Write
---

# gtmc-usage

Read, audit and change Google Tag Manager containers.

## 1. Confirm the tool and the credentials

```bash
command -v gtmc && gtmc --version
gtmc auth status
gtmc auth list-accounts
```

Missing binary? Run the `gtmc-install` skill.

If `list-accounts` fails, ask the user to run `gtmc auth login` — it opens a
browser, so **do not run it unattended**. For a container you must not change
(a client's, under review only), suggest `gtmc auth login --readonly`.

**Read the full reference once:** `gtmc llm`. It explains the
workspace/version model, the JSON shapes and every flag of the installed version.

## 2. Find the container

Use the `GTM-XXXX` id from the site's HTML — no account id is needed:

```bash
gtmc container info GTM-XXXX
gtmc container list --all --format table   # everything this account can see
```

Pass `--container GTM-XXXX` on each command, or save it with
`gtmc container use GTM-XXXX`.

## 3. Read and audit (no changes)

```bash
gtmc audit --container GTM-XXXX --format table
gtmc export --container GTM-XXXX --from live -o live.json
gtmc tag list --container GTM-XXXX --format table
```

`audit` reports facts (paused tags, tags with no firing trigger, unused
triggers/variables, Universal Analytics, duplicated custom HTML,
`document.write`). They are **candidates**, not verdicts: a variable flagged
unused may be read by a custom template's code. Present them as such.

An export file someone sent can be audited offline:
`gtmc audit --file export.json`.

## 4. Change things — the confirmation procedure

Edits never reach the site until a version is created **and** published. Keep it
that way until the user has seen the change.

1. **Snapshot first:** `gtmc export --from live -o before.json`.
2. **Use a dedicated workspace:**
   `gtmc workspace create --name "<purpose>" --use`.
   Do not edit someone's Default Workspace; if the CLI says the container has
   several workspaces, ask the user which one.
3. **Edit:** `tag pause`, `tag update`, `trigger create --file`, ...
   Prefer `tag pause` to `tag delete` when the effect is uncertain.
   For JSON bodies, copy the shape from `tag get <id>` of the same type rather
   than writing `parameter` arrays from memory.
   To copy several related entities (e.g. from another container's export),
   use `workspace bulk-update` with `new_1`, `new_2`, ... IDs so their trigger
   and folder references stay connected (`gtmc llm` shows the shape).
4. **Show the diff:** `gtmc workspace status --format table` and tell the
   user exactly what changed.
5. **Compile check:** `gtmc workspace quick-preview`. Exit 1 means it
   does not compile; the API rarely says why, so read the CLI's `diagnostics`
   (usually a `{{variable}}` that does not exist) and fix before going on.
6. **Version:** `gtmc version create --name "..." --notes "why"`.
   This consumes the workspace; nothing is live yet. If the user has a staging
   environment, offer `environment update <env-id> --version <id>` to try it
   there first — it does not touch the live site.
7. **Publish only on explicit approval**, stating the current live version id
   for rollback: `gtmc version publish <id> --yes`.

### Operations that need the user's explicit go-ahead

These stop with "Re-run with --yes" until confirmed. **Never add `--yes`
without the user's explicit go-ahead for that specific action.** Approval to
edit is not approval to publish. Before asking, say what will happen:

| Command | Tell the user |
| --- | --- |
| `<kind> delete <id>... --yes` | which entities (name and ID) leave the workspace; `revert` can undo until a version is created (`gtag-config` has no revert) |
| `workspace bulk-update --yes` | the bulk update includes deletions — list them |
| `workspace delete <id> --yes` | every unsaved change in that workspace is lost |
| `version publish <id> --yes` | what changes on the live site, and the current live version id for rollback |
| `version delete <id> --yes` | that version disappears from the rollback history (undelete restores it); the CLI refuses the live one |
| `environment delete <id> --yes` | pages loading that environment stop getting tags |
| `environment reauthorize <id> --yes` | every snippet and preview link with the old `gtm_auth` stops working |

`template import ... --acknowledge-permissions` is the same kind of decision:
it grants the gallery template its declared permissions (scripts it injects,
cookies, globals). Tell the user what the template's gallery page lists before
importing it.

Rollback: `gtmc version list --format table`, then publish the previous
version id (again with the user's approval).

## 5. Report

State the container (`GTM-XXXX`), which source you read (`live`, `latest`,
`workspace`), and for changes: the workspace, the version id created, and
whether it was published.

Every API request is recorded in a local audit log. If the user limited you to
certain accounts, prove it at the end and include the result:

```bash
gtmc log summary --since <when you started, e.g. 2h> --allow-account <id>,...
```

It exits 1 and lists the requests if anything touched another account. While
under such a limit, do not run `container list --all` (it reads every visible
account); use `container info GTM-XXXX` or `container list --account <id>`.
For a fuller look at the log — every change with the command that made it,
failed requests, `--allow-container` — use the `gtmc-log-audit` skill.

## Failure modes

| Symptom | Cause | Fix |
| --- | --- | --- |
| `command not found` | not installed | run the `gtmc-install` skill |
| `refresh token` missing | not logged in | ask the user to run `gtmc auth login` |
| 403 mentioning scope | logged in with `--readonly` | user re-runs `auth login` without it |
| 403 otherwise | no GTM permission, or Tag Manager API disabled in the Cloud project | ask the user |
| 404 on `versions.live` | never published | use `--from workspace` or `--from latest` |
| "the container has N workspaces" | ambiguous target | ask which one; pass `--workspace` |
| 400 "fingerprint is not valid" (or 409) | changed since read | `get` again and redo the edit |
| 400 "Unknown entity type (template public ID: cvt_...)" | wrong template type | use the TYPE from `template list --format table` (`cvt_<galleryId>` for gallery templates) |
| `create` succeeds but the tag never fires | trigger IDs copied from another container | use this container's trigger IDs, or `workspace bulk-update` with `new_N` IDs |
| `quick-preview` exits 1 | does not compile; `diagnostics` names the entity (missing `{{variable}}`) | fix them before `version create` |
| default workspace cleared after `version create` | the consumed workspace was not replaced (others exist) | `workspace create --use` before more edits |
| `version create` fails with merge conflicts | `workspace sync` found the same entity changed elsewhere | show both sides, then `workspace resolve-conflict --kind ... --id ... --take workspace\|base` as the user decides |
| user asks to manage users, create or delete containers | out of scope for this CLI | point them to the GTM UI |
