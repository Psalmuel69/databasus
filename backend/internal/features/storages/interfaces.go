package storages

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"

	"databasus-backend/internal/util/encryption"
)

type StorageFileSaver interface {
	SaveFile(
		ctx context.Context,
		encryptor encryption.FieldEncryptor,
		logger *slog.Logger,
		fileName string,
		file io.Reader,
	) error

	// The returned reader may keep fetching from the remote as it is consumed, so ctx must span the
	// lifetime of the stream, not just the call: a caller that cancels ctx on return gets a stream
	// that dies on the first read.
	GetFile(
		ctx context.Context,
		encryptor encryption.FieldEncryptor,
		logger *slog.Logger,
		fileName string,
	) (io.ReadCloser, error)

	DeleteFile(
		ctx context.Context,
		encryptor encryption.FieldEncryptor,
		logger *slog.Logger,
		fileName string,
	) error

	Validate(encryptor encryption.FieldEncryptor) error

	TestConnection(encryptor encryption.FieldEncryptor) error

	HideSensitiveData()

	EncryptSensitiveData(encryptor encryption.FieldEncryptor) error
}

type StorageDatabaseCounter interface {
	GetStorageAttachedDatabasesIDs(storageID uuid.UUID) ([]uuid.UUID, error)
}

// Backup rows are the only record of the file names in a storage, so removing the
// storage while they exist would make the objects unreachable. Each backup feature
// counts its own rows, because the tables belong to it.
type StorageBackupCounter interface {
	GetStorageBackupReferenceCount(storageID uuid.UUID) (int64, error)
}
