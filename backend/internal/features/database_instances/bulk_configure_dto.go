package database_instances

import (
	"github.com/google/uuid"

	backups_config_logical "databasus-backend/internal/features/backups/config/logical"
	backups_config_physical "databasus-backend/internal/features/backups/config/physical"
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
