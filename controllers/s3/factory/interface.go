package factory

import (
	"fmt"
)

type S3Client interface {
	BucketExists(name string) (bool, error)
	CreateBucket(name string) error
	DeleteBucket(name string) error
	SetQuota(name string, quota int32) error
	GetQuota(name string) (int32,error)
}

type S3Config struct {
	S3Provider string
    S3UrlEndpoint  string
	Region  string
	AccessKey string
	SecretKey string
	UseSsl bool
}


func GetS3Client(s3Provider string, S3Config *S3Config) (S3Client, error) {
	if s3Provider == "mockedS3Provider" {
        return newMockedS3Client(), nil
    }
	if s3Provider == "minio" {
		return newMinioS3Client(S3Config), nil
	}
	//todo others
	return nil, fmt.Errorf("s3 provider "+s3Provider+"not supported")
}


