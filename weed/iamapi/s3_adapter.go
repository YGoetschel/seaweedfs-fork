package iamapi

import (
	"context"
	"fmt"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/iam_pb"
)

// S3IdentityManager defines the minimal interface iamapi needs from S3
// In PR2, this is implemented by a stub
// In PR4 (S3 Integration), this is implemented by s3api.IdentityAccessManagement
type S3IdentityManager interface {
	// Auth returns credentials for a given access key
	Auth(f *filer.Filer, action string, bucket string, objectKey string, allowAnonymous bool) (*iam_pb.S3ApiConfiguration, *Credential, error)

	// LoadS3ApiConfigurationFromCredentialManager loads S3 configuration
	LoadS3ApiConfigurationFromCredentialManager(ctx context.Context, option *S3ApiServerOption) error
}

// S3ApiServerOption mimics the options needed from s3api
type S3ApiServerOption struct {
	Filer                    *filer.Filer
	Port                     int
	DomainName               string
	BucketsPath              string
	AllowDeleteBucketNotEmpty bool
	AllowAnonymous            bool
	LocalFilerSocket         string
}

// Credential represents an S3 credential
type Credential struct {
	AccessKey string
	SecretKey string
}

// StubS3IdentityManager is a stub implementation for PR2
// This allows iamapi to build without s3api being present
type StubS3IdentityManager struct{}

// NewStubS3IdentityManager creates a new stub S3 identity manager
func NewStubS3IdentityManager() *StubS3IdentityManager {
	return &StubS3IdentityManager{}
}

// Auth returns an error indicating S3 integration is not available
func (s *StubS3IdentityManager) Auth(f *filer.Filer, action string, bucket string, objectKey string, allowAnonymous bool) (*iam_pb.S3ApiConfiguration, *Credential, error) {
	return nil, nil, fmt.Errorf("S3 integration not available in this build - will be available in next release")
}

// LoadS3ApiConfigurationFromCredentialManager returns an error indicating S3 integration is not available
func (s *StubS3IdentityManager) LoadS3ApiConfigurationFromCredentialManager(ctx context.Context, option *S3ApiServerOption) error {
	return fmt.Errorf("S3 integration not available in this build - will be available in next release")
}
