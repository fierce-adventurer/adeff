package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	MinioClient *minio.Client
	BucketName  string
)

// establish connection to MinIO server
func InitMinio() {
	endpoint := os.Getenv("S3_ENDPOINT")
	accessKeyId := os.Getenv("S3_ACCESS_KEY")
	secretAccessKey := os.Getenv("S3_SECRET_KEY")
	BucketName = os.Getenv("S3_BUCKET_NAME")
	useSSLStr := os.Getenv("S3_USE_SSL")

	useSSL, _ := strconv.ParseBool(useSSLStr)

	// initialize minio client object.
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyId, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize MinIO client: %v", err)
	}
	MinioClient = client

	// check if bucket exists create if doesnt exist

	ctx := context.Background()
	exists, errBucketExists := MinioClient.BucketExists(ctx, BucketName)
	if errBucketExists == nil && exists {
		log.Printf("[Storage] MinIO bucket '%s' already exists", BucketName)
	} else {
		err = client.MakeBucket(ctx, BucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatalf("FATAL: Failed to create bucket: %v", err)
		}
		log.Printf("[Storage] MinIO bucket '%s' created successfully", BucketName)
	}
}

func UploadStream(ctx context.Context, objectName string, reader io.Reader, objectSize int64, contentType string) error {
	log.Printf("[Storage] Uploading object '%s' |Size : '%d bytes'", objectName, objectSize)

	info, err := MinioClient.PutObject(ctx, BucketName, objectName, reader, objectSize, minio.PutObjectOptions{
		ContentType: contentType,
	})

	if err != nil {
		log.Printf("[Storage] ERROR: MinIO upload failed for %s: %v", objectName, err)
		return err
	}
	log.Printf("[Storage] Upload successful -> Key: %s | ETag: %s", objectName, info.ETag)
	return nil
}

func DownloadFile(ctx context.Context, objectName string) ([]byte, error) {
	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	log.Printf("[Storage] Downloading object '%s'", objectName)

	if bucketName == "" {
		return nil, fmt.Errorf("MINIO_BUCKET_NAME environment variable is not set")
	}

	// get object from MinIO
	object, err := MinioClient.GetObject(ctx, BucketName, objectName, minio.GetObjectOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to get object %s from bucket %s: %w", objectName, bucketName, err)
	}
	defer object.Close()

	// Read the stream into a byte slice
	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from MinIO object: %w", err)
	}

	return data, nil
}
