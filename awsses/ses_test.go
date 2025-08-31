package awsses

import (
	"context"
	"errors"
	"testing"

	"github.com/International-Combat-Archery-Alliance/email"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/smithy-go"
)

// Mock SES client for testing
type mockSESClient struct {
	sendEmailFunc func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

func (m *mockSESClient) SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	if m.sendEmailFunc != nil {
		return m.sendEmailFunc(ctx, params, optFns...)
	}
	return &sesv2.SendEmailOutput{}, nil
}

func TestSendEmail_Success(t *testing.T) {
	tests := []struct {
		name  string
		email email.Email
	}{
		{
			name: "valid email with HTML and text body",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				HTMLBody:    "<h1>Hello World</h1>",
				TextBody:    "Hello World",
			},
		},
		{
			name: "valid email with only HTML body",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				HTMLBody:    "<h1>Hello World</h1>",
			},
		},
		{
			name: "valid email with only text body",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				TextBody:    "Hello World",
			},
		},
		{
			name: "valid email with CC and BCC",
			email: email.Email{
				FromAddress:  "sender@example.com",
				ToAddresses:  []string{"recipient@example.com"},
				CCAddresses:  []string{"cc@example.com"},
				BCCAddresses: []string{"bcc@example.com"},
				Subject:      "Test Subject",
				HTMLBody:     "<h1>Hello World</h1>",
			},
		},
		{
			name: "valid email with reply-to addresses",
			email: email.Email{
				FromAddress:      "sender@example.com",
				ToAddresses:      []string{"recipient@example.com"},
				ReplyToAddresses: []string{"replyto@example.com"},
				Subject:          "Test Subject",
				TextBody:         "Hello World",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSESClient{
				sendEmailFunc: func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
					// Verify the request is constructed correctly
					if params.FromEmailAddress == nil || *params.FromEmailAddress != tt.email.FromAddress {
						t.Errorf("expected FromEmailAddress %s, got %v", tt.email.FromAddress, params.FromEmailAddress)
					}
					if len(params.Destination.ToAddresses) != len(tt.email.ToAddresses) {
						t.Errorf("expected %d ToAddresses, got %d", len(tt.email.ToAddresses), len(params.Destination.ToAddresses))
					}
					if params.Content.Simple.Subject.Data == nil || *params.Content.Simple.Subject.Data != tt.email.Subject {
						t.Errorf("expected Subject %s, got %v", tt.email.Subject, params.Content.Simple.Subject.Data)
					}

					return &sesv2.SendEmailOutput{}, nil
				},
			}

			sender := NewAWSSESSender(client)
			err := sender.SendEmail(context.Background(), tt.email)

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestSendEmail_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		email         email.Email
		expectedError email.ErrorReason
	}{
		{
			name: "missing from address",
			email: email.Email{
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test",
				TextBody:    "Hello",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
		{
			name: "invalid from address format",
			email: email.Email{
				FromAddress: "invalid-email",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test",
				TextBody:    "Hello",
			},
			expectedError: email.REASON_INVALID_EMAIL,
		},
		{
			name: "no recipients",
			email: email.Email{
				FromAddress: "sender@example.com",
				Subject:     "Test",
				TextBody:    "Hello",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
		{
			name: "invalid recipient address",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"invalid-email"},
				Subject:     "Test",
				TextBody:    "Hello",
			},
			expectedError: email.REASON_INVALID_EMAIL,
		},
		{
			name: "missing subject",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				TextBody:    "Hello",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
		{
			name: "missing body",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSESClient{}
			sender := NewAWSSESSender(client)

			err := sender.SendEmail(context.Background(), tt.email)

			if err == nil {
				t.Error("expected validation error, got nil")
				return
			}

			var emailErr *email.Error
			if !errors.As(err, &emailErr) {
				t.Errorf("expected email.Error, got %T", err)
				return
			}

			if emailErr.Reason != tt.expectedError {
				t.Errorf("expected error reason %s, got %s", tt.expectedError, emailErr.Reason)
			}
		})
	}
}

func TestSendEmail_AWSErrors(t *testing.T) {
	tests := []struct {
		name          string
		awsError      error
		expectedError email.ErrorReason
	}{
		{
			name: "rate limited error",
			awsError: &smithy.GenericAPIError{
				Code:    "TooManyRequestsException",
				Message: "Rate limit exceeded",
			},
			expectedError: email.REASON_RATE_LIMITED,
		},
		{
			name: "message rejected error",
			awsError: &smithy.GenericAPIError{
				Code:    "MessageRejected",
				Message: "Message rejected",
			},
			expectedError: email.REASON_MESSAGE_REJECTED,
		},
		{
			name: "unverified domain error",
			awsError: &smithy.GenericAPIError{
				Code:    "MailFromDomainNotVerifiedException",
				Message: "Domain not verified",
			},
			expectedError: email.REASON_UNVERIFIED_DOMAIN,
		},
		{
			name: "invalid parameter error",
			awsError: &smithy.GenericAPIError{
				Code:    "InvalidParameterValueException",
				Message: "Invalid parameter",
			},
			expectedError: email.REASON_INVALID_EMAIL,
		},
		{
			name: "service unavailable error",
			awsError: &smithy.GenericAPIError{
				Code:    "ServiceUnavailableException",
				Message: "Service unavailable",
			},
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name: "internal service error",
			awsError: &smithy.GenericAPIError{
				Code:    "InternalServiceErrorException",
				Message: "Internal error",
			},
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name: "unknown aws error",
			awsError: &smithy.GenericAPIError{
				Code:    "UnknownException",
				Message: "Unknown error",
			},
			expectedError: email.REASON_UNKNOWN,
		},
		{
			name:          "non-aws error",
			awsError:      errors.New("network error"),
			expectedError: email.REASON_UNKNOWN,
		},
	}

	validEmail := email.Email{
		FromAddress: "sender@example.com",
		ToAddresses: []string{"recipient@example.com"},
		Subject:     "Test Subject",
		TextBody:    "Hello World",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSESClient{
				sendEmailFunc: func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
					return nil, tt.awsError
				},
			}

			sender := NewAWSSESSender(client)
			err := sender.SendEmail(context.Background(), validEmail)

			if err == nil {
				t.Error("expected AWS error, got nil")
				return
			}

			var emailErr *email.Error
			if !errors.As(err, &emailErr) {
				t.Errorf("expected email.Error, got %T", err)
				return
			}

			if emailErr.Reason != tt.expectedError {
				t.Errorf("expected error reason %s, got %s", tt.expectedError, emailErr.Reason)
			}
		})
	}
}
