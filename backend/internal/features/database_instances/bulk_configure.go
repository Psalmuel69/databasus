package database_instances

import (
	"errors"
	"fmt"

	"databasus-backend/internal/features/databases"
	"databasus-backend/internal/features/databases/databases/mariadb"
	"databasus-backend/internal/features/databases/databases/mongodb"
	"databasus-backend/internal/features/databases/databases/mysql"
	postgresql_logical "databasus-backend/internal/features/databases/databases/postgresql/logical"
	postgresql_physical "databasus-backend/internal/features/databases/databases/postgresql/physical"
	"databasus-backend/internal/features/notifiers"
	users_models "databasus-backend/internal/features/users/models"
)

// BulkConfigureBackups turns selected discovered databases into managed ones:
// for every name it materializes a Database row (connection details derived from
// the parent instance) and saves the shared backup config template against it.
//
// Each database goes through the existing single-item creation path, so it gets
// the same validation, connection test, credential encryption, and audit logging
// as a manually added database. Failures are collected per item instead of
// aborting the batch - one unreachable database must not block the other N.
func (s *DatabaseInstanceService) BulkConfigureBackups(
	user *users_models.User,
	request *BulkConfigureBackupsRequest,
) (*BulkConfigureBackupsResponse, error) {
	instance, err := s.getAuthorizedInstance(user, request.InstanceID)
	if err != nil {
		return nil, err
	}

	if err := validateBulkRequest(instance, request); err != nil {
		return nil, err
	}

	configured, err := s.findConfiguredDatabases(user, instance)
	if err != nil {
		return nil, err
	}

	notifierStubs := make([]notifiers.Notifier, len(request.NotifierIDs))
	for i, id := range request.NotifierIDs {
		notifierStubs[i] = notifiers.Notifier{ID: id}
	}

	response := &BulkConfigureBackupsResponse{
		Skipped: []string{},
		Failed:  []BulkConfigureFailure{},
	}

	for _, name := range request.DatabaseNames {
		if _, isConfigured := configured[name]; isConfigured {
			response.Skipped = append(response.Skipped, name)

			continue
		}

		if err := s.configureSingleDatabase(user, instance, name, request, notifierStubs); err != nil {
			response.Failed = append(response.Failed, BulkConfigureFailure{Name: name, Error: err.Error()})

			continue
		}

		response.CreatedCount++
	}

	return response, nil
}

// validateBulkRequest checks the backup-type/config pairing once, up front,
// instead of failing every single database in the loop with the same error.
func validateBulkRequest(instance *DatabaseInstance, request *BulkConfigureBackupsRequest) error {
	switch request.BackupType {
	case BulkBackupTypeLogical:
		if request.LogicalConfig == nil {
			return errors.New("logicalConfig is required when backupType is LOGICAL")
		}

		return nil
	case BulkBackupTypePhysical:
		if instance.Type != InstanceTypePostgres {
			return fmt.Errorf(
				"physical backups are only supported for PostgreSQL instances, not %q",
				instance.Type,
			)
		}

		if request.PhysicalConfig == nil {
			return errors.New("physicalConfig is required when backupType is PHYSICAL")
		}

		switch request.PhysicalBackupType {
		case "",
			postgresql_physical.BackupTypeFullOnly,
			postgresql_physical.BackupTypeFullAndIncremental,
			postgresql_physical.BackupTypeFullIncrementalAndWalStream:
		default:
			return fmt.Errorf("invalid physicalBackupType: %q", request.PhysicalBackupType)
		}

		return nil
	default:
		return fmt.Errorf("invalid backup type: %q", request.BackupType)
	}
}

func (s *DatabaseInstanceService) configureSingleDatabase(
	user *users_models.User,
	instance *DatabaseInstance,
	databaseName string,
	request *BulkConfigureBackupsRequest,
	notifierStubs []notifiers.Notifier,
) error {
	database, err := buildDatabaseFromInstance(
		instance,
		databaseName,
		request.BackupType,
		request.PhysicalBackupType,
	)
	if err != nil {
		return err
	}

	database.Notifiers = notifierStubs

	created, err := s.databaseService.CreateDatabase(user, instance.WorkspaceID, database)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	switch request.BackupType {
	case BulkBackupTypeLogical:
		configCopy := *request.LogicalConfig
		configCopy.DatabaseID = created.ID

		if _, err := s.backupConfigService.SaveBackupConfigWithAuth(user, &configCopy); err != nil {
			return fmt.Errorf("failed to save backup config: %w", err)
		}
	case BulkBackupTypePhysical:
		configCopy := *request.PhysicalConfig
		configCopy.DatabaseID = created.ID

		if _, err := s.physicalBackupConfigService.SaveBackupConfigWithAuth(user, &configCopy); err != nil {
			return fmt.Errorf("failed to save backup config: %w", err)
		}
	}

	return nil
}

// buildDatabaseFromInstance derives a per-database registration from the parent
// instance. Credentials are copied as stored (encrypted): the creation path
// decrypts transparently for its connection test and skips re-encrypting
// already-encrypted values.
func buildDatabaseFromInstance(
	instance *DatabaseInstance,
	databaseName string,
	backupType BulkBackupType,
	physicalBackupType postgresql_physical.BackupType,
) (*databases.Database, error) {
	database := &databases.Database{Name: databaseName}

	switch instance.Type {
	case InstanceTypePostgres:
		if backupType == BulkBackupTypePhysical {
			if physicalBackupType == "" {
				physicalBackupType = postgresql_physical.BackupTypeFullOnly
			}

			database.Type = databases.DatabaseTypePostgresPhysical
			database.PostgresqlPhysical = &postgresql_physical.PostgresqlPhysicalDatabase{
				// Version, ReplicationSlotName, SystemIdentifier and
				// WalSegmentSizeBytes are auto-detected by
				// DatabaseService.CreateDatabase's PopulateDbData step -
				// same as the single-database creation path.
				BackupType:    physicalBackupType,
				Host:          instance.Host,
				Port:          derefPortOrZero(instance.Port),
				Username:      instance.Username,
				Password:      instance.Password,
				SslMode:       instance.SslMode,
				SslClientCert: instance.SslClientCert,
				SslClientKey:  instance.SslClientKey,
				SslRootCert:   instance.SslRootCert,
			}

			break
		}

		database.Type = databases.DatabaseTypePostgresLogical
		database.PostgresqlLogical = &postgresql_logical.PostgresqlLogicalDatabase{
			Host:          instance.Host,
			Port:          derefPortOrZero(instance.Port),
			Username:      instance.Username,
			Password:      instance.Password,
			Database:      &databaseName,
			SslMode:       instance.SslMode,
			SslClientCert: instance.SslClientCert,
			SslClientKey:  instance.SslClientKey,
			SslRootCert:   instance.SslRootCert,
			// Matches the default used everywhere else a database is created
			// (see initializeDatabaseTypeData.ts / edit forms) - adjustable
			// afterwards from the database's edit screen.
			CpuCount: 1,
		}
	case InstanceTypeMysql:
		database.Type = databases.DatabaseTypeMysql
		database.Mysql = &mysql.MysqlDatabase{
			Host:     instance.Host,
			Port:     derefPortOrZero(instance.Port),
			Username: instance.Username,
			Password: instance.Password,
			Database: &databaseName,
			IsHttps:  instance.IsTlsEnabled,
		}
	case InstanceTypeMariadb:
		database.Type = databases.DatabaseTypeMariadb
		database.Mariadb = &mariadb.MariadbDatabase{
			Host:     instance.Host,
			Port:     derefPortOrZero(instance.Port),
			Username: instance.Username,
			Password: instance.Password,
			Database: &databaseName,
			IsHttps:  instance.IsTlsEnabled,
		}
	case InstanceTypeMongodb:
		database.Type = databases.DatabaseTypeMongodb
		database.Mongodb = &mongodb.MongodbDatabase{
			Host:         instance.Host,
			Port:         instance.Port,
			Username:     instance.Username,
			Password:     instance.Password,
			Database:     databaseName,
			AuthDatabase: instance.AuthDatabase,
			IsSrv:        instance.IsSrv,
			CpuCount:     1,
		}
	default:
		return nil, fmt.Errorf("bulk configuration is not supported for instance type: %q", instance.Type)
	}

	return database, nil
}

func derefPortOrZero(port *int) int {
	if port == nil {
		return 0
	}

	return *port
}
