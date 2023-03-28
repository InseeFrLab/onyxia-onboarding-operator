package factory

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/madmin-go/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioS3Client struct {
	s3Config    S3Config
	client      minio.Client
	adminClient madmin.AdminClient
}

func (minioS3Client *MinioS3Client) BucketExists(name string) (bool, error) {
	log.Println("check if bucket " + name + "exists")
	return minioS3Client.client.BucketExists(context.Background(), name)
}

func (minioS3Client *MinioS3Client) GetQuota(name string) (int32, error) {
	log.Println("bucket " + name + " get quota")
	bucketQuota, err := minioS3Client.adminClient.GetBucketQuota(context.Background(), name)
	if err != nil {
		log.Fatalln(err)
	}
	return int32(bucketQuota.Quota), err
}

func (minioS3Client *MinioS3Client) CreateBucket(name string) error {
	log.Println("create bucket " + name + "exists")
	return minioS3Client.client.MakeBucket(context.Background(), name, minio.MakeBucketOptions{Region: minioS3Client.s3Config.Region})
}

func (minioS3Client *MinioS3Client) DeleteBucket(name string) error {
	log.Println("delete bucket " + name + "exists")
	return minioS3Client.client.RemoveBucket(context.Background(), name)
}

func (minioS3Client *MinioS3Client) SetQuota(name string, quota int32) error {
	log.Println("set quota " + fmt.Sprint(quota) + "on bucket " + name + "exists")
	minioS3Client.adminClient.SetBucketQuota(context.Background(), name, &madmin.BucketQuota{Quota: uint64(quota), Type: madmin.HardQuota})
	return nil
}

func newMinioS3Client(S3Config *S3Config) *MinioS3Client {
	log.Println("create minio clients")
	minioClient, err := minio.New(S3Config.S3UrlEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(S3Config.AccessKey, S3Config.SecretKey, ""),
		Secure: S3Config.UseSsl,
	})
	if err != nil {
		log.Fatalln(err)
	}

	adminClient, err := madmin.New(S3Config.S3UrlEndpoint, S3Config.AccessKey, S3Config.SecretKey, S3Config.UseSsl)
	if err != nil {
		log.Fatalln(err)
	}

	return &MinioS3Client{*S3Config, *minioClient, *adminClient}
}
