## Context

See `proposal.md` for the user-visible failure. File I/O currently starts from methods on `Storage` (`backend/internal/features/storages/model.go:40` and `:68`), and `StorageService.GetStorageByID` exposes that model to every caller (`backend/internal/features/storages/service.go:331`). This leaves lifecycle policy spread across backup packages.

The logical path starts a storage writer inside each database-specific use case. A producer can write a valid storage object and still return a dump error later (`backend/internal/features/backups/backups/usecases/logical/postgresql/create_backup_uc.go:199`). `Backuper.MakeBackup` deletes the main file only in its cancellation branch (`backend/internal/features/backups/backups/backuping/logical/backuper.go:157`) and does not delete it for the ordinary failure branch at `:183`. Retention logs storage deletion errors and removes the catalog row anyway (`backend/internal/features/backups/backups/backuping/logical/cleaner.go:88`).

The physical stream has the same split outcome: storage writes in a goroutine (`backend/internal/features/backups/backups/usecases/physical/postgresql/stream.go:148`), while the producer result is classified later. FULL and INCR names are persisted before upload (`backend/internal/features/backups/backups/usecases/physical/postgresql/create_full_uc.go:40`), but restart recovery still contains the old assumption that `file_name` is NULL until completion (`backend/internal/features/backups/backups/backuping/physical/scheduler.go:710`). Physical cascade cleanup contains both fail-open and fail-closed direct deletion policies (`backend/internal/features/backups/backups/core/physical/service/service.go:1056` and `:1073`), and both run inside the cascade transaction that opens at `:613`, so a rollback restores rows whose objects are already gone.

Provider cleanup is not yet a uniform logical-file operation. S3 can leave chunk objects without a manifest when its best-effort abandon path fails (`backend/internal/features/storages/models/s3/model.go:535`), while its later deletion discovers chunks only through that manifest (`backend/internal/features/storages/models/s3/model.go:273`). Local storage writes to the temporary folder first (`backend/internal/features/storages/models/local/model.go:53`) but deletion checks only the data folder (`backend/internal/features/storages/models/local/model.go:163`). FTP, SFTP and NAS collapse broad stat failures into successful absence (`ftp/model.go:185`, `sftp/model.go:197`, `nas/model.go:238`).

Two existing guards bound how far storage lifecycle work has to reach. Workspace deletion already refuses while the workspace still contains databases (`backend/internal/features/databases/service.go:662`, reached through the listener loop at `backend/internal/features/workspaces/services/workspace_service.go:182`), so a workspace cannot be removed out from under live backup rows. Storage deletion already refuses while databases are attached to it (`backend/internal/features/storages/service.go:192`). What is missing is the case where a storage keeps historical backup rows with no attached database, and the two artifact families behave differently there. `backups.storage_id` is `ON DELETE CASCADE` (`backend/migrations/20260127132617_add_cascade_delete_for_backups_and_databases.sql:8`), so deleting that storage drops the logical rows and strands every object they name. The physical tables declare no delete rule at all (`backend/migrations/20260523120000_add_physical_pg_17_backups.sql:132`, `:168`, `:212`, `:287` and `:317`), so the same deletion already fails with a raw constraint error instead of an actionable one.

One platform guarantee shapes every coordination decision below. The `single-instance-runtime` capability specifies exactly one application process for each installation, and background `Run` methods already panic on a second in-process call. Writers and the cleanup worker therefore share one address space, which is why an in-memory set is a sound way to tell a live write from a dead one.

The design follows `AGENTS.md` and `backend/AGENTS.md`. In particular, the storage feature keeps its repository instance private and exposes no repository getter (`backend/internal/features/storages/di.go:12`), background `Run` methods reject a second call, files have one responsibility, DI uses positional fields, comments explain hidden constraints only and tests assert observable cleanup rather than log text.

## Goals / Non-Goals

**Goals:**

- Close the crash window before upload and the commit window after upload.
- Give backup code one transactional way to publish or discard every file it owns.
- Make a logical file deletion complete and idempotent for every provider.
- Prevent stale cleanup from deleting a published file or a later write.
- Keep the existing test suite meaningful once deletion stops being synchronous.

**Non-Goals:**

- Treating storage as the owner of backup file hierarchies or retention decisions.
- Keeping successful file rows after their catalog transaction commits.
- Retrying uploads inside the storage file layer.
- Refactoring the read path in this change.
- Listing and deleting arbitrary provider objects that no Databasus record can identify.
- Proving that a provider location is unchanged, whether by contacting the provider or by restricting configuration edits.
- Adding a coordination protocol for several concurrent cleanup processes, which the deployment model does not produce.

## Decisions

### Invert the default: a file is kept only when a transaction claims it

The whole change rests on one invariant. **A pending deletion row exists for a file from before its first byte is written until a committed catalog transaction takes responsibility for it.** Every failure mode is then the same failure mode: the row stays, and the worker eventually removes the file. There is no separate reasoning for a provider error, a crash, a rolled back catalog transaction or a canceled backup, because none of them removes the row.

This inverts today's default, where a file is kept unless some code path remembers to delete it, and the missing paths are the bug.

Alternatives rejected:

- Keep the file name in the catalog and add a deletion-only queue fed by terminal transitions. Three artifact types already commit their object name before upload (`backuping/logical/scheduler.go:158`, `create_full_uc.go:40`, `create_incremental_uc.go:76`), so this is tempting. It fails on the crash window: a process that stops between the upload and any terminal transition leaves an object that no row and no queue entry names. It also spreads recovery reasoning across one sweep per artifact type, which is where the current bug already lives, at `scheduler.go:710`.
- A deletion-only queue created after `SaveFile` returns. A process can stop before the insert.
- A permanent file registry. Backup tables already own successful file relationships, and copying them would create two authorities for retention and restore.

### Put the write and deletion boundary in `storages/files`

Create `backend/internal/features/storages/files` with `Store`, `StoredFileReference`, `WriteReceipt`, `PendingDeletionRepository` and `DeletionWorker`. `Store.WriteFile` accepts a `StoredFileReference` carrying a storage ID and a file name, and backup code still creates the exact name. `ConfirmFileWrites` and `RequestFileDeletions` accept batches because a backup result commonly owns a base file and sidecars.

The directory is `files` and the package is `storage_files`, matching how the repository already aliases provider packages (`local` declares `package local_storage`). The bare name `files` is taken by `backend/internal/util/files`, which declares `package files_utils`. `Store` names what the type owns, which is the state of pending deletions, and `StoredFileReference` names the pair it carries rather than the fact that it points at something. `Store` reads as a whole at its call sites, `storage_files.Store`, and it names a thing that holds state rather than an unnamed activity, which is what `AGENTS.md:126` rules out in `Manager` and `Handler`. A name built around the obligation, such as `CleanupObligations`, would describe one of its three methods and misname `WriteFile`.

The child package declares narrow `FileWriter`, `FileRemover` and `StorageLocator` interfaces and does not import its parent `storages` package. Splitting write from removal keeps each name honest: `Store` uses the writer, `DeletionWorker` uses the remover and neither name hides a second effect. The parent package implements `StorageLocator`, wires `Store` and exposes its getter, which avoids a Go import cycle and leaves provider selection in the feature that owns storage configuration.

Only write and deletion callers move in this change. Reads keep using the existing path because they do not participate in publication or cleanup. `backend/cmd/storage_command.go` is the one production exception: the `--test-storage` startup probe writes and removes its own file before the application exists, so it has no catalog state to publish and no row to own.

Alternatives rejected:

- Add all methods to the current `StorageService`. It already owns configuration CRUD, authorization, audit, attachment counting, testing and transfer, and file lifecycle would mix a control plane with a data path.
- Move reads at the same time. That adds restore and streaming risk without strengthening cleanup.
- Name the type `FileManager`. `Manager` names nothing, which `AGENTS.md:126` rules out.

### Use one pending deletion table

Add `storage_pending_deletions` with these columns:

| Column | Purpose |
| --- | --- |
| `id` | UUID row and receipt identity |
| `storage_id`, `file_name` | Logical file reference, unique as a pair |
| `not_before` | Earliest time the worker may act on this file |
| `generation` | The value a write receipt matches, raised by anything that takes the file from its writer |
| `attempt_count` | Deletion attempts, which is what the retry backoff grows with |
| `last_error` | Sanitized bounded failure text |
| `created_at`, `updated_at` | Ordering and operational diagnosis |

The table has a unique constraint on `(storage_id, file_name)`, a foreign key to `storages` with `ON DELETE CASCADE` and one index on `not_before`. It holds no state column, no lease token, no backup ID, no owner type and no provider configuration or credentials. Row presence is the state.

`ON DELETE CASCADE` is deliberate. The alternative, a restrictive foreign key, turns every teardown fixture and every workspace cascade into a raw database error and buys nothing: the storage row is what carries the credentials, so once it is gone the obligation is not executable anyway. Storage deletion instead drains what it can and records what it could not, which is covered below.

`generation` and `attempt_count` move separately on purpose. A deletion request has to raise the generation, because that is what lets a write receipt detect that cleanup, or another caller, took the file away from its writer. It is not a deletion attempt, though, and letting it drive the backoff would start the first real attempt against a file that was requested a few times already deep in the retry curve.

Alternatives rejected:

- Separate lifecycle states for writing, awaiting commit and pending deletion. Each state needs its own expiry, its own conversion pass and its own test matrix, and all three answer the same question that row presence already answers.
- Separate write-intent and deletion tables. Their transition would need another transaction and duplicate indexes without adding a guarantee.
- `ON DELETE RESTRICT` with a service-level refusal in front of it. It makes an unreachable provider location permanently block deletion of the storage and its workspace, which then needs an abandonment timer, an audit trail for abandonment and fixture changes in three shared teardown helpers.

### Register before writing, confirm inside the publishing transaction

`Store.WriteFile` generates the row ID, joins an in-process set of live writes, inserts the pending deletion row with `not_before = now() + commitWindow`, then calls the provider. Joining before the insert and starting the row ineligible close the same gap from two sides: a row the worker can see belongs to a writer that already joined, and a freshly inserted row is not due anyway. When the provider returns:

- on success it refreshes `not_before` to `now() + commitWindow` conditionally on `generation` still holding the value it started with, leaves the live set and returns a `WriteReceipt` carrying the row ID and that `generation`;
- on failure it sets `not_before = now()` under the same condition and only then leaves the live set, so the file becomes eligible exactly when its writer is gone;
- if either update matches no row, someone requested deletion of this file while the write was running, so it returns an error instead of a receipt and the caller never tries to publish.

The insert takes no action on conflict, and `WriteFile` then fails without sending a byte, reporting that the name is already registered. Two things produce that conflict. A cascading removal can request deletion of a file whose write has not started, which is a legitimate race and the reason the write must not proceed; if the worker drains that row first the later write simply starts a fresh obligation, which is equally safe. A second concurrent write of the same name also collides, and per-attempt naming makes that a defect rather than a case to merge, so the error says what the writer can observe rather than guessing which cause it was.

The caller then does one of two things:

- `ConfirmFileWrites(ctx, tx, receipts)` inside the transaction that makes the backup, WAL, history, manifest or metadata catalog state usable. It deletes each row by `id` conditionally on the receipt's `generation` and fails the transaction unless every receipt matched exactly one row;
- `RequestFileDeletions(ctx, tx, references)` inside the transaction that records failure, cancellation, retention removal or user deletion.

`commitWindow` is the only deadline in the design. It answers a single question: how long may a file sit recorded but unclaimed by any catalog transaction before cleanup assumes its publisher is never coming. Publication normally follows the upload within the same function, so the window is generous.

There is no heartbeat and no lease renewal. A long physical backup does not need one, because the live-write set already tells the worker that this file has a writer, and the set is empty for every write inherited from a process that stopped. Removing the heartbeat also removes the failure mode where a metadata database blip cancels a multi-hour backup.

Each independent attempt gets an immutable name, and attempt means one call to a provider, not one backup. Logical naming already satisfies that: `GenerateFilename` runs once per backup at scheduling (`backend/internal/features/backups/backups/backuping/logical/scheduler.go:158`) and nothing retries the upload under that name. Three physical paths do not.

FULL and INCR reuse one name across the codec fallback. `streamWithCodecFallback` retries the same backup through zstd, gzip and none with one `fileName` (`backend/internal/features/backups/backups/usecases/physical/postgresql/executor.go:50`), so a server that rejects zstd produces two or three writes of the same name, and its doc comment says so outright at `:45`. Under the rule above the second write would be refused and the downgrade path would stop working. Calling `buildObjectName` (`.../executor.go:152`) once per attempt does not help, because its four inputs are constant across the loop. It gains an attempt UUID instead, which also covers a same-codec retry that a codec suffix would not. Two comments and one test state the old shape and go with it: `executor.go:45`, the `buildObjectName` comment about uniqueness coming from the trailing backup ID, and the `expected` table field in `executor_test.go`.

The loop cannot mint that name by itself. It holds neither the naming inputs nor the repository that persists `file_name`, and its parameter list is already eight positional values, so it takes a per-attempt `mintAndSaveAttemptName` callback from its caller, named for both of its effects because it produces the name and persists it on the row, and the stream inputs move to a named struct rather than growing that list. The two callers own the repositories the callback needs (`create_full_uc.go:40` and `create_incremental_uc.go:76`, each followed by its `Save`). One use of `fileName` deliberately does not follow the attempt: `newPgBasebackupCommand` takes it as the pg_basebackup label (`pg_basebackup.go:50`), which stays stable across attempts because it names the backup, not the object.

Everything downstream then reads the settled name from the stream result rather than from the caller's variable. `streamWithCodecFallback` returns it, `runStreamParams` carries it into the stream (`.../executor.go:79`) so the upload-time manifest derived at `stream.go:317` follows it, and the remaining consumers move with them: the chain-broken removal and the reported `FileName` (`create_full_uc.go:84` and `:92`), `streamResult.FileName` (`:113`), the incremental equivalent (`create_incremental_uc.go:116`) and the metadata sidecar (`metadata.go:123`).

WAL and history rebuild their keys from identity. `walSegmentObjectName` derives from database, timeline and segment name (`.../wal_upload.go:439`) and is called twice per upload, once for the artifact at `:232` and once for the sidecar at `:380`. The history key derives from database and timeline (`.../timeline.go:231`), and its sidecar appends `.metadata` to that key at `:246`. Both gain a UUID minted once per upload attempt, and both sidecars derive from the minted name rather than calling the generator again, or the artifact and its sidecar would carry different UUIDs and every consumer that reads `file_name` off the row and appends `.metadata` would miss. History already mints `historyFileID` one line above its key (`timeline.go:230`), so it has the value it needs. Restore already reads these names from catalog rows rather than rebuilding them, so only tests that rebuild the key are affected.

Backup packages also stop reaching providers directly, and a test enforces it rather than a convention. It walks the non-test files under `backend/internal/features/backups` for calls to `SaveFile` and `DeleteFile` on a provider value. That scope keeps out the storages feature's own methods, its provider tests, the `--test-storage` probe and the backup test fixtures that legitimately create and remove provider objects to set up a case. The rule is worth a test because the old call shape still compiles everywhere and reintroducing it would silently reopen the leak.

Alternatives rejected:

- Unexport the provider write and delete methods. Provider implementations live in child packages, so the methods have to stay exported for the storages feature itself.
- Add a custom linter rule. It would live outside the test command every contributor already runs, and the check is one package walk.
- Treat provider success as publication. This leaves a file unowned when the following catalog write fails.
- Let `Store` update backup tables. That would teach storage about logical, FULL, INCR, WAL, history, manifest and metadata relationships.
- Reuse object names across attempts. A pending deletion for the previous attempt would remove the new bytes.

### Let the worker skip live writes instead of leasing them

`DeletionWorker` holds the live-write mutex across its claim statement and passes the set into it: `UPDATE ... SET generation = generation + 1, attempt_count = attempt_count + 1, not_before = now() + attemptLease WHERE id IN (SELECT id FROM storage_pending_deletions WHERE not_before <= now() AND id <> ALL(<live writes>) ORDER BY not_before LIMIT <batch> FOR UPDATE SKIP LOCKED) RETURNING *`. It commits the claim, then deletes remotely without holding a database lock. Both completion and rescheduling are conditional on the claimed `generation`: success deletes the row, failure records a sanitized error and a jittered exponential `not_before` capped at a package maximum and grown from `attempt_count`.

`SKIP LOCKED` matters because `RequestFileDeletions` runs inside a caller's catalog transaction, and task 8.4 makes database removal one transaction that records every FULL, INCR, WAL, history, manifest and metadata reference it owns. Without it, one such transaction would stall the whole claim batch rather than the rows it holds. Holding the mutex across that statement, rather than snapshotting the set before it, is what makes the exclusion sound. A writer joins the set before it inserts, so a writer that joins after the claim has not inserted its row yet and a writer that joined before it is in the exclusion list. Snapshotting first leaves a gap in between, and the gap is reachable: `RequestFileDeletions` can pull a live writer's row forward to `now()`, which a cascade does to a FULL or INCR file whose name the catalog already carries (`create_full_uc.go:40`). The mutex is safe to hold here because `FOR UPDATE SKIP LOCKED` means the statement never waits on a row lock, and no writer holds it across an insert.

Two properties make this enough, and both follow from one process:

- a row absent from the live set has no writer in this address space, and no other address space exists, so nothing is going to publish it;
- the claiming update is a compare and swap on `generation`, so a receipt issued before the claim can no longer confirm, and a slow attempt that outlived its `attemptLease` can neither reschedule nor complete over a newer one.

`attemptLease` is how long a claimed row belongs to the pass that took it, not a bound on the provider call. Every provider gives its own deadline to a deletion and takes the passed context only for log correlation, so the lease has to stay longer than the slowest of those deadlines: a lease that runs out first lets the next pass claim a row whose attempt is still running, and the attempt that finishes second can no longer release it.

The live set is a mutex-guarded map of row IDs written by `Store.WriteFile` and read by the worker inside the same process. A writer joins before its insert and leaves only after its own database update has committed, so the worker can never observe a row as both eligible and unowned while its writer is still deciding the outcome.

The worst case this accepts is a provider write that hangs without honoring cancellation. Its row stays in the live set and its file is not cleaned until the call returns or the process stops. Nothing is lost and nothing is deleted early, so it needs no bounded-stop contract and no conformance suite across eight providers.

`Run` uses `atomic.Bool.Swap(true)` and panics on a second call, matching `backend/internal/features/backups/backups/download/download_token/background.go:29`. It scans immediately at startup, then on a ticker and exits on context cancellation. Every pass uses the stable `job_name` `storage_file_deletion` and a fresh `job_id`; attempt logs also retain storage ID, file name, attempt count and next retry time after centralized redaction. Each pass ends with one summary line carrying how many obligations are pending, how many are overdue and the age of the oldest one, because the change adds no interface for reading this table. The age is what separates a queue that is draining from one obligation that will never succeed, which two counts alone cannot show once a capped retry makes every stuck row look briefly overdue. The pass is one exported method that `Run` calls at startup and on every tick, so a test drives cleanup as a sequence rather than sleeping on the ticker. A `DrainForTest` helper in the package's `testing.go` runs passes until the files a test names carry no obligation, following the seam used by `backend/internal/features/backups/backups/backuping/physical/testing.go:221`.

Alternatives rejected:

- Lease tokens with expiry and a termination grace. They reconstruct, across two extra columns and a conversion pass, the fact that the live set already knows.
- Delete the row first and re-insert it if the provider call fails. It makes the claim trivially safe and reintroduces the exact bug this change exists to fix, because a crash between the delete and the re-insert loses the obligation.
- Hold `FOR UPDATE` during remote deletion. A storage outage would occupy database locks for the full network timeout.
- Report queue depth through a metrics endpoint or an API. A summary log line reaches the same operator through the sink they already have.

### Keep catalog transactions in the owning backup service

The backup service owns its PostgreSQL transaction and passes its `*gorm.DB` to the two batch methods. The storage repository remains private behind `Store`; backup packages never receive it. The deletion-request insert and the receipt confirmation use the caller transaction, while remote provider calls always run after commit in the background worker. The registration insert inside `WriteFile` is the one that must not: it commits on its own connection before the provider call, because a caller transaction that later rolled back would take the obligation with it.

Database deletion currently invokes listeners and then performs a separate cascading delete. This change makes database removal transactional and changes its internal listener contract to receive the same `*gorm.DB`. The backup listener enumerates logical, FULL, INCR, WAL, history, manifest and metadata references and requests their deletion before the database row is removed. Cancellation and replication-slot cleanup dial the source database, so that work runs to completion before the transaction opens rather than inside it. Otherwise one catalog transaction would stay open across an SSH or PostgreSQL dial timeout. Inside the transaction there are only catalog writes and deletion records, and catalog removal cannot commit unless every record is durable.

That transaction still holds uncommitted unique-index entries on `(storage_id, file_name)` for every reference it recorded. A `WriteFile` for one of those names blocks until it commits, rather than failing fast, which is the WAL upload racing a database deletion. `SKIP LOCKED` covers the worker, not the writer. The wait is bounded by the transaction, which is why the external listener work is outside it, and blocking is the correct outcome anyway: the write cannot be allowed to publish a file the cascade is removing.

This is a deliberate GORM seam. It is smaller than a general unit-of-work abstraction and matches existing physical services that already coordinate several repositories in one transaction. Method names include `FileWrites` or `FileDeletions`, and their parameters use named structs rather than long positional lists.

Alternatives rejected:

- Record deletions before catalog deletion in a separate transaction. A worker could remove a file while a failed catalog transaction leaves the backup visible.
- Record them after catalog deletion. A process stop between commits loses the only durable file reference.
- Give the repository to backup packages. That violates the repository ownership rule and lets callers bypass the contract.

### Accept a deletion request at any point in the write lifecycle

`RequestFileDeletions` never fails because a writer is still running and never fails because the same reference was requested before. It inserts the row with `not_before = now()` when it is missing, because a file nobody is writing has no writer to wait for, and against an existing row it sets `not_before = now()` and raises `generation`. That raise is what makes the request safe against a live writer: the writer's own completion update and its receipt both carry the earlier value, so the writer cannot extend the window and its caller cannot publish the file.

The alternative shape, where the method rejects a request against a live writer, breaks a caller that already exists. `DeleteFull` cascade deletion runs while WAL archiving is active and is documented to catch a claim mid-upload (`.../wal_upload.go:275`). A rejection there would roll back the whole cascade transaction every time a segment happens to be uploading.

Alternatives rejected:

- Return an error and require the caller to cancel and join the writer first. Cascade deletion has no handle on a WAL writer in another goroutine, and the rollback would be indistinguishable from a real failure.
- Delete the row and let the writer recreate it. The writer would then publish an object nobody is tracking.

### Make `DeleteFile` mean every provider representation

The provider contract remains one logical `fileName`, but every implementation must remove its own derived physical state:

- local removes matching temporary and published paths, including a partial destination from cross-filesystem publication;
- S3 removes the base object, manifest, exact numbered chunk objects and matching incomplete multipart uploads even when no manifest exists;
- Azure removes the committed blob and any uncommitted blocks for its exact blob name. There is no API that drops staged blocks, so cleanup commits an empty block list first, which turns them into a zero-length blob the delete then removes. That step runs only when the blob does not exist, because a blob written with one Put Blob carries no committed blocks and an empty commit over it would truncate real content. Azurite does not reproduce this, since its delete already succeeds on a blob that holds only uncommitted blocks;
- FTP, SFTP and NAS drop the stat probe that turned any lookup error into a successful deletion, and classify the removal reply instead. FTP has one irreducible gap: 550 is the only reply for a missing file and also the reply for a refused one, so it counts as absence;
- Google Drive removes every matching incomplete or completed file created for the attempt, and reads a missing backups folder as absence rather than as a failure to retry forever;
- rclone removes the exact remote object and reports lookup errors other than confirmed absence.

S3 discovery matches the exact derived grammar rather than a bare prefix, so deleting `backup-1` cannot touch `backup-10`. Provider cleanup may retain its immediate best-effort rollback, but an ignored rollback result never replaces the durable row.

The WAL retention capability needs no delta. Its requirement that each deletion transaction rechecks the FULL claim before removing a segment governs which segments may be chosen, and that recheck still happens inside the transaction that decides. Only the remote object removal moves after the commit.

The connection check is unchanged. A credential that can write but not delete now fails at cleanup time, where the worker records the provider error on the row, keeps retrying and names the storage and file in its attempt log. Checking those permissions up front would reject storage configurations that work today, and it would rewrite `TestConnection` across every provider, call site and test double for a diagnosis that the deletion log already gives.

Alternatives rejected:

- Persist provider-specific part names in the table. That leaks representation details above providers and cannot describe incomplete multipart uploads after a hard stop.
- Delete only the base object when no manifest exists. The current custom S3 layout can complete numbered objects before publishing its manifest.
- Treat every stat error as not found. It lets the worker acknowledge deletion during an outage or permission failure.

### Let storage deletion drain, then report what is left

Storage deletion refuses while backup rows still reference the storage, which is the one question that has to leave the feature. It is answered through the existing registration mechanism at `backend/internal/features/storages/interfaces.go:48`, extended with a second method `GetStorageBackupReferences` rather than duplicated as a parallel registry, and implemented by the backup services that already register there. A counter that also reports backup references no longer matches the name `StorageDatabaseCounter`, so the interface, its registrar, the service field `storageDatabaseCounters` (`backend/internal/features/storages/service.go:22`) and the fixture `SetStorageDatabaseCountersForTest` (`backend/internal/features/storages/testing.go:13`) are renamed together.

Pending deletions do not block anything. When a storage with no referencing backups is deleted, the service runs one drain pass over that storage's rows, bounded by both a row count and a wall-clock budget, then deletes the storage row, which cascades the rest away. The budget matters because the drain runs in the request path and the workspace listener repeats it for every storage in the workspace, so without it an unreachable provider would hold the user for the batch size times the provider timeout. The budget is read between files rather than inside one, so the worst case it buys is the budget plus one provider timeout. The drain claims through the same path as the worker, so it excludes live writes for the same reason and leaves any file still being written to the cascade and the audit entry. Every pending deletion row still present for that storage after the drain goes into one audit-log entry through `AuditLogService.WriteAuditLog` (`backend/internal/features/audit_logs/service.go:24`) naming the storage, how many files are left behind and the first of their names up to a fixed limit, plus the same at warning level. The count tells the user the size of what was left, and the names give them something to act on, without putting thousands of lines into one audit entry. Reporting on what remains rather than on what the drain failed at also covers the rows the drain never saw, which are the ones the worker had already claimed and the ones its skip-locked claim passed over. Workspace deletion reaches storages through the listener at `backend/internal/features/storages/service.go:73` and takes the same path. `OnBeforeWorkspaceDeletion(workspaceID uuid.UUID) error` (`backend/internal/features/workspaces/interfaces/interfaces.go:6`) carries no context, and the drain gets a background one. Threading a context through three implementations buys one log field on a path that already outlives the request that started it.

The user-visible outcome matches what a retry-until-abandoned design would eventually produce, without making the user wait out an abandonment age for it, and without the fourth lifecycle state and the restrictive foreign key that design needs.

Alternatives rejected:

- Retry until a bounded age, then abandon. It leaves the storage undeletable for the length of that age and needs a fourth lifecycle state to end.
- Delete the storage silently. The files are still in the user's storage, so the record of what was left has to survive somewhere the user can read.
- Let storage deletion cascade the backup rows as it does today. Those rows are the only record of the object names, so the objects become unreachable at the moment the rows disappear.

### Keep the existing suite honest once deletion is asynchronous

Two groups of existing tests encode assumptions this change breaks.

Object-absence assertions that run right after a catalog operation now have to drive the worker to completion before asserting. `Test_DeleteFull_WithDependents_CascadesEntireChainAndObjects` in `backend/internal/features/backups/backups/core/physical/service/service_test.go` is one. `Test_WalUpload_DeleteCascadesIntoNullClaim_UploadCompletes_DeleteFileCleansOrphan` (`.../wal_upload_test.go:207`) is the sharper one, because it asserts at `:239` that the orphan is already gone, which is exactly the synchronous `deleteObject` call at `wal_upload.go:282` that this change replaces. Both get the single-pass seam, so they assert sequence rather than elapsed time.

Tests that rebuild a WAL object key from its segment name break once the name carries a per-attempt UUID: `wal_upload_test.go:74`, `wal_upload_test.go:212`, `wal_watch_dir_test.go:51`, `wal_watch_dir_test.go:71` and `wal_stream_test.go:444`. They read the name from the catalog row instead, which is what production restore already does.

The storage and workspace teardown fixtures need no change. `storages.RemoveTestStorage` and `workspaces_testing.RemoveTestWorkspaceDirect` delete rows that cascade into `storage_pending_deletions`, and `workspaces_testing.RemoveTestWorkspace` deletes through the API, which meets no new refusal. Only the storage-deletion controller tests move, because of the new backup-reference conflict. The database fixture does change: task 8.5 moves `DatabaseService.DeleteForTest` onto the transactional listener contract, because it runs the same chain as production deletion and would otherwise stop recording the references that chain owes.

Alternatives rejected:

- Rework these tests at the end of the change. Their assumptions are what tells the implementation whether the contract is right, so discovering them late would make each one look like a regression to explain away.
- Give tests a synchronous store double that deletes inline. It would leave the production ordering untested exactly where the current bug lives.

## Risks / Trade-offs

- [A hung provider write blocks cleanup of its own file] -> The row stays in the live set until the call returns or the process stops, so the file is cleaned late rather than early. Nothing is deleted under a live writer, which is the property worth protecting.
- [Two numbers move on the same row] -> `generation` is the compare-and-swap value every receipt matches and `attempt_count` is the retry counter. Folding them into one column was the first shape, and it made every deletion request look like a failed attempt, so a file requested a few times began its first real attempt near the retry cap.
- [A published backup could lose its file if publication is slower than `commitWindow`] -> Confirmation is conditional on the receipt generation, so the transaction fails instead of publishing a file that cleanup already claimed. The backup is then failed rather than silently broken.
- [Storage deletion now refuses while backup rows reference the storage] -> The refusal is actionable and the path out is the backup deletion flow this change makes reliable. The previous behavior removed the rows and made the objects unreachable forever.
- [An obligation against a permanently broken target retries forever] -> Nothing bounds the number of attempts, so a file whose provider never accepts a deletion is retried once per capped retry delay for as long as its storage exists. No storage, workspace or backup operation waits on it, and the summary's oldest-obligation age keeps it visible. What it does cost is queue position: the claim orders by `not_before`, so a backlog of permanently overdue rows on one storage sorts ahead of a row that just came due on another, and the second row waits about backlog divided by batch size passes. There is no per-storage fairness rule, because expressing one would need a window function that PostgreSQL will not accept alongside `FOR UPDATE SKIP LOCKED`, and the delay it would prevent is bounded, visible through the oldest-obligation age and measured in passes rather than in anything a user waits on. This is the price of rejecting the alternative that stops retrying past a bounded age, which needs a fourth lifecycle state and an age threshold to end.
- [Files remain after a storage is deleted with an unreachable provider] -> The drain attempt and the audit-log entry record exactly which files were left, in the same place the user reads other destructive events.
- [A credential without delete permission fails only at cleanup time] -> Every failure is recorded on its row with the provider error and named in the attempt log, and the summary's oldest-obligation age shows that something is not draining. Rejecting such credentials at configuration time would break storages that work today for write-only workloads.
- [A storage repointed at another location strands its old files] -> Per-attempt names mean the pending deletion targets a name that cannot collide with anything else, so the risk is a leftover file in the old location, which is what happens today.
- [A second application process would defeat the live-write set] -> Writes in another process are invisible to this one's set, so its worker could delete a file mid-write. The `single-instance-runtime` capability makes that topology unsupported, and this change relies on that guarantee rather than re-deriving it.
- [Provider cleanup discovery can delete a similarly named object] -> Providers match exact derived names, and MinIO coverage includes prefix-collision fixtures.
- [Transactional database removal holds a transaction open across listener work] -> The listeners that talk to the source database keep doing best-effort work outside the transaction, and only catalog writes and deletion records run inside it. Production `DeleteDatabase` (`backend/internal/features/databases/service.go:252`) has no deadlock retry today; only the test path at `:299` retries `40P01`. A longer transaction makes a `40P01` more likely, so the deletion returns that error to its caller and the operation is repeated, which is safe because nothing was committed.
- [Backups that failed before this change keep their files] -> They are cleaned when the backup or its database is deleted, which routes through the new contract. A migration that seeded them would run remote deletions derived from historical rows on the first upgraded start, against provider code whose complete-deletion behavior nothing has exercised yet.

## Migration Plan

1. Add `storage_pending_deletions`, its constraint, index and cascade. Nothing writes to it yet.
2. Deploy the provider complete-deletion changes.
3. Deploy the file store and migrate logical writes and terminal-state transactions, then physical FULL and INCR, WAL, history, sidecars, manifests, database deletion and retention deletion. Remove direct production write and deletion calls from backup packages in the same release.
4. Start one `DeletionWorker` after dependency setup.
5. Add the storage deletion reference check and the drain on delete.
6. Verify queue drain, retry logs, provider absence and backup restore coverage before considering the rollout complete.

Rollback first stops the worker, then reverts callers and provider changes. The down migration drops the table, which discards obligations recorded by the newer release; the release notes say so, because the alternative is a down migration that refuses to run.
