# Metalhost CLI

Public CLI for AES Metalhost.

The `metalhost` binary is a human and automation-friendly wrapper around the public Metalhost API and SDK.

## Current Status

The CLI is in early public release. It covers the customer-facing command surface for health, catalog, projects, operations, VMs, images, disks, networking, object storage, wallets, quotas, audit, IAM keys, and webhooks.

The VM lifecycle has been validated end-to-end against a lab control plane: `metalhost vm create` starts an operation and reaches KubeVirt, and `metalhost vm delete` removes the matching VM resources. Customer production readiness still depends on the service endpoint's capacity checks, VM state reconciliation, persistent boot disk setup, and billing settlement.

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
FOUNDRY_ENDPOINT=https://api.example.com \
FOUNDRY_API_KEY=... \
metalhost auth whoami --format json
```

## Command Surface

The CLI is organized around Metalhost resources:

```sh
metalhost catalog datacenter list
metalhost catalog sku list
metalhost project resource list --org organizations/acme
metalhost vm list --project projects/demo
metalhost vm create --project projects/demo --region datacenters/dfw1 --sku skus/vm.cascadelake.c2m4 --image projects/demo/images/ubuntu
metalhost image list --project projects/demo
metalhost disk create --project projects/demo --region datacenters/dfw1 --size-gib 100 --class ssd
metalhost network create --project projects/demo --region datacenters/dfw1 --id app --cidr 10.10.0.0/24
metalhost bucket create app-artifacts --project projects/demo
metalhost bucket object presign-upload app-artifacts --object releases/app.tar.gz
metalhost ops wait operations/01HY...
metalhost wallet list --billing-account billingAccounts/acme
metalhost quota --project projects/demo
metalhost audit search --project projects/demo --since 24h
metalhost iam keys create --display-name ci --project projects/demo
```

Nested resource groups are available both under their service namespace and as top-level shortcuts where that is nicer for daily use, for example `metalhost storage disk list` and `metalhost disk list`.

## Configuration

Profiles live in the user config directory by default and can be overridden with `--config`.

Environment variables override profile values:

```sh
FOUNDRY_ENDPOINT=https://api.metalhost.example
FOUNDRY_API_KEY=...
FOUNDRY_PROJECT=projects/demo
FOUNDRY_REGION=datacenters/dfw1
FOUNDRY_FORMAT=json
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
