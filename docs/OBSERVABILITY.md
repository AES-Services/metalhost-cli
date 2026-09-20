# September observability (unreleased)

These commands require the coordinated September backend and SDK release.
They do not enable monitoring or modify running VMs by themselves. Existing
`iam keys` commands retain their legacy behavior; use `automation keys` for
new project-scoped capability-limited credentials.

## Explore and configure

```sh
metalhost automation capabilities
metalhost automation accounts list --project projects/example
metalhost automation keys list --project projects/example
metalhost automation github list --project projects/example
metalhost monitoring catalog --project projects/example
metalhost monitoring vms --project projects/example
metalhost monitoring rules list --project projects/example
metalhost monitoring incidents list --project projects/example
metalhost monitoring incidents summary --project projects/example
metalhost monitoring destinations list --project projects/example
metalhost monitoring config --target grafana --project projects/example
metalhost monitoring config --target prometheus --project projects/example
metalhost monitoring promql 'sum(metalhost_vm_observed_running)' --project projects/example --format json
```

Each leaf command accepts its public SDK protobuf JSON request using `--file
request.json` or `--file -` for stdin. Use `--format json` for structured output.
Unknown fields are rejected; input is capped at 1 MiB and output at 4 MiB.
List requests return the server cursor; pass `page_token` in the next request.
`--project` fills only an omitted top-level `project_name`, never a nested scope.

Mutations require an explicit file. Preserve the exact `request_id`, resource ID,
and `expected_version` after an ambiguous failure. Do not issue a new creation
because its response was lost. Requests are not automatically retried.

Example scoped key request:

```json
{
  "project_name": "projects/example",
  "display_name": "Metrics scraper",
  "permissions": ["monitoring.read"],
  "request_id": "11111111-1111-4111-8111-111111111111"
}
```

Generate a fresh UUID for a new creation (the example is illustrative), then:

```sh
metalhost automation keys create --file key-request.json --secret-file ./metrics-key
```

The secret is written only to the explicitly requested new `0600` file, never
printed. An existing file is rejected before the RPC. Keep the file out of source
control, caches, and artifacts. Rotation uses the same secret-file requirement.
If the original one-time response was lost, retry returns metadata only: revoke
that recovered credential before creating another.

Rules expose `save`, `preview`, `delete`, and `instances`; incidents expose `get`
and `update` (acknowledge/snooze); destinations expose `save`, `delete`, `verify`,
`request-verification`, `test`, and `test-status`. Creation is not native evaluator
acceptance. Inspect applied version and evaluation health. SENT means provider
acceptance, not an inbox/read receipt. Missing telemetry does not imply recovery.

`monitoring config` outputs configuration with placeholders/file paths, never
your actual API key. Protect the referenced Prometheus credential file. Hosted
PromQL uses the current project's immutable tenant boundary; no custom tenant
header or storage URL is accepted. For a range query, provide both `--start` and
`--end` in RFC3339 format plus `--step 30s` (or longer). The server enforces its
own query and response budgets. Redirects are never followed with credentials.

For webhook secrets, use the private-file commands:

```sh
metalhost webhook create-from-file --file endpoint.json --secret-file ./webhook-secret
metalhost webhook rotate --file rotation.json --secret-file ./replacement-secret
```

Creation should include a stable `webhook-subscriptions/<UUID>` name. After a
lost creation response, inspect that exact name instead of issuing a new name.
Rotation requires `name`, `expected_secret_version`, `request_id` and
`overlap_seconds` (0–86400). A lost rotation response is recoverable with the
same request; its secret is not retrievable. To invalidate a lost secret, rotate
the current version again with zero overlap. Upgrade receivers to timestamped
V2 signatures before enabling overlap. The older flag-based `webhook create`
command retains its existing output behavior for compatibility; do not use it
in logged CI jobs.

## GitHub Actions without a saved key

Approve the actual workflow through Developers → GitHub Actions first. Use its
exact policy name and environment-specific audience, not an example subject.

```yaml
permissions:
  contents: read
  id-token: write
steps:
  # Install a pinned, checksum-verified Metalhost CLI release first.
  - name: Read monitoring inventory
    env:
      METALHOST_ENDPOINT: https://api.metalhost.net
    run: >-
      metalhost auth github
      --trust github-trusts/APPROVED_ID
      --audience YOUR_APPROVED_AUDIENCE
      -- metalhost monitoring vms --format json
```

The command obtains a fresh GitHub assertion, exchanges it for a 15-minute
credential, masks both in GitHub logs, and supplies the credential only to the
child process environment. It neither reads nor writes a CLI profile and never
falls back to a saved key. The child receives the approved project and endpoint;
inherited key/project/profile values and GitHub token-request credentials are
removed. Do not print environment variables in the child or upload them as
artifacts. No refresh token is issued; run a fresh exchange for a later command.

Only GitHub.com Actions OIDC request URLs are supported, using HTTPS with no
redirects. Enterprise Server's custom OIDC issuer is not supported by this release.
See [GitHub's OIDC request documentation](https://docs.github.com/en/actions/reference/security/oidc).
