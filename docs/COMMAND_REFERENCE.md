# Command reference

This is a compact map of the Cobra command tree. Run `metalhost COMMAND --help` for the authoritative flags and examples for a specific command.

## Setup and identity

- `init`: interactive endpoint, authentication, project, and datacenter setup (`--force`, `--non-interactive`).
- `profile`: `list`, `create`, `use`, `set`, and `delete`.
- `auth`: `signup`, `verify`, `login`, `whoami`, `link`, and `mfa-submit`.
- `iam`: API `keys`, organization `members`, `sessions`, `mfa`, `invites`, `user`, `password`, `notifications`, and `import-github-ssh-keys`.

## Unified verbs

- `get KIND [NAME]`: list a registered kind or fetch one; list mode supports `--page-size`, `--page-token`, `--limit`, and `--all`.
- `describe KIND NAME`: fetch and render one resource.
- `delete KIND NAME`: delete when that kind has a Delete RPC; `--yes` skips confirmation.
- `apply -f FILE`: create or update from YAML/JSON; `-f -` reads stdin, and bare resource documents use `--kind` plus optional `--parent`.

Canonical unified kinds are:

```text
vm ssh-key disk file-share network baremetal webhook project
```

Not every verb is available for every kind. For example, `network` is list-only in the unified registry and `baremetal` has no unified delete. Service-specific trees expose additional actions.

## Service command groups

- `catalog`: `capacity`, `datacenter`, and `pricing`.
- `health`: API health checks.
- `project`, `org`: project and organization lifecycle/activity.
- `ops` (`operations`): `list`, `get`, and `wait`.
- `vm`: virtual-machine lifecycle and VM-specific actions.
- `storage`: nested disk and file-share commands.
- `disk`: top-level disk shortcut and disk actions.
- `file-share`: top-level file-share shortcut.
- `network`: tenant network lifecycle and network actions.
- `firewall`: firewall-rule management.
- `baremetal`: offers, inventory, quote, order, lifecycle, power, reinstall, rescue, boot device, console, and ISO/virtual-media operations.
- `wallet`: accounts, wallets, top-ups, payments, invoices, and usage.
- `quota`: current project quotas.
- `audit`: event search.
- `webhook`: subscriptions and delivery attempts.
- `support`: customer support operations.

## Utility commands

- `completion bash|zsh|fish|powershell`: emit a completion script.
- `version`: print version, commit, and build date.

## Global flags

```text
--config PATH             alternate config file
--profile NAME            profile for this invocation
--endpoint URL            API endpoint
-o, --format FORMAT       table, json, or yaml
-q, --quiet               resource names only
--project NAME            project scope
--org NAME                organization scope
--region NAME             datacenter scope
--wait                    poll an operation-bearing response
--wait-timeout DURATION   global wait limit; default 10m, 0 means unlimited
```

Pagination flags recur on list commands. Destructive service commands generally use `--yes`; consult the leaf command's help rather than assuming it is present.
