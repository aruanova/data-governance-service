package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode represents a unique error code for each error type
type ErrorCode string

const (
	// General errors
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeBadRequest   ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeConflict     ErrorCode = "CONFLICT"

	// File processing errors
	ErrCodeInvalidFile      ErrorCode = "INVALID_FILE"
	ErrCodeFileTooLarge     ErrorCode = "FILE_TOO_LARGE"
	ErrCodeUnsupportedFormat ErrorCode = "UNSUPPORTED_FORMAT"
	ErrCodeFileParseError   ErrorCode = "FILE_PARSE_ERROR"

	// LLM errors
	ErrCodeLLMRequestFailed ErrorCode = "LLM_REQUEST_FAILED"
	ErrCodeLLMInvalidResponse ErrorCode = "LLM_INVALID_RESPONSE"
	ErrCodeLLMRateLimited   ErrorCode = "LLM_RATE_LIMITED"

	// Database errors
	ErrCodeDatabaseError    ErrorCode = "DATABASE_ERROR"
	ErrCodeRecordNotFound   ErrorCode = "RECORD_NOT_FOUND"
	ErrCodeDuplicateRecord  ErrorCode = "DUPLICATE_RECORD"

	// Queue errors
	ErrCodeQueueError       ErrorCode = "QUEUE_ERROR"
	ErrCodeTaskNotFound     ErrorCode = "TASK_NOT_FOUND"
)

// AppError represents a structured application error
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	StatusCode int                    `json:"-"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Err        error                  `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s - %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements the errors.Unwrap interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithDetails adds additional context to the error
func (e *AppError) WithDetails(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// New creates a new AppError
func New(code ErrorCode, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// Wrap wraps an existing error with AppError context
func Wrap(err error, code ErrorCode, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
	}
}

// Common error constructors

func Internal(message string) *AppError {
	return New(ErrCodeInternal, message, http.StatusInternalServerError)
}

func InternalWrap(err error, message string) *AppError {
	return Wrap(err, ErrCodeInternal, message, http.StatusInternalServerError)
}

func NotFound(message string) *AppError {
	return New(ErrCodeNotFound, message, http.StatusNotFound)
}

func BadRequest(message string) *AppError {
	return New(ErrCodeBadRequest, message, http.StatusBadRequest)
}

func Unauthorized(message string) *AppError {
	return New(ErrCodeUnauthorized, message, http.StatusUnauthorized)
}

func Forbidden(message string) *AppError {
	return New(ErrCodeForbidden, message, http.StatusForbidden)
}

func Conflict(message string) *AppError {
	return New(ErrCodeConflict, message, http.StatusConflict)
}

// File processing errors

func InvalidFile(message string) *AppError {
	return New(ErrCodeInvalidFile, message, http.StatusBadRequest)
}

func FileTooLarge(maxSize int64) *AppError {
	return New(ErrCodeFileTooLarge,
		fmt.Sprintf("file size exceeds maximum allowed size of %d MB", maxSize),
		http.StatusBadRequest)
}

func UnsupportedFormat(format string) *AppError {
	return New(ErrCodeUnsupportedFormat,
		fmt.Sprintf("unsupported file format: %s", format),
		http.StatusBadRequest)
}

// LLM errors

func LLMRequestFailed(err error) *AppError {
	return Wrap(err, ErrCodeLLMRequestFailed, "LLM request failed", http.StatusInternalServerError)
}

func LLMInvalidResponse(message string) *AppError {
	return New(ErrCodeLLMInvalidResponse, message, http.StatusInternalServerError)
}

// Database errors

func DatabaseError(err error) *AppError {
	return Wrap(err, ErrCodeDatabaseError, "database operation failed", http.StatusInternalServerError)
}

func RecordNotFound(resource string) *AppError {
	return New(ErrCodeRecordNotFound,
		fmt.Sprintf("%s not found", resource),
		http.StatusNotFound)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// GetAppError extracts AppError from error chain
func GetAppError(err error) (*AppError, bool) {
	var appErr *AppError
	ok := errors.As(err, &appErr)
	return appErr, ok
}