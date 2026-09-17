package storage_files

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	db "databasus-backend/internal/storage"
)

// Both batch methods refuse a bare handle, so every call here goes through one.
func inTransaction(t *testing.T, work func(tx *gorm.DB) error) error {
	t.Helper()

	return db.GetDb().Transaction(work)
}

func Test_ConfirmFileWrites_WhenGivenNoTransaction_Refuses(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	reference := StoredFileReference{StorageID: createStorageRow(t), FileName: "backup-1"}

	receipt, err := store.WriteFile(t.Context(), reference, strings.NewReader("payload"))
	require.NoError(t, err)

	assert.ErrorIs(t,
		store.ConfirmFileWrites(t.Context(), db.GetDb(), []WriteReceipt{receipt}), ErrTransactionRequired)
	assert.ErrorIs(t, store.ConfirmFileWrites(t.Context(), nil, []WriteReceipt{receipt}), ErrTransactionRequired)
}

func Test_RequestFileDeletions_WhenGivenNoTransaction_Refuses(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	references := []StoredFileReference{{StorageID: createStorageRow(t), FileName: "backup-1"}}

	assert.ErrorIs(t,
		store.RequestFileDeletions(t.Context(), db.GetDb(), references), ErrTransactionRequired)
	assert.ErrorIs(t, store.RequestFileDeletions(t.Context(), nil, references), ErrTransactionRequired)
}

func Test_ConfirmFileWrites_WhenTransactionCommits_KeepsTheFile(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	storageID := createStorageRow(t)
	base := StoredFileReference{StorageID: storageID, FileName: "backup-1"}
	sidecar := StoredFileReference{StorageID: storageID, FileName: "backup-1.metadata"}

	baseReceipt, err := store.WriteFile(t.Context(), base, strings.NewReader("payload"))
	require.NoError(t, err)

	sidecarReceipt, err := store.WriteFile(t.Context(), sidecar, strings.NewReader("meta"))
	require.NoError(t, err)

	require.NoError(t, db.GetDb().Transaction(func(tx *gorm.DB) error {
		return store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{baseReceipt, sidecarReceipt})
	}))

	for _, reference := range []StoredFileReference{base, sidecar} {
		pending, findErr := store.repository.FindByReference(db.GetDb(), reference)
		require.NoError(t, findErr)
		assert.Nil(t, pending, "a published file carries no obligation")
	}
}

func Test_ConfirmFileWrites_WhenTransactionRollsBack_KeepsTheObligation(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	reference := StoredFileReference{StorageID: createStorageRow(t), FileName: "backup-1"}

	receipt, err := store.WriteFile(t.Context(), reference, strings.NewReader("payload"))
	require.NoError(t, err)

	err = db.GetDb().Transaction(func(tx *gorm.DB) error {
		if confirmErr := store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{receipt}); confirmErr != nil {
			return confirmErr
		}

		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)

	pending, err := store.repository.FindByReference(db.GetDb(), reference)
	require.NoError(t, err)
	assert.NotNil(t, pending, "an uncommitted publication leaves the file for cleanup")
}

func Test_ConfirmFileWrites_WhenCalledTwice_FailsTheSecondTime(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	reference := StoredFileReference{StorageID: createStorageRow(t), FileName: "backup-1"}

	receipt, err := store.WriteFile(t.Context(), reference, strings.NewReader("payload"))
	require.NoError(t, err)

	require.NoError(t, inTransaction(t, func(tx *gorm.DB) error {
		return store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{receipt})
	}))

	err = inTransaction(t, func(tx *gorm.DB) error {
		return store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{receipt})
	})

	assert.ErrorIs(t, err, ErrReceiptNoLongerValid)
}

func Test_ConfirmFileWrites_WhenDeletionWasRequestedMeanwhile_Fails(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	reference := StoredFileReference{StorageID: createStorageRow(t), FileName: "backup-1"}

	receipt, err := store.WriteFile(t.Context(), reference, strings.NewReader("payload"))
	require.NoError(t, err)

	require.NoError(t, inTransaction(t, func(tx *gorm.DB) error {
		return store.RequestFileDeletions(t.Context(), tx, []StoredFileReference{reference})
	}))

	err = inTransaction(t, func(tx *gorm.DB) error {
		return store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{receipt})
	})

	assert.ErrorIs(t, err, ErrReceiptNoLongerValid)

	pending, findErr := store.repository.FindByReference(db.GetDb(), reference)
	require.NoError(t, findErr)
	assert.NotNil(t, pending)
}

func Test_ConfirmFileWrites_WhenOneReceiptIsStale_FailsTheWholeBatch(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	storageID := createStorageRow(t)
	good := StoredFileReference{StorageID: storageID, FileName: "backup-1"}
	stale := StoredFileReference{StorageID: storageID, FileName: "backup-1.metadata"}

	goodReceipt, err := store.WriteFile(t.Context(), good, strings.NewReader("payload"))
	require.NoError(t, err)

	staleReceipt, err := store.WriteFile(t.Context(), stale, strings.NewReader("meta"))
	require.NoError(t, err)

	require.NoError(t, inTransaction(t, func(tx *gorm.DB) error {
		return store.RequestFileDeletions(t.Context(), tx, []StoredFileReference{stale})
	}))

	err = db.GetDb().Transaction(func(tx *gorm.DB) error {
		return store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{goodReceipt, staleReceipt})
	})

	require.ErrorIs(t, err, ErrReceiptNoLongerValid)

	pending, findErr := store.repository.FindByReference(db.GetDb(), good)
	require.NoError(t, findErr)
	assert.NotNil(t, pending, "a failed batch leaves every file in the batch for cleanup")
}

func Test_RequestFileDeletions_WhenRepeated_KeepsOneObligation(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})
	reference := StoredFileReference{StorageID: createStorageRow(t), FileName: "backup-1"}

	for range 2 {
		require.NoError(t, inTransaction(t, func(tx *gorm.DB) error {
			return store.RequestFileDeletions(t.Context(), tx, []StoredFileReference{reference})
		}))
	}

	var count int64
	require.NoError(t, db.GetDb().Model(&PendingDeletion{}).
		Where("storage_id = ? AND file_name = ?", reference.StorageID, reference.FileName).
		Count(&count).Error)

	assert.EqualValues(t, 1, count)
}

func Test_RequestFileDeletions_WhenWriterIsRunning_Succeeds(t *testing.T) {
	store := newTestStore(nil)
	reference := StoredFileReference{StorageID: createStorageRow(t), FileName: "backup-1"}

	var requestErr error

	provider := &fakeProvider{onSave: func() {
		requestErr = inTransaction(t, func(tx *gorm.DB) error {
			return store.RequestFileDeletions(t.Context(), tx, []StoredFileReference{reference})
		})
	}}
	store.locator = &fakeLocator{provider: provider}

	_, writeErr := store.WriteFile(t.Context(), reference, strings.NewReader("payload"))

	assert.NoError(t, requestErr, "a cascade cannot wait for an upload it does not own")
	assert.ErrorIs(t, writeErr, ErrFileTakenForDeletion)

	pending, findErr := store.repository.FindByReference(db.GetDb(), reference)
	require.NoError(t, findErr)
	require.NotNil(t, pending)
	assert.True(t, isDue(t, reference), "the writer must not push a file that was taken from it further out")
}

func Test_RequestFileDeletions_WhenBatchIsEmpty_Succeeds(t *testing.T) {
	store := newTestStore(&fakeLocator{provider: &fakeProvider{}})

	assert.NoError(t, inTransaction(t, func(tx *gorm.DB) error {
		return store.RequestFileDeletions(t.Context(), tx, nil)
	}))
}

// A process that stops between the upload and the catalog commit leaves a receipt
// nobody will ever spend, which is indistinguishable from a caller that simply
// never published. Both end with the file gone once the commit window closes.
func Test_WriteFile_WhenNobodyPublishes_TheFileIsRemovedAndConfirmedOnesAreKept(t *testing.T) {
	provider := &fakeProvider{}
	locator := &fakeLocator{provider: provider}
	store := newTestStore(locator)
	worker := newTestWorker(t, store, locator)
	storageID := createStorageRow(t)
	unpublished := StoredFileReference{StorageID: storageID, FileName: "unpublished"}
	published := StoredFileReference{StorageID: storageID, FileName: "published"}

	if _, err := store.WriteFile(t.Context(), unpublished, strings.NewReader("payload")); err != nil {
		require.NoError(t, err)
	}

	publishedReceipt, err := store.WriteFile(t.Context(), published, strings.NewReader("payload"))
	require.NoError(t, err)

	require.NoError(t, inTransaction(t, func(tx *gorm.DB) error {
		return store.ConfirmFileWrites(t.Context(), tx, []WriteReceipt{publishedReceipt})
	}))

	worker.RunOnce(t.Context())
	assert.Empty(t, provider.deleted, "a file inside its commit window is not cleanup's yet")

	time.Sleep(store.GetTimings().CommitWindow)

	require.NoError(t, worker.DrainForTest(t.Context(), unpublished))

	assert.Equal(t, []string{"unpublished"}, provider.deleted,
		"the published file carries no obligation, so nothing can delete it")
}
