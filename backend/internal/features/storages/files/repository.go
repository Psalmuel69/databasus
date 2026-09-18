package storage_files

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PendingDeletionRepository struct{}

// A conflict means the pair already carries an obligation, so the write must not
// start: either a cascade reached the file first, or a second writer picked a name
// per-attempt naming should have made unique.
func (r *PendingDeletionRepository) InsertIfAbsent(
	tx *gorm.DB,
	id uuid.UUID,
	reference StoredFileReference,
	dueIn time.Duration,
) (*PendingDeletion, error) {
	var inserted []PendingDeletion

	result := tx.Raw(`
		INSERT INTO storage_pending_deletions (id, storage_id, file_name, not_before)
		VALUES (?, ?, ?, now() + (? * interval '1 second'))
		ON CONFLICT (storage_id, file_name) DO NOTHING
		RETURNING *`,
		id, reference.StorageID, reference.FileName, dueIn.Seconds(),
	).Scan(&inserted)
	if result.Error != nil {
		return nil, fmt.Errorf("insert pending deletion: %w", result.Error)
	}

	if len(inserted) == 0 {
		return nil, ErrFileNameAlreadyRegistered
	}

	return &inserted[0], nil
}

// Against an existing row this also increments the generation, which is what takes
// the file away from a writer that is still running and stops it publishing.
func (r *PendingDeletionRepository) InsertOrTake(tx *gorm.DB, references []StoredFileReference) error {
	unique := deduplicateReferences(references)
	if len(unique) == 0 {
		return nil
	}

	placeholders := make([]string, 0, len(unique))
	args := make([]any, 0, len(unique)*2)

	for _, reference := range unique {
		placeholders = append(placeholders, "(?, ?, now())")
		args = append(args, reference.StorageID, reference.FileName)
	}

	statement := fmt.Sprintf(`
		INSERT INTO storage_pending_deletions (storage_id, file_name, not_before)
		VALUES %s
		ON CONFLICT (storage_id, file_name) DO UPDATE
		SET not_before = now(),
		    generation = storage_pending_deletions.generation + 1,
		    updated_at = now()`, strings.Join(placeholders, ", "))

	if err := tx.Exec(statement, args...).Error; err != nil {
		return fmt.Errorf("record pending deletions: %w", err)
	}

	return nil
}

// This is how a publishing transaction keeps a file and how the worker records a
// finished deletion, so it refuses when the generation moved and someone else owns
// the outcome.
func (r *PendingDeletionRepository) CompleteIfGeneration(
	tx *gorm.DB,
	id uuid.UUID,
	generation int,
) (bool, error) {
	result := tx.
		Where("id = ? AND generation = ?", id, generation).
		Delete(&PendingDeletion{})
	if result.Error != nil {
		return false, fmt.Errorf("complete pending deletion: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

func (r *PendingDeletionRepository) SetNotBeforeIfGeneration(
	tx *gorm.DB,
	id uuid.UUID,
	generation int,
	dueIn time.Duration,
) (bool, error) {
	result := tx.Model(&PendingDeletion{}).
		Where("id = ? AND generation = ?", id, generation).
		Updates(map[string]any{
			"not_before": databaseTimeIn(dueIn),
			"updated_at": gorm.Expr("now()"),
		})
	if result.Error != nil {
		return false, fmt.Errorf("reschedule pending deletion: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

func (r *PendingDeletionRepository) RescheduleIfGeneration(
	tx *gorm.DB,
	id uuid.UUID,
	generation int,
	dueIn time.Duration,
	lastError string,
) (bool, error) {
	result := tx.Model(&PendingDeletion{}).
		Where("id = ? AND generation = ?", id, generation).
		Updates(map[string]any{
			"not_before": databaseTimeIn(dueIn),
			"last_error": lastError,
			"updated_at": gorm.Expr("now()"),
		})
	if result.Error != nil {
		return false, fmt.Errorf("reschedule pending deletion: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

// Raising the generation invalidates any receipt still outstanding for these files,
// and pushing not_before by the lease keeps a stalled provider call from holding a
// row forever. SKIP LOCKED keeps an open deletion request from stalling the whole
// batch.
func (r *PendingDeletionRepository) ClaimDue(
	tx *gorm.DB,
	request ClaimRequest,
) ([]PendingDeletion, error) {
	exclusion := ""
	args := []any{request.AttemptLease.Seconds()}

	if len(request.ExcludedIDs) > 0 {
		exclusion = "AND id NOT IN (?)"

		args = append(args, request.ExcludedIDs)
	}

	args = append(args, request.Limit)

	statement := fmt.Sprintf(`
		UPDATE storage_pending_deletions
		SET generation = generation + 1,
		    attempt_count = attempt_count + 1,
		    not_before = now() + (? * interval '1 second'),
		    updated_at = now()
		WHERE id IN (
			SELECT id
			FROM storage_pending_deletions
			WHERE not_before <= now() %s
			ORDER BY not_before
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		RETURNING *`, exclusion)

	var claimed []PendingDeletion

	if err := tx.Raw(statement, args...).Scan(&claimed).Error; err != nil {
		return nil, fmt.Errorf("claim pending deletions: %w", err)
	}

	return claimed, nil
}

func (r *PendingDeletionRepository) GetPendingDeletionSummary(tx *gorm.DB) (PendingDeletionSummary, error) {
	var summary PendingDeletionSummary

	err := tx.Raw(`
		SELECT count(*)                                                              AS pending,
		       count(*) FILTER (WHERE not_before <= now())                           AS overdue,
		       coalesce(extract(epoch FROM now() - min(created_at)) * 1000, 0)::bigint AS oldest_age_ms
		FROM storage_pending_deletions`).Scan(&summary).Error
	if err != nil {
		return PendingDeletionSummary{}, fmt.Errorf("summarize pending deletions: %w", err)
	}

	return summary, nil
}

func (r *PendingDeletionRepository) CountByReferences(
	tx *gorm.DB,
	references []StoredFileReference,
) (int64, error) {
	unique := deduplicateReferences(references)
	if len(unique) == 0 {
		return 0, nil
	}

	pairs := make([][]any, 0, len(unique))
	for _, reference := range unique {
		pairs = append(pairs, []any{reference.StorageID, reference.FileName})
	}

	var count int64

	err := tx.Model(&PendingDeletion{}).
		Where("(storage_id, file_name) IN ?", pairs).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count pending deletions: %w", err)
	}

	return count, nil
}

func (r *PendingDeletionRepository) FindByStorage(
	tx *gorm.DB,
	storageID uuid.UUID,
) ([]PendingDeletion, error) {
	var pending []PendingDeletion

	if err := tx.Where("storage_id = ?", storageID).Order("created_at").Find(&pending).Error; err != nil {
		return nil, fmt.Errorf("list pending deletions of a storage: %w", err)
	}

	return pending, nil
}

func (r *PendingDeletionRepository) FindByReference(
	tx *gorm.DB,
	reference StoredFileReference,
) (*PendingDeletion, error) {
	var pending PendingDeletion

	err := tx.
		Where("storage_id = ? AND file_name = ?", reference.StorageID, reference.FileName).
		First(&pending).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("find pending deletion: %w", err)
	}

	return &pending, nil
}

// databaseTimeIn keeps every deadline on the database clock, so app-to-Postgres
// skew cannot shorten a commit window on one path and lengthen it on another.
func databaseTimeIn(offset time.Duration) clause.Expr {
	return gorm.Expr("now() + (? * interval '1 second')", offset.Seconds())
}

func deduplicateReferences(references []StoredFileReference) []StoredFileReference {
	seen := make(map[StoredFileReference]struct{}, len(references))
	unique := make([]StoredFileReference, 0, len(references))

	for _, reference := range references {
		if _, duplicate := seen[reference]; duplicate {
			continue
		}

		seen[reference] = struct{}{}
		unique = append(unique, reference)
	}

	return unique
}
