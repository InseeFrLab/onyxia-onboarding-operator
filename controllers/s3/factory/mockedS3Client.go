package factory

import (
	"fmt"
	"log"
)

type MockedS3Client struct{}

func (mockedS3Provider *MockedS3Client) BucketExists(name string) (bool, error) {
	log.Println("check if bucket " + name + "exists")
	return false, nil
}

func (mockedS3Provider *MockedS3Client) GetQuota(name string) (int32, error) {
	log.Println("bucket " + name + " get quota")
	return 1, nil
}

func (mockedS3Provider *MockedS3Client) CreateBucket(name string) error {
	log.Println("create bucket " + name + "exists")
	return nil
}

func (mockedS3Provider *MockedS3Client) DeleteBucket(name string) error {
	log.Println("delete bucket " + name + "exists")
	return nil
}

func (mockedS3Provider *MockedS3Client) SetQuota(name string, quota int32) error {
	log.Println("set quota " + fmt.Sprint(quota) + "on bucket " + name + "exists")
	return nil
}

func newMockedS3Client() *MockedS3Client {
	return &MockedS3Client{}
}
