# Local Storage File Lifecycle Specification

## Purpose

Defines how local storage writes, publishes, reads, deletes, and probes backup files across one or more filesystems.

## Requirements

### Requirement: Local storage stages files before publication

Local storage SHALL write an incoming backup to the configured temporary folder. It SHALL sync and close the temporary file before publishing it at the corresponding path in the configured data folder. It SHALL create missing parent directories for both paths.

#### Scenario: Backup file is saved

- **WHEN** local storage receives a backup stream
- **THEN** it writes the stream to the temporary folder
- **AND** it syncs and closes the temporary file before publishing the final path

#### Scenario: Context is canceled during a write

- **WHEN** the operation context is canceled before or while local storage copies the stream
- **THEN** the save returns the context error
- **AND** it does not publish the temporary path through the normal completion path

### Requirement: Local storage supports separate temporary and data filesystems

Local storage SHALL publish a completed temporary file with a filesystem rename when the source and destination support it. When the temporary and data folders are on different filesystems, local storage SHALL copy the temporary file to the final path, sync the destination, and remove the temporary file after the copy succeeds.

#### Scenario: Temporary and final paths share a filesystem

- **WHEN** the completed temporary file can be renamed to the final path
- **THEN** local storage publishes it with that rename

#### Scenario: Temporary and final paths use different filesystems

- **WHEN** the temporary and final paths are on different filesystems
- **THEN** local storage copies the file to the final path and syncs it
- **AND** it removes the temporary file after the copy succeeds

#### Scenario: Rename fails for another reason

- **WHEN** the rename fails for a reason other than a filesystem boundary
- **THEN** local storage returns that failure without attempting the cross-filesystem copy path

### Requirement: Local storage reads published files

Local storage SHALL open files from the configured data folder. It SHALL return a file-not-found error when the requested path does not exist.

#### Scenario: Published file exists

- **WHEN** a caller requests an existing file
- **THEN** local storage returns a readable handle for the path in the data folder

#### Scenario: Published file is missing

- **WHEN** a caller requests a path that does not exist
- **THEN** local storage returns a file-not-found error

### Requirement: Local storage deletion is idempotent

Local storage SHALL remove both the requested path from the configured data folder and the matching staging path from the configured temporary folder. A missing path in either location SHALL be treated as successfully deleted only when the filesystem confirms that it does not exist.

#### Scenario: Published file is deleted

- **WHEN** a caller deletes an existing published file
- **THEN** local storage removes it from the data folder
- **AND** it removes a matching staging file when one exists

#### Scenario: Interrupted staging file is deleted

- **WHEN** an interrupted write leaves a file only in the temporary folder
- **THEN** local storage removes that staging file

#### Scenario: Missing file is deleted

- **WHEN** neither the published path nor the staging path exists
- **THEN** local storage returns success

#### Scenario: Filesystem lookup fails

- **WHEN** local storage cannot determine whether either path exists
- **THEN** deletion returns the filesystem error

### Requirement: Local storage connection tests exercise the temporary folder

The local-storage connection test SHALL create, close, and remove a probe file in the configured temporary folder. It SHALL return an error when any required operation fails.

#### Scenario: Temporary folder permits the required operations

- **WHEN** the connection test can create, close, and remove its probe file
- **THEN** the connection test succeeds

#### Scenario: Temporary folder rejects a required operation

- **WHEN** probe creation, close, or removal fails
- **THEN** the connection test returns the operation error
