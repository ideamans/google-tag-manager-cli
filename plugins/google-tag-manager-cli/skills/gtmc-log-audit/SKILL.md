---
name: gtmc-log-audit
description: Audit what gtmc actually did to Google Tag Manager by reading its local audit log — prove that a session or an agent only touched the GTM accounts and containers it was allowed to, list every change (create, update, delete, publish) with the command that made it, find failed or retried API requests, and reconstruct one CLI run step by step. Use when the user asks what was changed in Tag Manager, whether anything outside the permitted account or container was touched, who published a version, why a GTM operation failed, or wants an audit trail or compliance report of GTM work.
license: MIT
compatibility: Requires the `gtmc` binary on PATH — run the gtmc-install skill if it is missing. Reads only local files; needs no credentials and makes no API calls. The log covers only commands run on this machine by this OS user.
allowed-tools: Bash(gtmc:*) Bash(jq:*) Bash(command:*) Read
---

# gtmc-log-audit

Answer "what did gtmc do, and where?" from the local audit log. This is
read-only: it never calls the GTM API, so it is safe to run at any time.

## 1. Find the log

```bash
command -v gtmc && gtmc --version
gtmc log dir      # bare path, e.g. ~/.local/state/google-tag-manager/logs
gtmc log path     # the same directory, its files and the retention, as JSON
```

One JSON Lines file per day (`audit-YYYY-MM-DD.jsonl`). Files older than the
retention (`$GTM_LOG_RETENTION_DAYS`, default 90 days) are already gone —
say so if the user asks about an older period.

What a record means:

| `event` | Fields | Meaning |
| --- | --- | --- |
| `command`, `phase: start` | `args`, `target` | one CLI run began; `args` is the command line (secrets masked) |
| `api` | `method`, `path`, `account`, `container`, `workspace`, `write`, `status`, `request`, `error` | one HTTP attempt; retries are separate lines |
| `command`, `phase: end` | `ok`, `error`, `durationMs` | the run finished |

All records carry `time` and `invocation`; the invocation ID groups one run.
`write` is true for every method but GET. Long strings in `request` are cut
(`…(N chars)`), so the log shows *what kind* of change was made, not always
the full new body — use `version get` / `export` for that.

## 2. Prove the scope (the most common request)

Ask the user which accounts or containers were allowed, and since when. Then:

```bash
gtmc log summary --since 2026-11-03 --allow-account 1234567890
gtmc log summary --since 8h --allow-container 98765432,87654321
```

- Exit code 0 and `"violations": []` = every API request stayed inside the list.
- Exit code 1 = the `violations` array lists each request outside it
  (`outside: "account"` or `"container"`). **Report these verbatim** — do not
  soften or explain them away.
- `--allow-container` takes numeric container IDs, not `GTM-XXXX`. Get them from
  the `container` field of `log show` lines or from `container info GTM-XXXX`
  (which calls the API — only if the user agrees).
- `unscoped` counts requests with no account in the path (`accounts.list`, or a
  `containers.lookup` that failed). They read the list of accounts the user can
  see; mention them when the user's restriction was strict.
- Requests inside an allowed account but outside any container (`containers.list`)
  are not container violations; they appear under the account's totals.

## 3. List the changes

```bash
gtmc log show --since 24h --writes | jq -c '{time, method, path, status, invocation}'
```

To name the command behind a change, look up its invocation:

```bash
gtmc log show --since 24h --invocation <id> | jq -c 'select(.event=="command") | {phase, args, ok}'
```

Classify for the user (path suffix → operation):

| Method and path | Operation |
| --- | --- |
| `POST …/workspaces/N/tags` (also triggers, variables, folders, templates, clients, …) | created an entity |
| `PUT …/workspaces/N/tags/N` | updated an entity |
| `DELETE …/workspaces/N/tags/N` | deleted an entity |
| `POST …/tags/N:revert` | reverted an entity to the base version |
| `POST …/workspaces/N/bulk_update` | bulk change (may include deletions — read `request`) |
| `POST …/workspaces/N:create_version` | created a version (the workspace is consumed) |
| `POST …/versions/N:publish` | **published** — changed the live site |
| `DELETE …/versions/N`, `POST …/versions/N:undelete`, `:set_latest` | version deleted / restored / marked latest |
| `POST …/workspaces`, `PUT …/workspaces/N`, `DELETE …/workspaces/N`, `:sync`, `:resolve_conflict` | workspace operations |
| `POST …/environments`, `PUT`/`DELETE …/environments/N`, `:reauthorize` | environment changes (`:reauthorize` invalidates existing preview links) |

`write: true` means "not a GET", not always "changed something": these POSTs
only read — `:quick_preview` (compile check) and `folders/N:entities` (list a
folder). Leave them out of a list of changes.

Publishes deserve their own line in any report: time, container, version, and
the command line that did it.

## 4. Failures and retries

```bash
gtmc log show --since 24h --failed | jq -c '{time, method, path, status, error}'
```

- Several identical lines a few seconds apart with 429 or 5xx = the CLI's
  automatic retries (429 always; 5xx only for GET/PUT/DELETE).
- A POST with 5xx is never retried — whether it took effect is unknown from the
  log. Say so, and suggest checking with `version list` / `<kind> list` (an API
  call, so only with the user's agreement).
- A `command` end record with `ok: false` and no API lines failed before calling
  the API (bad flags, missing credentials).

## 5. Report

Keep it short and factual:

1. Period and log files read (`log path`).
2. Totals from `log summary`: runs, API requests, reads and writes per account
   and container, failed requests.
3. Scope verdict: the exact `--allow-…` used and the violation count.
4. Changes in time order, publishes first-class.
5. Failures that matter (an unconfirmed POST, a 403 on a container the user
   expected to reach).

## Limits

- Only this machine and this OS user (or whatever `$GTM_LOG_DIR` points
  to). Changes made in the GTM web UI or by other tools are not here — the
  container's own history (`version list`) is the authority for those.
- The log is a local file the user can edit or delete; it is evidence of what
  this CLI did, not a tamper-proof record.
- Do not delete or rewrite log files. Retention is handled by the CLI.
