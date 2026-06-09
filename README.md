# Metalhost CLI

Public CLI for AES Metalhost.

The `metalhost` binary is a human and automation-friendly wrapper around the public Metalhost API and SDK.

## Current Status

The CLI is in early public release. It covers the customer-facing command surface for health, catalog, projects, operations, VMs, disks, file shares, networking, bare-metal, wallets, quotas, audit, support, IAM keys, SSH keys, and webhooks.

The VM lifecycle has been validated end-to-end against a lab control plane: `metalhost vm create` starts an operation and reaches KubeVirt, and `metalhost vm delete` removes the matching VM resources. Customer production readiness still depends on the service endpoint's capacity checks, VM state reconciliation, persistent boot disk setup, and billing settlement.

## Install (macOS / Linux)

```sh
curl -fsSL https://metalhost.net/install-cli.sh | bash
metalhost version
```

Pin a version or install directory:

```sh
VERSION=v1.0.0-rc6 curl -fsSL https://metalhost.net/install-cli.sh | bash
INSTALL_DIR=~/.local/bin curl -fsSL https://metalhost.net/install-cli.sh | bash
```

The script is also in this repo at `scripts/install.sh`.

## Install From Source

```sh
go install github.com/AES-Services/metalhost-cli/cmd/metalhost@latest
```

## Download Binaries

Signed release tags publish prebuilt binaries for Linux, macOS, and Windows on the GitHub Releases page. Archives include the `metalhost` binary, `README.md`, `LICENSE`, and a `checksums.txt` file for verification.

## Build From Source

```sh
make ci
./bin/metalhost version
```

## Quick Start

```sh
metalhost profile create default --endpoint https://api.metalhost.example
metalhost profile use default
metalhost auth login --api-key
metalhost auth whoami
```

For automation:

```sh
METALHOST_ENDPOINT=https://api.example.com \
METALHOST_API_KEY=... \
metalhost auth whoami --format json
```

## Command Surface

The CLI is organized around Metalhost resources:

```sh
metalhost catalog datacenter list
metalhost catalog pricing quote --vcpus 4 --ram-gib 16 --cpu-class cascadelake --boot-disk-gib 50
metalhost project list --org organizations/acme
metalhost vm list --project projects/demo
metalhost vm create --project projects/demo --region datacenters/us-dal-1 --vcpus 4 --ram-gib 16 --cpu-class cascadelake --image ubuntu-24-04 --disk-size-gib 50
metalhost compute ssh-key list --project projects/demo
metalhost disk create --project projects/demo --region datacenters/us-dal-1 --size-gib 100 --class nvme
metalhost network create --project projects/demo --region datacenters/us-dal-1 --id app --subnet-cidr-v4 10.10.0.0/24
metalhost ops wait operations/01HY...
metalhost wallet account list --org organizations/acme
metalhost quota --project projects/demo
metalhost audit search --project projects/demo --since 24h
metalhost iam keys create --display-name ci --project projects/demo
```

Nested resource groups are available both under their service namespace and as top-level shortcuts where that is nicer for daily use, for example `metalhost storage disk list` and `metalhost disk list`.

## Configuration

Profiles live in the user config directory by default and can be overridden with `--config`.

Environment variables override profile values:

```sh
METALHOST_ENDPOINT=https://api.metalhost.example
METALHOST_API_KEY=...
METALHOST_PROJECT=projects/demo
METALHOST_REGION=datacenters/dfw1
METALHOST_FORMAT=json
```

## SDK

This CLI uses the public `github.com/AES-Services/metalhost-sdk` module.

## Releases

Maintainers publish releases by pushing a version tag:

```sh
VERSION=v0.0.3
git tag -s "$VERSION" -m "$VERSION"
git push origin "$VERSION"
```

Use the next semver patch/minor tag for subsequent public CLI releases.
