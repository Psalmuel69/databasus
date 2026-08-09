package database_instances

import (
	"github.com/google/uuid"

	backups_config_logical "databasus-backend/internal/features/backups/config/logical"
	backups_config_physical "databasus-backend/internal/features/backups/config/physical"
	postgresql_physical "databasus-backend/internal/features/databases/databases/postgresql/physical"
)

// BulkBackupType selects which backup mechanism bulk-configure creates for
// every selected database. PHYSICAL is only valid for PostgreSQL instances -
// mirrors the single-database ChoosePostgresBackupTypeComponent restriction.
type BulkBackupType string

const (
	BulkBackupTypeLogical  BulkBackupType = "LOGICAL"
	BulkBackupTypePhysical BulkBackupType = "PHYSICAL"
)

type BulkConfigureBackupsRequest struct {
	InstanceID    uuid.UUID `json:"instanceId"`
	DatabaseNames []string  `json:"databaseNames"`

	BackupType BulkBackupType `json:"backupType"`

	// Exactly one of these is used, matching BackupType; DatabaseID on
	// either is set per database by the server.
	LogicalConfig  *backups_config_logical.LogicalBackupConfig   `json:"logicalConfig,omitempty"`
	PhysicalConfig *backups_config_physical.PhysicalBackupConfig `json:"physicalConfig,omitempty"`

	// Only used when BackupType is PHYSICAL - the database-level strategy
	// (FULL / FULL_INCREMENTAL / FULL_INCREMENTAL_WAL_STREAM) that decides
	// which retention modes PhysicalConfig may use. Defaults to FULL if empty.
	PhysicalBackupType postgresql_physical.BackupType `json:"physicalBackupType,omitempty"`

	// Notifiers attached to every created database; each config's own
	// sendNotificationsOn list decides which events actually notify them.
	NotifierIDs []uuid.UUID `json:"notifierIds"`
}

type BulkConfigureFailure struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

type BulkConfigureBackupsResponse struct {
	CreatedCount int                    `json:"createdCount"`
	Skipped      []string               `json:"skipped"`
	Failed       []BulkConfigureFailure `json:"failed"`
}
