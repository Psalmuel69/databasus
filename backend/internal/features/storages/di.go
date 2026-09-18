package storages

import (
	"sync"

	audit_logs "databasus-backend/internal/features/audit_logs"
	storage_files "databasus-backend/internal/features/storages/files"
	workspaces_services "databasus-backend/internal/features/workspaces/services"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
)

var (
	storageRepository = &StorageRepository{}
	storageService    = &StorageService{
		storageRepository,
		workspaces_services.GetWorkspaceService(),
		audit_logs.GetAuditLogService(),
		encryption.GetFieldEncryptor(),
		nil,
		nil,
	}
)

var (
	storageFileDependencies = storage_files.Dependencies{
		Repository:     &storage_files.PendingDeletionRepository{},
		Locator:        storageService,
		FieldEncryptor: encryption.GetFieldEncryptor(),
		Logger:         logger.GetLogger(),
		Timings:        storage_files.ProductionTimings(),
	}
	storageFileStore          = storage_files.NewStore(storageFileDependencies)
	storageFileDeletionWorker = storage_files.NewDeletionWorker(storageFileStore, storageFileDependencies)
)

var storageController = &StorageController{
	storageService,
	workspaces_services.GetWorkspaceService(),
}

func GetStorageService() *StorageService {
	return storageService
}

func GetStorageController() *StorageController {
	return storageController
}

func GetStorageFileStore() *storage_files.Store {
	return storageFileStore
}

func GetStorageFileDeletionWorker() *storage_files.DeletionWorker {
	return storageFileDeletionWorker
}

var SetupDependencies = sync.OnceFunc(func() {
	workspaces_services.GetWorkspaceService().AddWorkspaceDeletionListener(storageService)
})
