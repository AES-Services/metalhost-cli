# Scripting with `metalhost`

Prefer environment variables over persisted credentials in CI:

```sh
export METALHOST_ENDPOINT=https://api.metalhost.net
export METALHOST_API_KEY=aes_...
export METALHOST_PROJECT=projects/demo
export METALHOST_REGION=datacenters/us-dal-1

metalhost get vm --all -o json
```

Available configuration variables are `METALHOST_PROFILE`, `METALHOST_ENDPOINT`, `METALHOST_API_KEY`, `METALHOST_ORGANIZATION`, `METALHOST_PROJECT`, `METALHOST_REGION`, and `METALHOST_FORMAT`. Explicit flags override environment/profile values. Use `--config PATH` when a job needs an isolated profile file.

## Machine-readable output

- Use `-o json` for protobuf JSON field names and enum strings.
- Use `-o yaml` for the equivalent YAML representation.
- Use `-q` when only resource names are needed.
- Use `--all` on list commands to follow every `next_page_token`.
- `--limit N` lowers the requested page size; it is not a client-side cap across multiple pages.

Do not parse the default table output.

## Authentication

For an existing key, no login command is necessary: setting `METALHOST_API_KEY` authenticates requests. `auth login --api-key` copies that environment value into the active profile and is mainly for persistent local setup.

Email/password and OIDC login can prompt or open a browser. For a headless two-leg MFA flow, use:

```sh
metalhost auth mfa-submit \
  --endpoint "$METALHOST_ENDPOINT" \
  --challenge "$CHALLENGE_TOKEN" \
  --totp "$TOTP"
```

`--totp` and `--recovery` are alternatives. The resulting secret is saved to the selected/current profile (or a new `default` profile).

## Operations and exit status

For operation-bearing creates, updates, and deletes:

```sh
result="$(metalhost vm create ... --wait --wait-timeout 15m -o json)"
```

`--wait` polls every two seconds. It prints the final operation for `SUCCEEDED`, `FAILED`, or `CANCELLED`; those terminal states do not themselves select the process exit code, so inspect the returned state.

`metalhost ops wait NAME --timeout 30m` also prints the last operation and exits successfully when its timeout elapses. If timeout must be an error in a script, enforce it externally or inspect whether the returned state is terminal.

The CLI maps request failures to:

```text
0    success
1    generic/local error
2    invalid argument, failed precondition, out of range
3    permission denied
4    unauthenticated
5    not found
6    unavailable or deadline exceeded
9    already exists
130  interrupted
```

Global `--wait` timeout is an error. Destructive commands may prompt; pass the leaf command's `--yes` flag when available.

## Declarative pipelines

`get KIND NAME -o yaml` emits the bare resource shape accepted by `apply --kind`:

```sh
metalhost get disk projects/demo/disks/data -o yaml > disk.yaml
# edit mutable fields
metalhost apply --kind disk -f disk.yaml --wait
```

For creation, omit `name` and provide a parent through the manifest, `--parent`, `--project`, `METALHOST_PROJECT`, or the active profile.
