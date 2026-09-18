## MODIFIED Requirements

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

