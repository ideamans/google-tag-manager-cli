# Command catalog

Generated from the cobra command tree by `go generate ./...`.
Do not edit by hand — edit the command definitions instead.

## Global flags

| flag | type | default | description |
| --- | --- | --- | --- |
| `--account` | string | — | GTM account ID (default: $GTM_ACCOUNT_ID / credentials.json) |
| `--client-id` | string | — | OAuth client ID (default: $GTM_CLIENT_ID / oauth2-client.json) |
| `--client-secret` | string | — | OAuth client secret (default: $GTM_CLIENT_SECRET / oauth2-client.json) |
| `--container` | string | — | Container: GTM-XXXX public ID, numeric ID (needs --account), or accounts/<a>/containers/<c> (default: $GTM_CONTAINER_ID / credentials.json) |
| `--workspace` | string | — | Workspace ID (default: $GTM_WORKSPACE_ID / credentials.json / the only workspace) |

## `gtmc account`

List and inspect GTM accounts

Aliases: acc

### `gtmc account info`

Fetch account details (default: the account of the current container)

```
gtmc account info [account-id]
```

### `gtmc account list`

List accessible GTM accounts

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

## `gtmc audit`

Inventory a container: tag types, paused/orphaned tags, unused triggers/variables, custom HTML

Reports facts that can be checked mechanically, as cleanup candidates. It never changes anything.
Finding codes: PAUSED_TAG, TAG_WITHOUT_FIRING_TRIGGER, UNUSED_TRIGGER, UNUSED_VARIABLE,
BROKEN_VARIABLE_REFERENCE,
UNIVERSAL_ANALYTICS, DUPLICATE_CUSTOM_HTML, DOCUMENT_WRITE, DUPLICATE_TAG_NAME.
--file audits an export JSON offline (no API call, no credentials needed).

```
gtmc audit [--from live|latest|workspace|<version-id>] [--file export.json]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Audit an export JSON file instead of calling the API (- for stdin) |
| `--format` | string | `json` | Output format: json or table |
| `--from` | string | `live` | What to read: live (published), latest (newest version), workspace (current edits), or a version ID |

## `gtmc auth`

Manage authentication credentials

### `gtmc auth list-accounts`

List accessible GTM accounts (authentication connectivity check)

### `gtmc auth login`

Log in with OAuth2 and save the refresh token

Opens Google's consent screen in the browser and saves the resulting refresh token to credentials.json.
By default it requests read, workspace edit, version create, publish and workspace delete scopes.
With --readonly it requests only tagmanager.readonly, so every write command fails with 403.

| flag | type | default | description |
| --- | --- | --- | --- |
| `--no-browser` | bool | `false` | Do not open the browser automatically; only print the URL |
| `--readonly` | bool | `false` | Request only the read-only scope (for inspecting containers you must not change) |

### `gtmc auth status`

Show the resolved credentials and default account/container/workspace

## `gtmc builtin-variable`

List, enable, disable and revert built-in variables (Page URL, Click Element, ...)

Built-in variables are identified by their API type, not their UI name:
pageUrl, pagePath, pageHostname, referrer, event, clickElement, clickClasses, clickId,
clickTarget, clickUrl, clickText, formElement, formId, ..., scrollDepthThreshold, videoTitle, etc.

Aliases: builtin

### `gtmc builtin-variable disable`

Disable built-in variables by API type

```
gtmc builtin-variable disable <type>...
```

### `gtmc builtin-variable enable`

Enable built-in variables by API type

```
gtmc builtin-variable enable <type>...
```

### `gtmc builtin-variable list`

List built-in variables enabled in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc builtin-variable revert`

Discard the workspace's enable/disable of built-in variables (back to the latest version)

```
gtmc builtin-variable revert <type>...
```

## `gtmc client`

List, get, create, update, delete and revert clients in the workspace

Operates on clients in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.client in a GTM UI export (`gtmc export`).
Server containers only.

### `gtmc client create`

Create a client from API JSON

Creates a client in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc client create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc client delete`

Delete clients from the workspace (requires --yes)

Deletes clients from the workspace. Until a version is created this can be undone with `client revert <id>`; after a version is published it is gone
from the site.

```
gtmc client delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc client get`

Fetch one client as API JSON

```
gtmc client get <id>
```

### `gtmc client list`

List clients in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc client revert`

Discard the workspace's changes to clients (back to the latest version)

```
gtmc client revert <id>...
```

### `gtmc client update`

Update a client: replace it from JSON, or change only name/notes/folder

Update is a full replacement in the API. With --file, the JSON becomes the new client
(fetch it with `client get`, edit, and pass it back). Without --file, the current client is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc client update <id> [--file <json|->] [--name ...] [--notes ...] [--folder <id>]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--folder` | string | — | Move into this folder ID (empty string: out of any folder) |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc container`

List, look up, inspect and choose the default container; print its install snippet

Aliases: ct

### `gtmc container info`

Fetch container details; the argument may be GTM-XXXX, a path, or a numeric ID

Resolves the container (GTM-XXXX is looked up with containers.lookup, so no account ID is needed)
and prints it. Without an argument, uses --container or the saved default.

```
gtmc container info [container]
```

### `gtmc container list`

List containers in the account (--all: in every accessible account)

Lists containers under --account (or the account of the default container).
With --all, walks every accessible account — use this to find which account holds a GTM-XXXX.

| flag | type | default | description |
| --- | --- | --- | --- |
| `--all` | bool | `false` | List containers in every accessible account |
| `--format` | string | `json` | Output format: json or table |

### `gtmc container snippet`

Print the tagging snippet to install the container on a site

Prints the container's install code as the API returns it: snippet (the HTML to put in the
page) and, for some container types, containerConfig. To install a staging environment
instead, see `environment list` (its authorizationCode and environmentId go into the snippet).

```
gtmc container snippet [container]
```

### `gtmc container use`

Save the default container (GTM-XXXX, accounts/<a>/containers/<c>, or numeric ID with --account)

Resolves the container and saves its account ID, container ID and public ID to credentials.json.
The saved workspace is cleared because it belonged to the previous container;
pass --workspace to save one at the same time.

```
gtmc container use <container>
```

## `gtmc environment`

List, create, update and delete environments (staging installs of the container)

An environment is a separate install of the container — e.g. a staging site — that runs a
chosen version instead of the live one. Every container has the built-in Live and Latest
environments; user environments are created here.
A page loads an environment by adding gtm_auth=<authorizationCode>&gtm_preview=env-<environmentId>
&gtm_cookies_win=x to the gtm.js URL in the snippet. `environment list` prints that query string.
Deploy a version to a user environment with `environment update <id> --version <version-id>`;
this changes only pages that load that environment, not the live site.

Aliases: env

### `gtmc environment create`

Create a user environment (e.g. staging)

```
gtmc environment create --name <name> [--url ...] [--description ...] [--version <id>] [--enable-debug]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--description` | string | — | Environment description |
| `--enable-debug` | bool | `false` | Open the debug pane by default when previewing |
| `--name` | string | — | Environment name |
| `--url` | string | — | Default preview URL (the staging site) |
| `--version` | string | — | Container version ID to deploy to this environment |

### `gtmc environment delete`

Delete a user environment (requires --yes; pages still loading it stop getting tags)

```
gtmc environment delete <environment-id> --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc environment get`

Fetch one environment

```
gtmc environment get <environment-id>
```

### `gtmc environment list`

List environments with their deployed version and preview query string

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc environment reauthorize`

Regenerate an environment's authorization code (requires --yes; old links and snippets stop working)

Issues a new authorizationCode. Every snippet and shared preview link that carries the old
gtm_auth stops working until it is replaced — use it when a code has leaked.

```
gtmc environment reauthorize <environment-id> --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm regenerating the code |

### `gtmc environment update`

Change an environment; --version deploys a version to it (not to the live site)

Changes a user environment. --version <id> deploys that container version to the environment:
pages loading this environment start running it, the live site does not change.
Name and description can be changed only on user environments.

```
gtmc environment update <environment-id> [--version <id>] [--name ...] [--url ...] [--description ...] [--enable-debug]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--description` | string | — | Environment description |
| `--enable-debug` | bool | `false` | Open the debug pane by default when previewing |
| `--name` | string | — | Environment name |
| `--url` | string | — | Default preview URL (the staging site) |
| `--version` | string | — | Container version ID to deploy to this environment |

## `gtmc export`

Export the whole container as JSON in the GTM UI import format

Writes {exportFormatVersion, exportTime, containerVersion} — the same JSON as GTM's
Admin > Export Container, so it can be imported in the GTM UI (merge or overwrite).
--from workspace assembles it from the workspace's lists (several API calls).

```
gtmc export [--from live|latest|workspace|<version-id>] [-o file]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--from` | string | `live` | What to read: live (published), latest (newest version), workspace (current edits), or a version ID |
| `-o`, `--output` | string | — | Write to this file instead of stdout |

## `gtmc folder`

List, get, create, update, delete and revert folders in the workspace

Operates on folders in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.folder in a GTM UI export (`gtmc export`).

### `gtmc folder create`

Create a folder from API JSON

Creates a folder in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc folder create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc folder delete`

Delete folders from the workspace (requires --yes)

Deletes folders from the workspace. Until a version is created this can be undone with `folder revert <id>`; after a version is published it is gone
from the site.

```
gtmc folder delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc folder entities`

List the tags, triggers and variables in a folder

```
gtmc folder entities <folder-id>
```

### `gtmc folder get`

Fetch one folder as API JSON

```
gtmc folder get <id>
```

### `gtmc folder list`

List folders in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc folder move`

Move tags, triggers and variables into a folder

```
gtmc folder move <folder-id> [--tags 1,2] [--triggers 3] [--variables 4]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--tags` | stringSlice | `[]` | Tag IDs (comma separated) |
| `--triggers` | stringSlice | `[]` | Trigger IDs (comma separated) |
| `--variables` | stringSlice | `[]` | Variable IDs (comma separated) |

### `gtmc folder revert`

Discard the workspace's changes to folders (back to the latest version)

```
gtmc folder revert <id>...
```

### `gtmc folder update`

Update a folder: replace it from JSON, or change only name/notes

Update is a full replacement in the API. With --file, the JSON becomes the new folder
(fetch it with `folder get`, edit, and pass it back). Without --file, the current folder is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc folder update <id> [--file <json|->] [--name ...] [--notes ...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc gtag-config`

List, get, create, update and delete Google tag configs in the workspace

Operates on Google tag configs in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.gtagConfig in a GTM UI export (`gtmc export`).
Web containers. Has no name/notes and no revert in the API.

### `gtmc gtag-config create`

Create a gtag-config from API JSON

Creates a gtag-config in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc gtag-config create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc gtag-config delete`

Delete Google tag configs from the workspace (requires --yes)

Deletes Google tag configs from the workspace. The API has no revert for this kind: to undo, recreate it from `export` output or delete
the workspace; after a version is published it is gone
from the site.

```
gtmc gtag-config delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc gtag-config get`

Fetch one gtag-config as API JSON

```
gtmc gtag-config get <id>
```

### `gtmc gtag-config list`

List Google tag configs in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc gtag-config update`

Update a gtag-config: replace it from JSON

Update is a full replacement in the API. With --file, the JSON becomes the new gtag-config
(fetch it with `gtag-config get`, edit, and pass it back). Without --file, the current gtag-config is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc gtag-config update <id> [--file <json|->]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |

## `gtmc log`

Read the local audit log of every API request this CLI made

Every command that calls the API appends to a local audit log (JSON Lines, one file per day):
a command start/end record with the arguments and resolved target, and one record per HTTP
request (method, path, account/container/workspace, status, abbreviated request body).
Long strings and arrays in bodies are abbreviated; the OAuth token and --client-secret are never
written. The CLI refuses to call the API if the log cannot be opened.
Location: $GTM_LOG_DIR, else $XDG_STATE_HOME/google-tag-manager/logs, else
~/.local/state/google-tag-manager/logs. Files older than $GTM_LOG_RETENTION_DAYS days
(default 90) are deleted automatically.

### `gtmc log dir`

Print the audit log directory as a bare path (for cd, open, ls)

Prints only the directory the audit log is written to, followed by a newline, so it can be used
as `open "$(gtmc log dir)"` or `ls "$(gtmc log dir)"`. The directory may not exist yet
if no command has called the API. `log path` prints the same directory with the files and the
retention as JSON.

### `gtmc log path`

Print the audit log directory and its files

### `gtmc log show`

Print audit log records as JSON Lines (oldest first)

Prints matching records, one JSON object per line, oldest first. --limit keeps the newest N.
API records have event "api"; command records have event "command" and phase start/end.
With --account/--container-id/--writes only API records can match. --invocation keeps one CLI run
(its start record carries the arguments), so `--writes` then `--invocation <id>` answers
"which command made this change".

```
gtmc log show [--since 24h] [--account ID] [--container-id ID] [--writes] [--failed] [--invocation ID] [--limit N]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--account` | string | — | Only records for this account ID |
| `--container-id` | string | — | Only records for this numeric container ID |
| `--failed` | bool | `false` | Only failures: API requests with an error or status >= 400, and commands that ended with ok=false |
| `--invocation` | string | — | Only records of this CLI run (the "invocation" field) |
| `--limit` | int | `0` | Keep only the newest N records (0: all) |
| `--since` | string | `24h` | Start: a duration back from now (24h, 30m) or a date (2026-11-03) |
| `--writes` | bool | `false` | Only API requests that change something (non-GET) |

### `gtmc log summary`

Count API requests per account and container; --allow-account/--allow-container fail on anything else

Summarizes which accounts and containers were touched, with read (GET) and write counts.
Requests without an account in the path (accounts.list, containers.lookup before the result
is known) are counted under "unscoped".
--allow-account lists the accounts that may appear: the command exits 1 and lists the
violations if any API request touched another account. Use it to prove a session stayed
inside the accounts it was allowed to use. --allow-container does the same for numeric container
IDs (not GTM-XXXX): requests inside an account but outside any container (accounts.get,
containers.list) are not violations of it, requests in another container are.
The output also has failedRequests (error or status >= 400) and the write count per method.

```
gtmc log summary [--since 24h] [--allow-account ID,...] [--allow-container ID,...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--allow-account` | stringSlice | `[]` | Accounts that may appear; any other fails the command |
| `--allow-container` | stringSlice | `[]` | Numeric container IDs that may appear; any other fails the command |
| `--since` | string | `24h` | Start: a duration back from now (24h, 30m) or a date (2026-11-03) |

## `gtmc tag`

List, get, create, update, delete and revert tags in the workspace

Operates on tags in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.tag in a GTM UI export (`gtmc export`).

### `gtmc tag create`

Create a tag from API JSON

Creates a tag in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc tag create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc tag delete`

Delete tags from the workspace (requires --yes)

Deletes tags from the workspace. Until a version is created this can be undone with `tag revert <id>`; after a version is published it is gone
from the site.
Consider `tag pause` first to observe the effect before deleting.

```
gtmc tag delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc tag get`

Fetch one tag as API JSON

```
gtmc tag get <id>
```

### `gtmc tag list`

List tags in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc tag pause`

Pause tags (they stay in the container but never fire)

```
gtmc tag pause <tag-id>...
```

### `gtmc tag revert`

Discard the workspace's changes to tags (back to the latest version)

```
gtmc tag revert <id>...
```

### `gtmc tag unpause`

Resume paused tags

```
gtmc tag unpause <tag-id>...
```

### `gtmc tag update`

Update a tag: replace it from JSON, or change only name/notes/folder

Update is a full replacement in the API. With --file, the JSON becomes the new tag
(fetch it with `tag get`, edit, and pass it back). Without --file, the current tag is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc tag update <id> [--file <json|->] [--name ...] [--notes ...] [--folder <id>]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--folder` | string | — | Move into this folder ID (empty string: out of any folder) |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc template`

List, get, create, update, delete and revert templates in the workspace

Operates on templates in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.customTemplate in a GTM UI export (`gtmc export`).

### `gtmc template create`

Create a template from API JSON

Creates a template in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc template create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc template delete`

Delete templates from the workspace (requires --yes)

Deletes templates from the workspace. Until a version is created this can be undone with `template revert <id>`; after a version is published it is gone
from the site.

```
gtmc template delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc template get`

Fetch one template as API JSON

```
gtmc template get <id>
```

### `gtmc template import`

Import a template from the Community Template Gallery

Imports a Community Template Gallery template into the workspace. The gallery is backed by
GitHub: --owner and --repository are the template's GitHub owner and repository (shown on its
gallery page), --sha pins a commit (default: the latest).
A template declares permissions (URLs it may load, cookies, globals it may touch). The API
refuses the import unless they are acknowledged, so --acknowledge-permissions is required —
review the permissions on the gallery page first. After importing, `template get <id>` shows
them in templateData; the tag/variable type to use is cvt_<containerId>_<templateId>.

```
gtmc template import --owner <github-owner> --repository <repo> --acknowledge-permissions [--sha <commit>]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--acknowledge-permissions` | bool | `false` | Accept the permissions the template declares (required by the API) |
| `--owner` | string | — | GitHub owner of the gallery template |
| `--repository` | string | — | GitHub repository of the gallery template |
| `--sha` | string | — | Commit SHA to import (default: latest) |

### `gtmc template list`

List templates in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc template revert`

Discard the workspace's changes to templates (back to the latest version)

```
gtmc template revert <id>...
```

### `gtmc template update`

Update a template: replace it from JSON, or change only name/notes

Update is a full replacement in the API. With --file, the JSON becomes the new template
(fetch it with `template get`, edit, and pass it back). Without --file, the current template is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc template update <id> [--file <json|->] [--name ...] [--notes ...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc transformation`

List, get, create, update, delete and revert transformations in the workspace

Operates on transformations in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.transformation in a GTM UI export (`gtmc export`).
Server containers only.

### `gtmc transformation create`

Create a transformation from API JSON

Creates a transformation in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc transformation create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc transformation delete`

Delete transformations from the workspace (requires --yes)

Deletes transformations from the workspace. Until a version is created this can be undone with `transformation revert <id>`; after a version is published it is gone
from the site.

```
gtmc transformation delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc transformation get`

Fetch one transformation as API JSON

```
gtmc transformation get <id>
```

### `gtmc transformation list`

List transformations in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc transformation revert`

Discard the workspace's changes to transformations (back to the latest version)

```
gtmc transformation revert <id>...
```

### `gtmc transformation update`

Update a transformation: replace it from JSON, or change only name/notes/folder

Update is a full replacement in the API. With --file, the JSON becomes the new transformation
(fetch it with `transformation get`, edit, and pass it back). Without --file, the current transformation is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc transformation update <id> [--file <json|->] [--name ...] [--notes ...] [--folder <id>]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--folder` | string | — | Move into this folder ID (empty string: out of any folder) |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc trigger`

List, get, create, update, delete and revert triggers in the workspace

Operates on triggers in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.trigger in a GTM UI export (`gtmc export`).

### `gtmc trigger create`

Create a trigger from API JSON

Creates a trigger in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc trigger create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc trigger delete`

Delete triggers from the workspace (requires --yes)

Deletes triggers from the workspace. Until a version is created this can be undone with `trigger revert <id>`; after a version is published it is gone
from the site.

```
gtmc trigger delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc trigger get`

Fetch one trigger as API JSON

```
gtmc trigger get <id>
```

### `gtmc trigger list`

List triggers in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc trigger revert`

Discard the workspace's changes to triggers (back to the latest version)

```
gtmc trigger revert <id>...
```

### `gtmc trigger update`

Update a trigger: replace it from JSON, or change only name/notes/folder

Update is a full replacement in the API. With --file, the JSON becomes the new trigger
(fetch it with `trigger get`, edit, and pass it back). Without --file, the current trigger is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc trigger update <id> [--file <json|->] [--name ...] [--notes ...] [--folder <id>]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--folder` | string | — | Move into this folder ID (empty string: out of any folder) |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc variable`

List, get, create, update, delete and revert variables in the workspace

Operates on variables in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.variable in a GTM UI export (`gtmc export`).

### `gtmc variable create`

Create a variable from API JSON

Creates a variable in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc variable create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc variable delete`

Delete variables from the workspace (requires --yes)

Deletes variables from the workspace. Until a version is created this can be undone with `variable revert <id>`; after a version is published it is gone
from the site.

```
gtmc variable delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc variable get`

Fetch one variable as API JSON

```
gtmc variable get <id>
```

### `gtmc variable list`

List variables in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc variable revert`

Discard the workspace's changes to variables (back to the latest version)

```
gtmc variable revert <id>...
```

### `gtmc variable update`

Update a variable: replace it from JSON, or change only name/notes/folder

Update is a full replacement in the API. With --file, the JSON becomes the new variable
(fetch it with `variable get`, edit, and pass it back). Without --file, the current variable is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc variable update <id> [--file <json|->] [--name ...] [--notes ...] [--folder <id>]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--folder` | string | — | Move into this folder ID (empty string: out of any folder) |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |

## `gtmc version`

List, inspect, create, publish, rename, delete and undelete container versions

Aliases: ver

### `gtmc version create`

Create a version from the workspace (the workspace is consumed; nothing is published)

Saves the workspace as a new container version. GTM consumes the workspace in the process:
GTM creates a fresh workspace (newWorkspacePath) only when it was the container's last one.
If the consumed workspace was the saved default, the default moves to the new one, or is
cleared when no workspace was created.
Nothing changes on the site until `version publish`. Fails with compilerErrors if the
workspace has errors, and merge conflicts must be resolved first (`workspace status`).

```
gtmc version create --name <name> [--notes ...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--name` | string | — | Version name (required) |
| `--notes` | string | — | Version description: what changed and why |

### `gtmc version delete`

Delete a version from the history (requires --yes; `version undelete` restores it)

Hides a version from the version history. It does not change what is live, but a deleted
version can no longer be chosen for rollback until it is restored with `version undelete`
(`version list --include-deleted` shows it). The live version is refused.

```
gtmc version delete <version-id> --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc version get`

Fetch a whole version (tags, triggers, variables, ...); default: live

```
gtmc version get [live|latest|<version-id>]
```

### `gtmc version list`

List version headers (ID, name, entity counts) without their contents

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |
| `--include-deleted` | bool | `false` | Include deleted versions |

### `gtmc version publish`

Publish a version to the live site (requires --yes)

Makes the version live: every site using the container starts running it within minutes.
To roll back, publish the previous version ID again (see `version list`).

```
gtmc version publish <version-id|latest> --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm publishing to the live site |

### `gtmc version set-latest`

Make a version the base that workspaces sync with and are compared against

Sets which version counts as "latest": workspaces sync from it and their changes and
conflicts are computed against it. It does not publish anything.

```
gtmc version set-latest <version-id>
```

### `gtmc version undelete`

Restore a deleted version

```
gtmc version undelete <version-id>
```

### `gtmc version update`

Rename a version or rewrite its notes (the contents cannot be changed)

```
gtmc version update <version-id|live|latest> [--name ...] [--notes ...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--name` | string | — | New version name |
| `--notes` | string | — | New version description |

## `gtmc workspace`

Create, inspect, preview, sync, bulk-edit and delete workspaces; resolve conflicts; choose the default

Every edit in GTM happens in a workspace and stays invisible to the site until a version is
created from it and published. Create a dedicated workspace for each piece of work.

Aliases: ws

### `gtmc workspace bulk-update`

Apply many entity changes in one call; new entities can reference each other by new_N IDs

Applies a list of changes to the workspace in one API call. The body is
{"changes": [Entity, ...]} (a bare array is accepted too). Each Entity holds one entity
(tag, trigger, variable, folder, client, transformation, zone, customTemplate, gtagConfig)
and a changeStatus: added, updated or deleted.
New entities need temporary IDs of the form new_1, new_2, ... and may refer to each other
with them — e.g. a tag whose firingTriggerId is ["new_2"] and parentFolderId is "new_1".
This is how to copy a group of entities from another container's export with their
references intact: give each copied entity a new_N ID and rewrite the references to match.
Changes containing changeStatus "deleted" require --yes.

```
gtmc workspace bulk-update --file <json|-> [--yes]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Changes JSON ({"changes": [...]}); - for stdin |
| `--yes` | bool | `false` | Confirm when the changes include deletions |

### `gtmc workspace create`

Create a workspace (starts from the latest version)

```
gtmc workspace create --name <name>
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--description` | string | — | Workspace description |
| `--name` | string | — | Workspace name (required) |
| `--use` | bool | `false` | Also save it as the default workspace |

### `gtmc workspace delete`

Delete a workspace and every unsaved change in it (requires --yes)

```
gtmc workspace delete <workspace-id> --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc workspace info`

Fetch workspace details

```
gtmc workspace info [workspace-id]
```

### `gtmc workspace list`

List workspaces in the container

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc workspace quick-preview`

Compile the workspace without creating a version (exit 1 on compiler errors)

Compiles the workspace as if creating a version, without creating one or consuming the
workspace. Run it after editing and before `version create` to catch compiler errors.
Prints compilerError, compilerErrors, syncStatus and diagnostics; --full adds the compiled
containerVersion. Exits 1 when the workspace does not compile.
The API often reports compilerError without any compilerErrors detail, so on failure the CLI
adds diagnostics: references to {{variables}} that do not exist (the usual cause — the API
accepts them on write but cannot compile them; unknown trigger IDs are rejected on write).
This does not open GTM's Preview mode (Tag Assistant); that is only available in the UI.

```
gtmc workspace quick-preview [workspace-id]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--full` | bool | `false` | Include the compiled containerVersion |

### `gtmc workspace resolve-conflict`

Resolve a merge conflict after `workspace sync`

A sync can leave merge conflicts: the same entity changed both in this workspace and in a
version created meanwhile (see `workspace status`). Resolve each one before `version create`.
--take workspace keeps this workspace's side, --take base takes the newer version's side.
--file posts your own merged entity instead, in the Entity shape: {"tag": {...}} (or trigger,
variable, folder, client, transformation, zone, customTemplate, builtInVariable, gtagConfig);
to resolve as a deletion add "changeStatus": "deleted".
--kind is the kind shown by `workspace status --format table` (tag, trigger, variable, ...).

```
gtmc workspace resolve-conflict --kind <kind> --id <id> (--take workspace|base | --file <entity-json|->)
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Merged entity JSON ({"tag": {...}}); - for stdin |
| `--id` | string | — | Entity ID (for builtInVariable: its type) |
| `--kind` | string | — | Entity kind: tag, trigger, variable, folder, template, client, transformation, zone, builtInVariable, gtagConfig |
| `--take` | string | — | Keep one side: workspace or base |

### `gtmc workspace status`

Show the workspace's unsaved changes and merge conflicts

Lists the entities changed in the workspace compared with the latest version (changeStatus:
added / deleted / updated) and any merge conflicts. Check this before `version create`.

```
gtmc workspace status [workspace-id]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc workspace sync`

Bring the latest version's changes into the workspace

```
gtmc workspace sync [workspace-id]
```

### `gtmc workspace update`

Rename a workspace or change its description

```
gtmc workspace update [workspace-id] [--name ...] [--description ...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--description` | string | — | New description |
| `--name` | string | — | New name |

### `gtmc workspace use`

Save the default workspace (together with its container)

```
gtmc workspace use <workspace-id>
```

## `gtmc zone`

List, get, create, update, delete and revert zones in the workspace

Operates on zones in the target workspace. Changes are not live until a version is created
and published (`version create`, `version publish`).
Create/update bodies are the Tag Manager API JSON — the same shape as one element of
containerVersion.zone in a GTM UI export (`gtmc export`).
Web containers on Tag Manager 360 only; free accounts get 403.

### `gtmc zone create`

Create a zone from API JSON

Creates a zone in the workspace from a JSON file (- reads stdin).
IDs, paths and fingerprints in the body are ignored, so an element copied from another
container's export can be used — but its references (trigger IDs, folder IDs, {{variable}}
names, cvt_ template types) must exist in this container.

```
gtmc zone create --file <json|->
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | JSON body (API shape); - for stdin |

### `gtmc zone delete`

Delete zones from the workspace (requires --yes)

Deletes zones from the workspace. Until a version is created this can be undone with `zone revert <id>`; after a version is published it is gone
from the site.

```
gtmc zone delete <id>... --yes
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--yes` | bool | `false` | Confirm the deletion |

### `gtmc zone get`

Fetch one zone as API JSON

```
gtmc zone get <id>
```

### `gtmc zone list`

List zones in the workspace

| flag | type | default | description |
| --- | --- | --- | --- |
| `--format` | string | `json` | Output format: json or table |

### `gtmc zone revert`

Discard the workspace's changes to zones (back to the latest version)

```
gtmc zone revert <id>...
```

### `gtmc zone update`

Update a zone: replace it from JSON, or change only name/notes

Update is a full replacement in the API. With --file, the JSON becomes the new zone
(fetch it with `zone get`, edit, and pass it back). Without --file, the current zone is fetched
and only the given fields are changed. Field flags are applied on top of --file too.
The fingerprint in the file is used for optimistic locking when present (409 if someone
changed it meanwhile); otherwise the current fingerprint is used.

```
gtmc zone update <id> [--file <json|->] [--name ...] [--notes ...]
```

| flag | type | default | description |
| --- | --- | --- | --- |
| `--file` | string | — | Replacement JSON body (API shape); - for stdin |
| `--name` | string | — | New name |
| `--notes` | string | — | New notes |
