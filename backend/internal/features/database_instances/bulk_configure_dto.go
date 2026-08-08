package database_instances

import (
	"github.com/google/uuid"

	backups_config_logical "databasus-backend/internal/features/backups/config/logical"
)

type BulkConfigureBackupsRequest struct {
	InstanceID    uuid.UUID `json:"instanceId"`
	DatabaseNames []string  `json:"databaseNames"`

	// BackupConfig is a template applied to every selected database;
	// its DatabaseID is set per database by the server.
	BackupConfig backups_config_logical.LogicalBackupConfig `json:"backupConfig"`
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
