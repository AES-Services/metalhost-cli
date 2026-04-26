# Foundry CLI

Public CLI for AES Foundry.

The `foundry` binary is a human and automation-friendly wrapper around the public Foundry API and SDK.

## Install From Source

```sh
go install github.com/AES-Services/foundry-cli/cmd/foundry@latest
```

## Download Binaries

Signed release tags publish prebuilt binaries for Linux, macOS, and Windows on the GitHub Releases page. Archives include the `foundry` binary, `README.md`, `LICENSE`, and a `checksums.txt` file for verification.

## Build From Source

```sh
make ci
./bin/foundry version
```

## Quick Start

```sh
foundry profile create default --endpoint https://api.foundry.example
foundry profile use default
foundry auth login --api-key
foundry auth whoami
```

For automation:

```sh
FOUNDRY_ENDPOINT=https://api.example.com \
FOUNDRY_API_KEY=... \
foundry auth whoami --format json
```

## Command Surface

The CLI is organized around Foundry resources:

```sh
foundry catalog datacenter list
foundry catalog sku list
foundry project resource list --org organizations/acme
foundry vm list --project projects/demo
foundry vm create --project projects/demo --region datacenters/dfw1 --sku vm.small --image projects/demo/images/ubuntu
foundry image list --project projects/demo
foundry disk create --project projects/demo --region datacenters/dfw1 --size-gib 100 --class ssd
foundry network create --project projects/demo --region datacenters/dfw1 --id app --cidr 10.10.0.0/24
foundry bucket create app-artifacts --project projects/demo
foundry bucket object presign-upload app-artifacts --object releases/app.tar.gz
foundry ops wait operations/01HY...
foundry wallet list --billing-account billingAccounts/acme
foundry quota --project projects/demo
foundry audit search --project projects/demo --since 24h
foundry iam keys create --display-name ci --project projects/demo
```

Nested resource groups are available both under their service namespace and as top-level shortcuts where that is nicer for daily use, for example `foundry storage disk list` and `foundry disk list`.

## Configuration

Profiles live in the user config directory by default and can be overridden with `--config`.

Environment variables override profile values:

```sh
FOUNDRY_ENDPOINT=https://api.foundry.example
FOUNDRY_API_KEY=...
FOUNDRY_PROJECT=projects/demo
FOUNDRY_REGION=datacenters/dfw1
FOUNDRY_FORMAT=json
```

## SDK

This CLI uses the public `github.com/AES-Services/foundry-sdk` module.

## Releases

Maintainers publish releases by pushing a version tag:

```sh
git tag -s v0.0.1 -m "v0.0.1"
git push origin v0.0.1
```
