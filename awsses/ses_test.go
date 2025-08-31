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
		{
			name: "valid email with single attachment",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				TextBody:    "Hello World",
				Attachments: []email.Attachment{
					{
						FileName:    "document.pdf",
						Content:     []byte("fake pdf content"),
						Description: "Test PDF document",
						ContentType: "application/pdf",
					},
				},
			},
		},
		{
			name: "valid email with multiple attachments",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				HTMLBody:    "<h1>Hello World</h1>",
				Attachments: []email.Attachment{
					{
						FileName:    "document.pdf",
						Content:     []byte("fake pdf content"),
						Description: "Test PDF document",
						ContentType: "application/pdf",
					},
					{
						FileName:    "image.jpg",
						Content:     []byte("fake image content"),
						Description: "Test image",
						ContentType: "image/jpeg",
					},
					{
						FileName:    "data.txt",
						Content:     []byte("some text data"),
						Description: "Text file",
						ContentType: "text/plain",
					},
				},
			},
		},
		{
			name: "valid email with empty attachments slice",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				TextBody:    "Hello World",
				Attachments: []email.Attachment{},
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

					// Verify attachments are properly converted
					if len(params.Content.Simple.Attachments) != len(tt.email.Attachments) {
						t.Errorf("expected %d attachments, got %d", len(tt.email.Attachments), len(params.Content.Simple.Attachments))
					}

					for i, attachment := range params.Content.Simple.Attachments {
						if i >= len(tt.email.Attachments) {
							break
						}
						expectedAttachment := tt.email.Attachments[i]

						if attachment.FileName == nil || *attachment.FileName != expectedAttachment.FileName {
							t.Errorf("expected attachment[%d] FileName %s, got %v", i, expectedAttachment.FileName, attachment.FileName)
						}
						if attachment.ContentType == nil || *attachment.ContentType != expectedAttachment.ContentType {
							t.Errorf("expected attachment[%d] ContentType %s, got %v", i, expectedAttachment.ContentType, attachment.ContentType)
						}
						if attachment.ContentDescription == nil || *attachment.ContentDescription != expectedAttachment.Description {
							t.Errorf("expected attachment[%d] ContentDescription %s, got %v", i, expectedAttachment.Description, attachment.ContentDescription)
						}
						if string(attachment.RawContent) != string(expectedAttachment.Content) {
							t.Errorf("expected attachment[%d] RawContent %s, got %s", i, string(expectedAttachment.Content), string(attachment.RawContent))
						}
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

func TestAttachmentConversion(t *testing.T) {
	tests := []struct {
		name        string
		attachments []email.Attachment
	}{
		{
			name: "single attachment conversion",
			attachments: []email.Attachment{
				{
					FileName:    "test.pdf",
					Content:     []byte("test content"),
					Description: "Test document",
					ContentType: "application/pdf",
				},
			},
		},
		{
			name: "multiple attachments conversion",
			attachments: []email.Attachment{
				{
					FileName:    "document.pdf",
					Content:     []byte("pdf content"),
					Description: "PDF document",
					ContentType: "application/pdf",
				},
				{
					FileName:    "spreadsheet.xlsx",
					Content:     []byte("excel content"),
					Description: "Excel spreadsheet",
					ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
				},
				{
					FileName:    "photo.png",
					Content:     []byte("image content"),
					Description: "Photo attachment",
					ContentType: "image/png",
				},
			},
		},
		{
			name:        "empty attachments",
			attachments: []email.Attachment{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testEmail := email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test Subject",
				TextBody:    "Test body",
				Attachments: tt.attachments,
			}

			client := &mockSESClient{
				sendEmailFunc: func(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
					// Verify attachment count
					if len(params.Content.Simple.Attachments) != len(tt.attachments) {
						t.Errorf("expected %d attachments, got %d", len(tt.attachments), len(params.Content.Simple.Attachments))
						return &sesv2.SendEmailOutput{}, nil
					}

					// Verify each attachment is converted correctly
					for i, awsAttachment := range params.Content.Simple.Attachments {
						expectedAttachment := tt.attachments[i]

						if awsAttachment.FileName == nil {
							t.Errorf("attachment[%d] FileName is nil", i)
							continue
						}
						if *awsAttachment.FileName != expectedAttachment.FileName {
							t.Errorf("attachment[%d] FileName: expected %s, got %s", i, expectedAttachment.FileName, *awsAttachment.FileName)
						}

						if awsAttachment.ContentType == nil {
							t.Errorf("attachment[%d] ContentType is nil", i)
							continue
						}
						if *awsAttachment.ContentType != expectedAttachment.ContentType {
							t.Errorf("attachment[%d] ContentType: expected %s, got %s", i, expectedAttachment.ContentType, *awsAttachment.ContentType)
						}

						if awsAttachment.ContentDescription == nil {
							t.Errorf("attachment[%d] ContentDescription is nil", i)
							continue
						}
						if *awsAttachment.ContentDescription != expectedAttachment.Description {
							t.Errorf("attachment[%d] ContentDescription: expected %s, got %s", i, expectedAttachment.Description, *awsAttachment.ContentDescription)
						}

						if string(awsAttachment.RawContent) != string(expectedAttachment.Content) {
							t.Errorf("attachment[%d] RawContent: expected %s, got %s", i, string(expectedAttachment.Content), string(awsAttachment.RawContent))
						}
					}

					return &sesv2.SendEmailOutput{}, nil
				},
			}

			sender := NewAWSSESSender(client)
			err := sender.SendEmail(context.Background(), testEmail)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
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
