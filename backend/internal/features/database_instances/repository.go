package database_instances

import (
	"time"

	"github.com/google/uuid"

	"databasus-backend/internal/storage"
)

type DatabaseInstanceRepository struct{}

func (r *DatabaseInstanceRepository) Save(instance *DatabaseInstance) (*DatabaseInstance, error) {
	db := storage.GetDb()

	if instance.ID == uuid.Nil {
		instance.ID = uuid.New()

		if err := db.Create(instance).Error; err != nil {
			return nil, err
		}

		return instance, nil
	}

	if err := db.Save(instance).Error; err != nil {
		return nil, err
	}

	return instance, nil
}

func (r *DatabaseInstanceRepository) FindByID(id uuid.UUID) (*DatabaseInstance, error) {
	var instance DatabaseInstance

	if err := storage.
		GetDb().
		Where("id = ?", id).
		First(&instance).Error; err != nil {
		return nil, err
	}

	return &instance, nil
}

func (r *DatabaseInstanceRepository) FindByWorkspaceID(workspaceID uuid.UUID) ([]*DatabaseInstance, error) {
	var instances []*DatabaseInstance

	if err := storage.
		GetDb().
		Where("workspace_id = ?", workspaceID).
		Order("name ASC").
		Find(&instances).Error; err != nil {
		return nil, err
	}

	return instances, nil
}

func (r *DatabaseInstanceRepository) UpdateLastDiscoveredAt(id uuid.UUID, discoveredAt time.Time) error {
	return storage.
		GetDb().
		Model(&DatabaseInstance{}).
		Where("id = ?", id).
		Update("last_discovered_at", discoveredAt).Error
}

func (r *DatabaseInstanceRepository) Delete(id uuid.UUID) error {
	return storage.GetDb().Delete(&DatabaseInstance{}, id).Error
}
