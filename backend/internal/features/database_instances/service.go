package database_instances

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	audit_logs "databasus-backend/internal/features/audit_logs"
	audit_logs_models "databasus-backend/internal/features/audit_logs/models"
	backups_config_logical "databasus-backend/internal/features/backups/config/logical"
	backups_config_physical "databasus-backend/internal/features/backups/config/physical"
	"databasus-backend/internal/features/databases"
	users_models "databasus-backend/internal/features/users/models"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/encryption"
)

const discoveryTimeout = 60 * time.Second

type DatabaseInstanceService struct {
	instanceRepository          *DatabaseInstanceRepository
	workspaceService            *workspaces_services.WorkspaceService
	auditLogService             *audit_logs.AuditLogService
	fieldEncryptor              encryption.FieldEncryptor
	logger                      *slog.Logger
	databaseService             *databases.DatabaseService
	backupConfigService         *backups_config_logical.BackupConfigService
	physicalBackupConfigService *backups_config_physical.BackupConfigService
}

func (s *DatabaseInstanceService) RegisterInstance(
	ctx context.Context,
	user *users_models.User,
	workspaceID uuid.UUID,
	instance *DatabaseInstance,
) (*DatabaseInstance, error) {
	canManage, err := s.workspaceService.CanUserManageDBs(ctx, workspaceID, user)
	if err != nil {
		return nil, err
	}

	if !canManage {
		return nil, errors.New("insufficient permissions to register instances in this workspace")
	}

	instance.WorkspaceID = workspaceID

	if err := instance.Validate(); err != nil {
		return nil, err
	}

	if err := s.testConnection(ctx, instance); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	if err := instance.EncryptSensitiveFields(s.fieldEncryptor); err != nil {
		return nil, fmt.Errorf("failed to encrypt sensitive fields: %w", err)
	}

	instance, err = s.instanceRepository.Save(instance)
	if err != nil {
		return nil, err
	}

	s.auditLogService.WriteAuditLog(ctx, audit_logs_models.AuditEntry{
		Message:     fmt.Sprintf("Database instance registered: %s", instance.Name),
		UserID:      &user.ID,
		WorkspaceID: &workspaceID,
	})

	return instance, nil
}

func (s *DatabaseInstanceService) UpdateInstance(
	ctx context.Context,
	user *users_models.User,
	incoming *DatabaseInstance,
) (*DatabaseInstance, error) {
	existing, err := s.getAuthorizedInstance(ctx, user, incoming.ID)
	if err != nil {
		return nil, err
	}

	existing.Update(incoming)

	if err := existing.Validate(); err != nil {
		return nil, err
	}

	// After Update, fields may be mixed plaintext (newly set) and ciphertext
	// (untouched). Decrypt passes plaintext through unchanged and
	// EncryptSensitiveFields skips already-encrypted values, so both states are safe.
	if err := s.testConnection(ctx, existing); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	if err := existing.EncryptSensitiveFields(s.fieldEncryptor); err != nil {
		return nil, fmt.Errorf("failed to encrypt sensitive fields: %w", err)
	}

	updated, err := s.instanceRepository.Save(existing)
	if err != nil {
		return nil, err
	}

	s.auditLogService.WriteAuditLog(ctx, audit_logs_models.AuditEntry{
		Message:     fmt.Sprintf("Database instance updated: %s", updated.Name),
		UserID:      &user.ID,
		WorkspaceID: &updated.WorkspaceID,
	})

	return updated, nil
}

func (s *DatabaseInstanceService) GetInstancesByWorkspace(
	ctx context.Context,
	user *users_models.User,
	workspaceID uuid.UUID,
) ([]*DatabaseInstance, error) {
	canAccess, _, err := s.workspaceService.CanUserAccessWorkspace(ctx, workspaceID, user)
	if err != nil {
		return nil, err
	}

	if !canAccess {
		return nil, errors.New("insufficient permissions to view instances in this workspace")
	}

	instances, err := s.instanceRepository.FindByWorkspaceID(workspaceID)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		instance.HideSensitiveData()
	}

	return instances, nil
}

func (s *DatabaseInstanceService) GetInstance(
	ctx context.Context,
	user *users_models.User,
	instanceID uuid.UUID,
) (*DatabaseInstance, error) {
	instance, err := s.getAuthorizedInstance(ctx, user, instanceID)
	if err != nil {
		return nil, err
	}

	instance.HideSensitiveData()

	return instance, nil
}

func (s *DatabaseInstanceService) DeleteInstance(
	ctx context.Context,
	user *users_models.User,
	instanceID uuid.UUID,
) error {
	instance, err := s.getAuthorizedInstance(ctx, user, instanceID)
	if err != nil {
		return err
	}

	if err := s.instanceRepository.Delete(instanceID); err != nil {
		return err
	}

	s.auditLogService.WriteAuditLog(ctx, audit_logs_models.AuditEntry{
		Message:     fmt.Sprintf("Database instance deleted: %s", instance.Name),
		UserID:      &user.ID,
		WorkspaceID: &instance.WorkspaceID,
	})

	return nil
}

// DiscoverDatabases runs provider discovery against a registered instance and
// returns every database the instance credentials can access. It intentionally
// does not create Database rows - selection and configuration happen later.
func (s *DatabaseInstanceService) DiscoverDatabases(
	ctx context.Context,
	user *users_models.User,
	instanceID uuid.UUID,
) (*DiscoverDatabasesResponse, error) {
	instance, err := s.getAuthorizedInstance(ctx, user, instanceID)
	if err != nil {
		return nil, err
	}

	provider, err := GetDiscoveryProvider(instance.Type)
	if err != nil {
		return nil, err
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	databases, err := provider.DiscoverDatabases(discoveryCtx, instance, s.fieldEncryptor)
	if err != nil {
		s.logger.Error("Database discovery failed", "instanceId", instanceID, "error", err)

		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	configured, err := s.findConfiguredDatabases(ctx, user, instance)
	if err != nil {
		return nil, err
	}

	for index := range databases {
		info, isConfigured := configured[databases[index].Name]
		if !isConfigured {
			continue
		}

		databases[index].IsConfigured = true
		databases[index].LastBackupTime = info.lastBackupTime
	}

	discoveredAt := time.Now()

	if err := s.instanceRepository.UpdateLastDiscoveredAt(instanceID, discoveredAt); err != nil {
		s.logger.Error("Failed to update last discovered time", "instanceId", instanceID, "error", err)
	}

	return &DiscoverDatabasesResponse{Databases: databases, DiscoveredAt: discoveredAt}, nil
}

func (s *DatabaseInstanceService) getAuthorizedInstance(
	ctx context.Context,
	user *users_models.User,
	instanceID uuid.UUID,
) (*DatabaseInstance, error) {
	instance, err := s.instanceRepository.FindByID(instanceID)
	if err != nil {
		return nil, err
	}

	canManage, err := s.workspaceService.CanUserManageDBs(ctx, instance.WorkspaceID, user)
	if err != nil {
		return nil, err
	}

	if !canManage {
		return nil, errors.New("insufficient permissions for this instance")
	}

	return instance, nil
}

// testConnection reuses discovery as the connectivity probe: if the catalog
// query succeeds, the credentials, network path, and permissions all work.
func (s *DatabaseInstanceService) testConnection(ctx context.Context, instance *DatabaseInstance) error {
	provider, err := GetDiscoveryProvider(instance.Type)
	if err != nil {
		return err
	}

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err = provider.DiscoverDatabases(probeCtx, instance, s.fieldEncryptor)

	return err
}
