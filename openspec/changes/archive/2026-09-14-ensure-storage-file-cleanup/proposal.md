## Why

Logical and physical backup failures can leave partial or fully uploaded files behind after the catalog marks the backup failed or removes its row. Direct best-effort `DeleteFile` calls cannot survive a storage outage, a process restart or a database failure between uploading a file and publishing its catalog state.

## What Changes

- Add one storage-file lifecycle boundary that accepts a storage ID and a caller-chosen, unique file name. Backup code keeps ownership of file names and relationships; storage code owns provider selection and the cleanup obligation for one file.
- Record a pending deletion for every file before the first byte reaches the provider, and let the PostgreSQL transaction that publishes the backup remove that record. A file is kept only because a committed catalog transaction claimed it, so every other outcome leaves the obligation in place.
- Drain those obligations in one background worker with capped exponential backoff, persisted sanitized errors and a per-pass summary log.
- Require every storage provider to remove the complete physical representation derived from a logical file name, including partial files, temporary files, chunk objects, manifests, uncommitted blocks and incomplete multipart uploads.
- Give every object name a component unique to the write attempt that produced it. WAL and timeline history keys are currently derived from database, timeline and segment identity alone, and physical FULL and INCR keys are constant across the codec fallback that retries one backup through zstd, gzip and none, so in both cases a retry addresses the same object a pending cleanup is about to delete. Existing stored objects keep their names, which the catalog rows already carry.
- Migrate logical backups, physical FULL and INCR backups, WAL and history uploads, sidecars, manifests, retention cleanup, cancellation and restart recovery to the new write and deletion contract.
- Stop publishing a logical backup whose encryption metadata could not be written. Today that failure is logged and the backup is marked completed, which leaves a backup nobody can restore.
- Add regression coverage for returned errors, process interruption, catalog commit failure, repeated requests, storage outages and confirmed eventual cleanup. MinIO tests cover the S3 representations that cannot be observed through a single object lookup.
- Attempt a storage's outstanding cleanup when that storage is deleted, then record in the audit log every file the attempt could not remove. Outstanding cleanup never makes a storage or a workspace undeletable.
- Rework the existing tests that assert synchronous object removal or derive a WAL object name from its segment name, so the suite proves eventual cleanup instead of breaking on it.
- **BREAKING**: internal backup write and deletion callers stop receiving a raw `*storages.Storage` for file I/O and use the new file store instead. No compatibility adapter remains in backup packages.
- **BREAKING**: deleting a storage configuration is rejected while backup rows still reference it, with one actionable conflict for both of the shapes this replaces. Logical backup rows currently cascade away and strand their objects forever, while physical full, incremental, WAL and history rows have no delete rule at all and already fail with a raw database constraint error. Either way the backups have to be removed through their own deletion path first.
- **BREAKING**: a logical backup whose encryption metadata cannot be written or validated now fails instead of completing. The sidecar was best effort before, so a backup could be published with no way to decrypt it.
- **BREAKING**: provider deletion stops reporting success for an ambiguous result. FTP, SFTP and NAS return connection, permission and unknown stat errors instead of treating them as absence, and local deletion also removes the staging path. The `--test-storage` probe at `backend/cmd/storage_command.go:80` keeps calling the provider directly, so it starts failing on a lookup it used to pass.
- **BREAKING**: the storage attachment-count registration contract changes shape. `StorageDatabaseCounter` gains a second method reporting backup references, so it, its registrar, the service field and the test fixture are renamed to say what they now report. Both registered implementations move with it.

## Capabilities

### New Capabilities

- `storage-file-cleanup`: Defines pending deletion records, transactional publication, eventual deletion, retry behavior, provider cleanup guarantees and integration with logical and physical backup failures.

### Modified Capabilities

- `local-storage-file-lifecycle`: Extends idempotent local deletion to remove both published files and staging files left by interrupted writes.

## Impact

The backend gains a `backend/internal/features/storages/files` package, a PostgreSQL table for pending deletions and one background worker started by `backend/cmd/main.go`. Storage provider implementations under `backend/internal/features/storages/models` receive a stronger deletion contract. Logical and physical backup use cases, backupers, cleaners, schedulers, database deletion listeners and WAL upload code move their write and deletion paths to the new boundary. `backend/cmd/storage_command.go` keeps calling provider methods directly, because the `--test-storage` startup probe owns its own file and has no catalog state to publish.

Deleting a storage first attempts its outstanding cleanup and then writes one audit-log entry for whatever it could not remove. The pending deletion row belongs to its storage row and disappears with it, so the storage and workspace teardown fixtures keep working unchanged; the database fixture moves onto the new listener contract with production deletion. Physical and WAL tests have to read object names from catalog rows rather than rebuilding them, and object-absence assertions have to run the deletion worker to completion first.

The change adds schema and internal Go API changes but no new external dependency. Existing completed backup rows and their stored object names remain readable. The work answers to `AGENTS.md` and `backend/AGENTS.md`.

## Out of scope

- Keeping a permanent inventory of every successfully stored file.
- Retrying failed uploads; the existing backup and WAL schedulers retain that policy.
- Moving download and restore reads to the new package.
- A UI or public API for inspecting and manually retrying file operations.
- Discovering arbitrary orphaned objects that have no Databasus file name or pending deletion record.
- Backfilling cleanup for backup rows that already failed before this change. Their files are removed when the backup or its database is deleted, which routes through the new contract.
- Proving storage target identity, or restricting which storage configuration fields may change while cleanup is pending. Every write attempt produces a name no other attempt uses, so a repointed storage leaves its old files in the old location rather than deleting the wrong ones.
- Verifying cleanup permissions during a storage connection check. A missing permission surfaces as a recorded deletion failure instead.
- Changing how workspace deletion already refuses while the workspace still contains databases.
- Changing retention selection, recovery-chain rules or point-in-time restore semantics.
