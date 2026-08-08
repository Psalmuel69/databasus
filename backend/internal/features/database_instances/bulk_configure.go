package database_instances

import (
	"fmt"

	backups_config_logical "databasus-backend/internal/features/backups/config/logical"
	"databasus-backend/internal/features/databases"
	"databasus-backend/internal/features/databases/databases/mariadb"
	"databasus-backend/internal/features/databases/databases/mongodb"
	"databasus-backend/internal/features/databases/databases/mysql"
	postgresql_logical "databasus-backend/internal/features/databases/databases/postgresql/logical"
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

	configured, err := s.findConfiguredDatabases(user, instance)
	if err != nil {
		return nil, err
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

		if err := s.configureSingleDatabase(user, instance, name, request.BackupConfig); err != nil {
			response.Failed = append(response.Failed, BulkConfigureFailure{Name: name, Error: err.Error()})

			continue
		}

		response.CreatedCount++
	}

	return response, nil
}

func (s *DatabaseInstanceService) configureSingleDatabase(
	user *users_models.User,
	instance *DatabaseInstance,
	databaseName string,
	configTemplate backups_config_logical.LogicalBackupConfig,
) error {
	database, err := buildDatabaseFromInstance(instance, databaseName)
	if err != nil {
		return err
	}

	created, err := s.databaseService.CreateDatabase(user, instance.WorkspaceID, database)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	configTemplate.DatabaseID = created.ID

	if _, err := s.backupConfigService.SaveBackupConfigWithAuth(user, &configTemplate); err != nil {
		return fmt.Errorf("failed to save backup config: %w", err)
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
) (*databases.Database, error) {
	database := &databases.Database{Name: databaseName}

	switch instance.Type {
	case InstanceTypePostgres:
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
