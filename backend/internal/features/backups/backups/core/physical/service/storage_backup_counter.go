package physical_service

import (
	"fmt"

	"github.com/google/uuid"

	physical_models "databasus-backend/internal/features/backups/backups/core/physical/models"
	db "databasus-backend/internal/storage"
)

// The physical rows are the only record of the file names in a storage, so the
// storage cannot go while they exist.
func (s *PhysicalBackupService) GetStorageBackupReferenceCount(storageID uuid.UUID) (int64, error) {
	var total int64

	for _, model := range []any{
		&physical_models.PhysicalFullBackup{},
		&physical_models.PhysicalIncrementalBackup{},
		&physical_models.PhysicalWalSegment{},
		&physical_models.PhysicalWalHistoryFile{},
	} {
		var count int64

		if err := db.GetDb().Model(model).Where("storage_id = ?", storageID).Count(&count).Error; err != nil {
			return 0, fmt.Errorf("count physical rows of a storage: %w", err)
		}

		total += count
	}

	return total, nil
}
