package errors

import (
	"encoding/xml"
	"net/http"
)

// IAMErrorCode represents IAM-specific error codes
// Extracted from S3API to decouple IAM from S3 dependencies
type ErrorCode int

const (
	ErrNone ErrorCode = iota
	ErrAccessDenied
	ErrInvalidRequest
	ErrInternalError
	ErrMalformedXML
	ErrMalformedPolicy
	ErrInvalidPolicyDocument
	ErrNotImplemented
	ErrInvalidAccessKeyID
	ErrNoSuchEntity
	ErrEntityAlreadyExists
	ErrLimitExceeded
	ErrValidationError
	ErrInvalidParameter
	ErrMissingParameter
)

// APIError structure
type APIError struct {
	Code           string
	Description    string
	HTTPStatusCode int
}

// IAMErrorResponse - IAM API error response format
type IAMErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Error     IAMError `xml:"Error"`
	RequestID string   `xml:"RequestId"`
}

type IAMError struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// errorCodeResponse maps error codes to API errors
var errorCodeResponse = map[ErrorCode]APIError{
	ErrAccessDenied: {
		Code:           "AccessDenied",
		Description:    "Access Denied.",
		HTTPStatusCode: http.StatusForbidden,
	},
	ErrInvalidRequest: {
		Code:           "InvalidRequest",
		Description:    "Invalid Request",
		HTTPStatusCode: http.StatusBadRequest,
	},
	ErrInternalError: {
		Code:           "InternalError",
		Description:    "We encountered an internal error, please try again.",
		HTTPStatusCode: http.StatusInternalServerError,
	},
	ErrMalformedXML: {
		Code:           "MalformedXML",
		Description:    "The XML you provided was not well-formed or did not validate against our published schema.",
		HTTPStatusCode: http.StatusBadRequest,
	},
	ErrMalformedPolicy: {
		Code:           "MalformedPolicy",
		Description:    "Policy has invalid resource.",
		HTTPStatusCode: http.StatusBadRequest,
	},
	ErrInvalidPolicyDocument: {
		Code:           "InvalidPolicyDocument",
		Description:    "The content of the policy document is invalid.",
		HTTPStatusCode: http.StatusBadRequest,
	},
	ErrNotImplemented: {
		Code:           "NotImplemented",
		Description:    "A header you provided implies functionality that is not implemented",
		HTTPStatusCode: http.StatusNotImplemented,
	},
	ErrInvalidAccessKeyID: {
		Code:           "InvalidAccessKeyId",
		Description:    "The access key ID you provided does not exist in our records.",
		HTTPStatusCode: http.StatusForbidden,
	},
	ErrNoSuchEntity: {
		Code:           "NoSuchEntity",
		Description:    "The specified entity does not exist.",
		HTTPStatusCode: http.StatusNotFound,
	},
	ErrEntityAlreadyExists: {
		Code:           "EntityAlreadyExists",
		Description:    "The specified entity already exists.",
		HTTPStatusCode: http.StatusConflict,
	},
	ErrLimitExceeded: {
		Code:           "LimitExceeded",
		Description:    "The request was rejected because it attempted to create resources beyond the current account limits.",
		HTTPStatusCode: http.StatusConflict,
	},
	ErrValidationError: {
		Code:           "ValidationError",
		Description:    "The input fails to satisfy the constraints specified by IAM.",
		HTTPStatusCode: http.StatusBadRequest,
	},
	ErrInvalidParameter: {
		Code:           "InvalidParameterValue",
		Description:    "An invalid or out-of-range value was supplied for the input parameter.",
		HTTPStatusCode: http.StatusBadRequest,
	},
	ErrMissingParameter: {
		Code:           "MissingParameter",
		Description:    "A required parameter for the specified action is not supplied.",
		HTTPStatusCode: http.StatusBadRequest,
	},
}

// GetAPIError provides API Error for input API error code.
func GetAPIError(code ErrorCode) APIError {
	return errorCodeResponse[code]
}

// GetErrorCode returns the error code as a string
func (e ErrorCode) Error() string {
	apiError, ok := errorCodeResponse[e]
	if !ok {
		return "Unknown error"
	}
	return apiError.Description
}

// HTTPStatusCode returns the HTTP status code for this error
func (e ErrorCode) HTTPStatusCode() int {
	apiError, ok := errorCodeResponse[e]
	if !ok {
		return http.StatusInternalServerError
	}
	return apiError.HTTPStatusCode
}

// WriteXMLResponse writes an IAM error response as XML
func WriteXMLResponse(w http.ResponseWriter, r *http.Request, statusCode int, response interface{}) {
w.Header().Set("Content-Type", "application/xml")
w.WriteHeader(statusCode)
xml.NewEncoder(w).Encode(response)
}

// NotFoundHandler handles 404 errors for IAM API
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	errorResponse := IAMErrorResponse{
		Error: IAMError{
			Type:    "Sender",
			Code:    "NoSuchEntity",
			Message: "The resource you requested does not exist.",
		},
		RequestID: r.Header.Get("X-Request-ID"),
	}
	WriteXMLResponse(w, r, http.StatusNotFound, errorResponse)
}

