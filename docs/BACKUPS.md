# Backups (CLI v1.1.0)

Requires backend v1.0.67 or later. Use fully qualified resource names and select
the project with `--project` or your profile. Backups are DC-local disk copies,
not off-site disaster recovery or RAM checkpoints.

## Single-disk backups

```sh
metalhost disk backup create projects/p/disks/data --display-name before-upgrade --idempotency-key capture-1
metalhost disk backup list --project projects/p --all
metalhost disk backup get disk-snapshots/ID
metalhost disk create --project projects/p --from-backup disk-snapshots/ID --id restored-data --size-gib 100 --region datacenters/us-dal-1 --network projects/p/networks/default
```

Wait for the backup to be READY before restoring. The new disk must match the
backup's project, datacenter, network and storage tier, and be at least as large.
Poll `disk get` until AVAILABLE. Reuse the same `--id` when retrying a restore;
the CLI requires it with `--from-backup`.

`disk backup delete NAME` prompts before permanently deleting a backup;
`--yes` skips confirmation. `snapshot` remains an alias for `backup`.

## Whole-VM backups

```sh
metalhost vm backup create --vm virtual-machines/ID --display-name before-upgrade --idempotency-key vm-capture-1
metalhost vm backup get vm-snapshots/ID -o json
metalhost vm from-backup --snapshot vm-snapshots/ID --display-name restored --idempotency-key vm-restore-1 --wait
```

Wait for READY and inspect the captured volume list and consistency. Restore
creates a new VM and every captured disk; the source is unchanged. New resources
have separate billing and quotas. Monthly reservations and IP identities are not
copied. Reuse a capture/restore key only with the same request when retrying an
ambiguous error. `vm snapshot` remains supported.

## Automatic schedules

```sh
metalhost storage backup-schedule create --project projects/p --target virtual-machines/ID --cadence daily --hour-utc 3 --keep 7 --idempotency-key policy-1
metalhost storage backup-schedule list --project projects/p --all
metalhost storage backup-schedule update snapshot-schedules/ID --enabled=false
metalhost storage backup-schedule update snapshot-schedules/ID --keep 3
metalhost storage backup-schedule delete snapshot-schedules/ID
```

Omit `--target` to cover the selected project: every VM and each unattached disk.
Schedules are enabled by default. Use `--cadence hourly|daily|weekly`,
`--minute-utc 0..59`, `--hour-utc 0..23`, and `--weekday-utc 0..6` (Sunday=0).
These are fixed UTC times; browser-local dashboard times can shift with daylight
saving time. Updates change only explicitly supplied fields.

Keep-N retention applies per source and policy, only after a new successful copy.
Manual backups are never pruned. Reserve quota headroom for that next copy.
Pausing prevents new captures; deleting a schedule keeps its existing backups.
