package integration

import (
	"context"
	"time"

	"github.com/seaweedfs/seaweedfs/weed/iam/providers"
)

// STSAdapter defines the interface for STS (Security Token Service) operations
// This allows IAM Manager to work with or without STS being present
// In PR2 (Core IAM), a stub implementation is used
// In PR3 (STS Service), the real STS service implements this interface
type STSAdapter interface {
	// AssumeRoleWithCredentials generates temporary credentials for a role
	// using authenticated user credentials
	AssumeRoleWithCredentials(ctx context.Context, req *AssumeRoleRequest) (*AssumeRoleResponse, error)

	// AssumeRoleWithWebIdentity generates temporary credentials for a role
	// using a web identity token (OIDC/SAML)
	AssumeRoleWithWebIdentity(ctx context.Context, req *AssumeRoleWebIdentityRequest) (*AssumeRoleResponse, error)

	// ValidateSessionToken validates a session token and returns session info
	ValidateSessionToken(ctx context.Context, token string) (*SessionInfo, error)

	// RegisterProvider registers an identity provider (OIDC, LDAP, SAML)
	RegisterProvider(provider providers.IdentityProvider) error

	// GetCredentialsForSession retrieves credentials associated with a session token
	GetCredentialsForSession(ctx context.Context, sessionToken string) (*SessionCredentials, error)
}

// AssumeRoleRequest represents a request to assume a role with credentials
// These types are IAM-owned (not STS-owned) to maintain decoupling
type AssumeRoleRequest struct {
	RoleArn         string
	Identity        *providers.ExternalIdentity
	DurationSeconds *int64
	SessionName     string
	Policy          *string // Optional session policy
}

// AssumeRoleWebIdentityRequest represents a request to assume a role with web identity
type AssumeRoleWebIdentityRequest struct {
	RoleArn          string
	WebIdentityToken string
	SessionName      string
	DurationSeconds  *int64
	Policy           *string // Optional session policy
}

// AssumeRoleResponse contains the temporary credentials returned by STS
type AssumeRoleResponse struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
	AssumedRoleARN  string
}

// SessionInfo contains information about a validated session
type SessionInfo struct {
	RoleArn    string
	Principal  string
	Expiration time.Time
	SessionID  string
}

// SessionCredentials represents credentials associated with a session
type SessionCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}
