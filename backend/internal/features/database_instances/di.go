package database_instances

import (
	audit_logs "databasus-backend/internal/features/audit_logs"
	backups_config_logical "databasus-backend/internal/features/backups/config/logical"
	"databasus-backend/internal/features/databases"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var databaseInstanceRepository = &DatabaseInstanceRepository{}

var databaseInstanceService = &DatabaseInstanceService{
	databaseInstanceRepository,
	workspaces_services.GetWorkspaceService(),
	audit_logs.GetAuditLogService(),
	encryption.GetFieldEncryptor(),
	logger.GetLogger(),
	databases.GetDatabaseService(),
	backups_config_logical.GetBackupConfigService(),
}

var databaseInstanceController = &DatabaseInstanceController{
	databaseInstanceService,
}

func GetDatabaseInstanceService() *DatabaseInstanceService {
	return databaseInstanceService
}

func GetDatabaseInstanceController() *DatabaseInstanceController {
	return databaseInstanceController
}
