# Scoped automation and GitHub Actions

The September backend introduces scoped personal credentials, project service
accounts and GitHub workload identity. These require an explicitly enabled
environment and a coordinated backend release; a CLI/SDK version alone does not
enable them. Managed GitHub runners are planned separately, not part of this
authentication feature.

## What this CLI supports

The command tree in `internal/command/root.go` and `auth_flows.go` is
authoritative. The September branch implements `metalhost auth github`,
`metalhost automation` and `metalhost monitoring`; the older v1.1.0-based
checkout does not. Use a matching September CLI build and check its `--help`.
See [the command reference and workflow example](OBSERVABILITY.md).
`metalhost auth login --oidc github` is interactive human sign-in, not
GitHub Actions workflow authentication. The HTTP examples and matching SDK
remain alternatives that do not require the CLI helper.

Existing resource commands can use a scoped credential through
`METALHOST_API_KEY` where the requested RPC is on the backend's automation
allowlist. Permission names do not grant an entire service. In particular,
console access, IAM administration and arbitrary action submission are denied.
Provisioning resources remains a separate operation that may incur charges.

## Read-only example

Create a project service-account credential with `compute.read` in an enabled
dashboard, load it from a secret manager (do not commit it or use shell tracing),
then select the same project's canonical name and API origin:

```sh
export METALHOST_ENDPOINT="https://API_HOST"
export METALHOST_PROJECT="projects/PROJECT_ID"
# METALHOST_API_KEY is supplied securely by your secret manager.
metalhost vm list --format json
```

Replace the placeholders with the real enabled environment. This lists VMs;
it does not create them or establish guest SSH access. Follow pagination for a
complete inventory, and check the installed version's `--help` before assuming
additional flags exist.

## GitHub workflows and monitoring

The coordinated website docs provide:

- [GitHub pairing and complete HTTP exchange workflow](https://metalhost.net/docs/developers/guides/github-actions).
- [Typed monitoring, PromQL, Grafana and Prometheus examples](https://metalhost.net/docs/developers/observability).
- [Scoped keys, service accounts and rotation](https://metalhost.net/docs/dashboard/guides/automation).

These links require the corresponding documentation release. Workflow setup
verifies the exact signed repository/workflow identity before human approval.
Exchange produces a 15-minute token. Keep it in a single job step, mask it, and
never persist it in a CLI profile, GitHub artifact or `GITHUB_ENV`. The workflow
continues to run on GitHub-hosted or customer-operated runners.

API clients must retain the exact request body and request UUID after an
ambiguous create/rotate response. A successful retry can return metadata with
`secretUnavailable`; it cannot recover the one-time secret. Revoke the unusable
credential before issuing a replacement. Overlap rotation is a scoped-credential
feature, not an automatic change to legacy `iam keys rotate` semantics.
