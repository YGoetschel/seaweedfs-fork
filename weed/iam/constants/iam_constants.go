package constants

// IAM API Constants
const (
	// IAMAPIDomainName is the domain name for IAM API requests
	IamAPIDomainName = "iam.amazonaws.com"
	
	// IamAPIVersion is the API version
	IamAPIVersion = "2010-05-08"
	
	// Action permissions
	PermissionRead  = "Read"
	PermissionWrite = "Write"
	PermissionAdmin = "Admin"
	PermissionList  = "List"
	
	// Common HTTP headers
	HeaderContentType = "Content-Type"
	HeaderAccept      = "Accept"
	
	// Content types
	ContentTypeXML  = "application/xml"
	ContentTypeJSON = "application/json"
	ContentTypeForm = "application/x-www-form-urlencoded"
)
