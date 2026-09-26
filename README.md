# Metalhost CLI

`metalhost` is the public command-line client for AES Metalhost. The current stable release is `v1.0.0`. It uses the public `github.com/AES-Services/metalhost-sdk` generated Connect clients.

## Install

macOS and Linux installer:

```sh
curl -fsSL https://metalhost.net/install-cli.sh | bash
metalhost version
```

Pin the current release or choose a destination:

```sh
curl -fsSL https://metalhost.net/install-cli.sh | VERSION=v1.0.0 bash
curl -fsSL https://metalhost.net/install-cli.sh | INSTALL_DIR="$HOME/.local/bin" bash
```

The installer downloads a release archive and verifies it when `checksums.txt` is available (`sha256sum` is required for that check). It supports macOS and Linux; it does not install on Windows.

GitHub Releases contains prebuilt `.tar.gz` archives for Linux/macOS and `.zip` archives for Windows, plus `checksums.txt`. Releases are not currently cryptographically signed. Windows users can download the appropriate ZIP, verify its SHA-256 entry in `checksums.txt`, extract `metalhost.exe`, and place it on `PATH`.

Install or build with Go on any supported platform:

```sh
go install github.com/AES-Services/metalhost-cli/cmd/metalhost@v1.0.0
```

```sh
make ci
./bin/metalhost version
```

`make ci` runs `go test ./...` and builds the binary.

## Initialize and authenticate

For an interactive first run:

```sh
metalhost init
metalhost auth whoami
```

`init` creates or updates a profile, authenticates with an existing API key, email/password, or signup flow, and selects project and datacenter defaults. OIDC is presented by the wizard but currently redirects you to run `metalhost auth login --oidc google|github`. `--force` allows profile replacement; `--non-interactive` fails instead of prompting and is not a complete unattended initializer.

Direct authentication flows include:

```sh
metalhost auth signup --endpoint https://api.metalhost.net
metalhost auth verify --endpoint https://api.metalhost.net --token TOKEN
metalhost auth login --endpoint https://api.metalhost.net --email you@example.com
metalhost auth login --endpoint https://api.metalhost.net --oidc google
METALHOST_API_KEY=aes_... metalhost auth login --api-key
metalhost auth whoami
```

Email/password and OIDC logins complete MFA interactively when required. `auth mfa-submit` supports a separate scripted challenge-token flow, and `auth link --oidc` links an identity to an authenticated account. Credentials are stored in the selected profile with file mode `0600`.

## Configuration and precedence

Default config paths:

- macOS/Linux: `$XDG_CONFIG_HOME/metalhost/config.yaml`, or `~/.config/metalhost/config.yaml` when `XDG_CONFIG_HOME` is unset.
- Windows: `%AppData%\Metalhost\config.yaml`.

`--config PATH` selects another file. `--profile NAME` selects a profile for one invocation; otherwise `METALHOST_PROFILE`, then `current_profile`, is used.

For endpoint, format, and scope, command flags override environment variables, which override profile values. Command-local scope flags take precedence over global scope flags. Supported variables are:

```text
METALHOST_PROFILE
METALHOST_ENDPOINT
METALHOST_API_KEY
METALHOST_ORGANIZATION
METALHOST_PROJECT
METALHOST_REGION
METALHOST_FORMAT
```

Use fully qualified scope names, including `organizations/acme`, `projects/demo`, and `datacenters/us-dal-1`.

All commands inherit:

```text
--config PATH
--profile NAME
--endpoint URL
-o, --format table|json|yaml
-q, --quiet
--project NAME
--org NAME
--region NAME
--wait
--wait-timeout DURATION
```

`--quiet` overrides the configured format and prints resource names for scripting.

## Everyday resource commands

Service-specific commands expose the full action and flag set:

```sh
metalhost catalog datacenter list
metalhost project list --org organizations/acme
metalhost vm list --project projects/demo
metalhost vm create --project projects/demo --region datacenters/us-dal-1 --vcpus 4 --ram-gib 16 --cpu-class cascadelake --image ubuntu-24-04 --disk-size-gib 50
metalhost disk create --project projects/demo --region datacenters/us-dal-1 --size-gib 100 --class nvme
metalhost network create --project projects/demo --region datacenters/us-dal-1 --id app --subnet-cidr-v4 10.10.0.0/24
metalhost wallet account list --org organizations/acme
```

Top-level `get`, `describe`, and `delete` provide one grammar for registered kinds:

```sh
metalhost get vm
metalhost get disk projects/demo/disks/data -o yaml
metalhost describe project projects/demo
metalhost delete webhook projects/demo/webhook-subscriptions/builds --yes
```

Canonical kinds are `vm`, `ssh-key`, `disk`, `file-share`, `network`, `baremetal`, `webhook`, and `project`; aliases are accepted. `get KIND` lists and supports `--page-size`, `--page-token`, `--limit`, and `--all`. `get KIND NAME` and `describe` fetch one. Delete is available only when the API exposes a delete RPC and prompts unless `--yes` is passed.

`apply` accepts YAML or JSON:

```sh
metalhost apply -f disk.yaml
metalhost get disk projects/demo/disks/data -o yaml |
  metalhost apply --kind disk -f -
```

An envelope document has `kind`, optional `parent`, and `spec`. A bare resource document requires `--kind` and, for creates without another project default, `--parent`. A populated `spec.name` selects update; no name selects create.

## Output, waiting, and exits

- `table` is the default human format.
- `json` uses protobuf JSON names and enum strings.
- `yaml` is produced from protobuf JSON semantics.
- `--quiet`/name output emits only resource names.

`--wait` inspects operation-bearing responses, polls every two seconds, and replaces the original response with the terminal operation. Its default timeout is 10 minutes; `--wait-timeout 0` removes the limit. `metalhost ops wait NAME` is the explicit polling command and defaults to `--timeout 30m`.

Terminal operation states, including `FAILED` and `CANCELLED`, are rendered as responses; scripts must inspect the operation state. `ops wait` also returns the last operation without an error when its timeout is reached. Transport/RPC errors and the global `--wait` timeout are nonzero.

Stable error exits are `0` success, `1` generic, `2` invalid argument/failed precondition/out of range, `3` permission denied, `4` unauthenticated, `5` not found, `6` unavailable/deadline exceeded, `9` already exists, and `130` interrupted.

See [Command reference](docs/COMMAND_REFERENCE.md) and [Scripting guide](docs/SCRIPTING.md).

## Command groups

The root command includes `init`, `auth`, `profile`, `get`, `describe`, `delete`, `apply`, `iam`, `catalog`, `health`, `project`, `org`, `ops`, `vm`, `storage`, `disk`, `file-share`, `network`, `firewall`, `wallet`, `quota`, `audit`, `baremetal`, `webhook`, `support`, `completion`, and `version`.

Use `metalhost COMMAND --help` for command-local flags and actions.

## Shell completion

Generate completion for Bash, Zsh, Fish, or PowerShell:

```sh
metalhost completion bash > ~/.local/share/bash-completion/completions/metalhost
metalhost completion zsh > "${fpath[1]}/_metalhost"
metalhost completion fish > ~/.config/fish/completions/metalhost.fish
```

PowerShell:

```powershell
metalhost completion powershell | Out-String | Invoke-Expression
```

## Releases

Pushing a `v*` tag runs tests and GoReleaser, then publishes GitHub release archives and checksums. For the next compatible release:

```sh
VERSION=v1.0.1
git tag "$VERSION"
git push origin "$VERSION"
```
