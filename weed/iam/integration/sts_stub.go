package integration

import (
	"context"
	"fmt"

	"github.com/seaweedfs/seaweedfs/weed/glog"
	"github.com/seaweedfs/seaweedfs/weed/iam/providers"
)

// StubSTSAdapter is a stub implementation of STSAdapter
// Returns "not implemented" errors for all STS operations
// This allows IAM to build and run without STS being present in PR2
type StubSTSAdapter struct{}

// NewStubSTSAdapter creates a new stub STS adapter
func NewStubSTSAdapter() *StubSTSAdapter {
	glog.V(1).Infof("Using stub STS adapter - STS functionality not available in this build")
	return &StubSTSAdapter{}
}

// AssumeRoleWithCredentials returns not implemented error
func (s *StubSTSAdapter) AssumeRoleWithCredentials(ctx context.Context, req *AssumeRoleRequest) (*AssumeRoleResponse, error) {
	return nil, fmt.Errorf("STS service not configured - AssumeRole will be available in a future release")
}

// AssumeRoleWithWebIdentity returns not implemented error
func (s *StubSTSAdapter) AssumeRoleWithWebIdentity(ctx context.Context, req *AssumeRoleWebIdentityRequest) (*AssumeRoleResponse, error) {
	return nil, fmt.Errorf("STS service not configured - AssumeRoleWithWebIdentity will be available in a future release")
}

// ValidateSessionToken returns not implemented error
func (s *StubSTSAdapter) ValidateSessionToken(ctx context.Context, token string) (*SessionInfo, error) {
	return nil, fmt.Errorf("STS service not configured - session token validation will be available in a future release")
}

// RegisterProvider returns not implemented error
func (s *StubSTSAdapter) RegisterProvider(provider providers.IdentityProvider) error {
	glog.V(2).Infof("Stub STS: RegisterProvider called for %s (no-op in Core IAM build)", provider.Name())
	// Don't return error - just log that provider registration is a no-op
	// This allows IAM configuration to load without failing
	return nil
}

// GetCredentialsForSession returns not implemented error
func (s *StubSTSAdapter) GetCredentialsForSession(ctx context.Context, sessionToken string) (*SessionCredentials, error) {
	return nil, fmt.Errorf("STS service not configured - credential lookup will be available in a future release")
}
