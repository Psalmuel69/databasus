package storages

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	s3_storage "databasus-backend/internal/features/storages/models/s3"
	"databasus-backend/internal/util/encryption"
	"databasus-backend/internal/util/logger"
	"databasus-backend/internal/util/testing/containers"
)

func startS3CleanupTarget(t *testing.T) (*s3_storage.S3Storage, *minio.Client, string) {
	t.Helper()

	endpoint := containers.StartMinio(t)
	address := addressOf(endpoint)

	container, err := setupS3Container(t.Context(), address)
	require.NoError(t, err, "failed to set up the S3 container")

	storage := &s3_storage.S3Storage{
		StorageID:   uuid.New(),
		S3Bucket:    container.bucketName,
		S3Region:    container.region,
		S3AccessKey: container.accessKey,
		S3SecretKey: container.secretKey,
		S3Endpoint:  "http://" + address,
	}

	client, err := minio.New(address, &minio.Options{
		Creds:  credentials.NewStaticV4(container.accessKey, container.secretKey, ""),
		Secure: false,
		Region: container.region,
	})
	require.NoError(t, err)

	return storage, client, container.bucketName
}

func objectExists(t *testing.T, client *minio.Client, bucket, key string) bool {
	t.Helper()

	_, err := client.StatObject(t.Context(), bucket, key, minio.StatObjectOptions{})

	return err == nil
}

func putObject(t *testing.T, client *minio.Client, bucket, key, body string) {
	t.Helper()

	_, err := client.PutObject(
		t.Context(), bucket, key, strings.NewReader(body), int64(len(body)), minio.PutObjectOptions{},
	)
	require.NoError(t, err)
}

func deleteThroughStorage(t *testing.T, storage *s3_storage.S3Storage, fileName string) error {
	t.Helper()

	return storage.DeleteFile(t.Context(), encryption.GetFieldEncryptor(), logger.GetLogger(), fileName)
}

func Test_S3DeleteFile_WhenChunksHaveNoManifest_RemovesThemAnyway(t *testing.T) {
	storage, client, bucket := startS3CleanupTarget(t)
	fileName := "orphan-chunks"

	for index := 1; index <= 3; index++ {
		putObject(t, client, bucket, fmt.Sprintf("%s.part%06d", fileName, index), "chunk")
	}

	require.NoError(t, deleteThroughStorage(t, storage, fileName))

	for index := 1; index <= 3; index++ {
		key := fmt.Sprintf("%s.part%06d", fileName, index)
		assert.False(t, objectExists(t, client, bucket, key), "chunk %s survived cleanup", key)
	}
}

func Test_S3DeleteFile_WhenNeighbourSharesThePrefix_LeavesItAlone(t *testing.T) {
	storage, client, bucket := startS3CleanupTarget(t)

	putObject(t, client, bucket, "backup-1", "one")
	putObject(t, client, bucket, "backup-1.part000001", "one chunk")
	putObject(t, client, bucket, "backup-10", "ten")
	putObject(t, client, bucket, "backup-10.part000001", "ten chunk")
	putObject(t, client, bucket, "backup-1.metadata", "sidecar")

	require.NoError(t, deleteThroughStorage(t, storage, "backup-1"))

	assert.False(t, objectExists(t, client, bucket, "backup-1"))
	assert.False(t, objectExists(t, client, bucket, "backup-1.part000001"))
	assert.True(t, objectExists(t, client, bucket, "backup-10"), "a similarly named object must survive")
	assert.True(t, objectExists(t, client, bucket, "backup-10.part000001"))
	assert.True(t, objectExists(t, client, bucket, "backup-1.metadata"), "a sidecar is its own logical file")
}

func Test_S3DeleteFile_WhenUploadWasInterrupted_AbortsTheMultipartUpload(t *testing.T) {
	storage, client, bucket := startS3CleanupTarget(t)
	fileName := "interrupted-upload"

	coreClient := minio.Core{Client: client}

	uploadID, err := coreClient.NewMultipartUpload(t.Context(), bucket, fileName, minio.PutObjectOptions{})
	require.NoError(t, err)

	_, err = coreClient.PutObjectPart(
		t.Context(), bucket, fileName, uploadID, 1,
		bytes.NewReader(make([]byte, 5*1024*1024)), 5*1024*1024, minio.PutObjectPartOptions{},
	)
	require.NoError(t, err)

	require.True(t, hasIncompleteUpload(t, client, bucket, fileName))

	require.NoError(t, deleteThroughStorage(t, storage, fileName))

	assert.False(t, hasIncompleteUpload(t, client, bucket, fileName),
		"an interrupted upload keeps consuming storage until it is aborted")
}

func Test_S3DeleteFile_WhenCalledTwice_Succeeds(t *testing.T) {
	storage, client, bucket := startS3CleanupTarget(t)
	fileName := "twice"

	putObject(t, client, bucket, fileName, "payload")

	require.NoError(t, deleteThroughStorage(t, storage, fileName))
	assert.NoError(t, deleteThroughStorage(t, storage, fileName))
}

func Test_S3DeleteFile_WhenChunkedFileWasPublished_RemovesPartsAndManifest(t *testing.T) {
	storage, client, bucket := startS3CleanupTarget(t)
	fileName := "chunked-" + uuid.New().String()

	payload := strings.Repeat("x", 6*1024*1024)
	require.NoError(t, storage.SaveFile(
		t.Context(), encryption.GetFieldEncryptor(), logger.GetLogger(), fileName, strings.NewReader(payload),
	))

	require.NoError(t, deleteThroughStorage(t, storage, fileName))

	assert.False(t, objectExists(t, client, bucket, fileName))
	assert.False(t, objectExists(t, client, bucket, fileName+".parts"))
	assert.False(t, objectExists(t, client, bucket, fileName+".part000001"))
}

func hasIncompleteUpload(t *testing.T, client *minio.Client, bucket, key string) bool {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for upload := range client.ListIncompleteUploads(ctx, bucket, key, true) {
		require.NoError(t, upload.Err)

		if upload.Key == key {
			return true
		}
	}

	return false
}
