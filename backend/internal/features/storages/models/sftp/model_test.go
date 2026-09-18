package sftp_storage

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
	"databasus-backend/internal/util/testing/containers"
)

func startSftpStorage(t *testing.T) *SFTPStorage {
	t.Helper()

	endpoint := containers.StartSftp(t)

	return &SFTPStorage{
		StorageID:         uuid.New(),
		Host:              endpoint.Host,
		Port:              endpoint.Port,
		Username:          containers.SftpUsername,
		Password:          containers.SftpPassword,
		SkipHostKeyVerify: true,
		Path:              containers.SftpUploadDir,
	}
}

// A non-empty directory is the one refusal the provider's own API can set up.
func Test_DeleteFile_WhenTheServerRefusesTheRemoval_ReturnsTheError(t *testing.T) {
	storage := startSftpStorage(t)
	encryptor := encryption.GetFieldEncryptor()
	directory := "refused-" + uuid.New().String()

	client, sshConn, err := storage.connectWithContext(t.Context(), encryptor, 30*time.Second)
	require.NoError(t, err)

	require.NoError(t, client.Mkdir(storage.getFilePath(directory)))

	t.Cleanup(func() {
		_ = client.Close()
		_ = sshConn.Close()
	})

	require.NoError(t, storage.SaveFile(
		t.Context(), encryptor, logger.GetLogger(), directory+"/artifact", strings.NewReader("payload"),
	))

	err = storage.DeleteFile(t.Context(), encryptor, logger.GetLogger(), directory)

	assert.Error(t, err, "a removal the server refuses must not count as a completed deletion")
}
