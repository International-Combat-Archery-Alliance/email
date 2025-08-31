package email

import "fmt"

type ErrorReason string

const (
	REASON_UNKNOWN           ErrorReason = "UNKNOWN_ERROR"
	REASON_RATE_LIMITED      ErrorReason = "RATE_LIMITED"
	REASON_INVALID_EMAIL     ErrorReason = "INVALID_EMAIL"
	REASON_UNVERIFIED_DOMAIN ErrorReason = "UNVERIFIED_DOMAIN"
	REASON_MESSAGE_REJECTED  ErrorReason = "MESSAGE_REJECTED"
	REASON_SERVICE_ERROR     ErrorReason = "SERVICE_ERROR"
	REASON_VALIDATION_ERROR  ErrorReason = "VALIDATION_ERROR"
)

var _ error = &Error{}

type Error struct {
	Message string
	Reason  ErrorReason
	Cause   error
}

func (e *Error) Error() string {
	s := fmt.Sprintf("%s: %s.", e.Reason, e.Message)
	if e.Cause != nil {
		s += fmt.Sprintf(" Cause: %s", e.Cause)
	}
	return s
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func newError(reason ErrorReason, message string, cause error) *Error {
	return &Error{
		Message: message,
		Reason:  reason,
		Cause:   cause,
	}
}

func NewUnknownError(message string, cause error) *Error {
	return newError(REASON_UNKNOWN, message, cause)
}

func NewRateLimitedError(message string, cause error) *Error {
	return newError(REASON_RATE_LIMITED, message, cause)
}

func NewInvalidEmailError(message string, cause error) *Error {
	return newError(REASON_INVALID_EMAIL, message, cause)
}

func NewUnverifiedDomainError(message string, cause error) *Error {
	return newError(REASON_UNVERIFIED_DOMAIN, message, cause)
}

func NewMessageRejectedError(message string, cause error) *Error {
	return newError(REASON_MESSAGE_REJECTED, message, cause)
}

func NewServiceError(message string, cause error) *Error {
	return newError(REASON_SERVICE_ERROR, message, cause)
}

func NewValidationError(message string, cause error) *Error {
	return newError(REASON_VALIDATION_ERROR, message, cause)
}
