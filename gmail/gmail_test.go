package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"testing"

	"github.com/International-Combat-Archery-Alliance/email"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

// Mock Gmail service for testing
type mockGmailService struct {
	sendMessageFunc func(ctx context.Context, userID string, message *gmail.Message) (*gmail.Message, error)
}

func (m *mockGmailService) sendMessage(ctx context.Context, userID string, message *gmail.Message) (*gmail.Message, error) {
	if m.sendMessageFunc != nil {
		return m.sendMessageFunc(ctx, userID, message)
	}
	return &gmail.Message{Id: "mock-message-id"}, nil
}

// Test version of GmailSender that uses our mock service
type testGmailSender struct {
	mockService *mockGmailService
	userID      string
}

func newTestGmailSender(mockService *mockGmailService) *testGmailSender {
	return &testGmailSender{
		mockService: mockService,
		userID:      "me",
	}
}

func (g *testGmailSender) SendEmail(ctx context.Context, e email.Email) error {
	if err := g.validateEmail(e); err != nil {
		return err
	}

	message, err := g.createMessage(e)
	if err != nil {
		return email.NewValidationError("Failed to create message", err)
	}

	_, err = g.mockService.sendMessage(ctx, g.userID, message)
	if err != nil {
		return g.mapGmailError(err)
	}

	return nil
}

// Copy validation, message creation, and error mapping methods from original implementation
func (g *testGmailSender) validateEmail(e email.Email) error {
	if e.FromAddress == "" {
		return email.NewValidationError("From address is required", nil)
	}

	if _, err := mail.ParseAddress(e.FromAddress); err != nil {
		return email.NewInvalidEmailError("Invalid from address format", err)
	}

	if len(e.ToAddresses) == 0 && len(e.CCAddresses) == 0 && len(e.BCCAddresses) == 0 {
		return email.NewValidationError("At least one recipient is required", nil)
	}

	allRecipients := append(e.ToAddresses, e.CCAddresses...)
	allRecipients = append(allRecipients, e.BCCAddresses...)
	for _, addr := range allRecipients {
		if _, err := mail.ParseAddress(addr); err != nil {
			return email.NewInvalidEmailError(fmt.Sprintf("Invalid recipient address format: %s", addr), err)
		}
	}

	if e.Subject == "" {
		return email.NewValidationError("Subject is required", nil)
	}

	if e.HTMLBody == "" && e.TextBody == "" {
		return email.NewValidationError("Email body is required", nil)
	}

	return nil
}

func (g *testGmailSender) createMessage(e email.Email) (*gmail.Message, error) {
	headers := []string{
		fmt.Sprintf("From: %s", e.FromAddress),
		fmt.Sprintf("To: %s", strings.Join(e.ToAddresses, ", ")),
		fmt.Sprintf("Subject: %s", mime.QEncoding.Encode("utf-8", e.Subject)),
		"MIME-Version: 1.0",
	}

	if len(e.CCAddresses) > 0 {
		headers = append(headers, fmt.Sprintf("Cc: %s", strings.Join(e.CCAddresses, ", ")))
	}

	if len(e.BCCAddresses) > 0 {
		headers = append(headers, fmt.Sprintf("Bcc: %s", strings.Join(e.BCCAddresses, ", ")))
	}

	if len(e.ReplyToAddresses) > 0 {
		headers = append(headers, fmt.Sprintf("Reply-To: %s", strings.Join(e.ReplyToAddresses, ", ")))
	}

	var body string
	if e.HTMLBody != "" && e.TextBody != "" {
		boundary := "boundary123456789"
		headers = append(headers, fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s", boundary))

		body = fmt.Sprintf(`
--%s
Content-Type: text/plain; charset=utf-8
Content-Transfer-Encoding: 8bit

%s

--%s
Content-Type: text/html; charset=utf-8
Content-Transfer-Encoding: 8bit

%s

--%s--`, boundary, e.TextBody, boundary, e.HTMLBody, boundary)
	} else if e.HTMLBody != "" {
		headers = append(headers, "Content-Type: text/html; charset=utf-8")
		headers = append(headers, "Content-Transfer-Encoding: 8bit")
		body = e.HTMLBody
	} else {
		headers = append(headers, "Content-Type: text/plain; charset=utf-8")
		headers = append(headers, "Content-Transfer-Encoding: 8bit")
		body = e.TextBody
	}

	if len(e.Attachments) > 0 {
		return g.createMessageWithAttachments(headers, body, e.Attachments)
	}

	raw := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	return &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(raw)),
	}, nil
}

func (g *testGmailSender) createMessageWithAttachments(headers []string, body string, attachments []email.Attachment) (*gmail.Message, error) {
	boundary := "mixed_boundary_123456789"

	for i, header := range headers {
		if strings.HasPrefix(header, "Content-Type:") {
			headers[i] = fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s", boundary)
			break
		}
	}
	if !containsContentType(headers) {
		headers = append(headers, fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s", boundary))
	}

	var parts []string

	textPart := fmt.Sprintf(`--%s
Content-Type: text/plain; charset=utf-8
Content-Transfer-Encoding: 8bit

%s`, boundary, body)
	parts = append(parts, textPart)

	for _, attachment := range attachments {
		encodedContent := base64.StdEncoding.EncodeToString(attachment.Content)

		attachmentPart := fmt.Sprintf(`--%s
Content-Type: %s; name="%s"
Content-Disposition: attachment; filename="%s"
Content-Transfer-Encoding: base64

%s`, boundary, attachment.ContentType, attachment.FileName, attachment.FileName, encodedContent)
		parts = append(parts, attachmentPart)
	}

	parts = append(parts, fmt.Sprintf("--%s--", boundary))

	raw := strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.Join(parts, "\r\n")

	return &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(raw)),
	}, nil
}

func (g *testGmailSender) mapGmailError(err error) error {
	if apiErr, ok := err.(*googleapi.Error); ok {
		switch apiErr.Code {
		case 400:
			if strings.Contains(strings.ToLower(apiErr.Message), "invalid") {
				if strings.Contains(strings.ToLower(apiErr.Message), "recipient") ||
					strings.Contains(strings.ToLower(apiErr.Message), "email") ||
					strings.Contains(strings.ToLower(apiErr.Message), "address") {
					return email.NewInvalidEmailError("Invalid email address", err)
				}
			}
			if strings.Contains(strings.ToLower(apiErr.Message), "malformed") ||
				strings.Contains(strings.ToLower(apiErr.Message), "encoding") {
				return email.NewValidationError("Invalid message format", err)
			}
			if strings.Contains(strings.ToLower(apiErr.Message), "too large") ||
				strings.Contains(strings.ToLower(apiErr.Message), "size") {
				return email.NewValidationError("Message too large", err)
			}
			return email.NewValidationError("Invalid request parameters", err)

		case 401:
			return email.NewValidationError("Authentication failed - check service account credentials", err)

		case 403:
			if strings.Contains(strings.ToLower(apiErr.Message), "scope") ||
				strings.Contains(strings.ToLower(apiErr.Message), "permission") {
				return email.NewUnverifiedDomainError("Insufficient permissions to send email", err)
			}
			if strings.Contains(strings.ToLower(apiErr.Message), "domain") {
				return email.NewUnverifiedDomainError("Domain policy prevents sending", err)
			}
			if strings.Contains(strings.ToLower(apiErr.Message), "blocked") {
				return email.NewMessageRejectedError("Sender blocked by recipient", err)
			}
			return email.NewUnverifiedDomainError("Permission denied", err)

		case 429:
			if strings.Contains(strings.ToLower(apiErr.Message), "quota") {
				return email.NewRateLimitedError("Gmail API quota exceeded", err)
			}
			if strings.Contains(strings.ToLower(apiErr.Message), "rate") {
				return email.NewRateLimitedError("Gmail API rate limit exceeded", err)
			}
			return email.NewRateLimitedError("Too many requests", err)

		case 500:
			return email.NewServiceError("Internal Gmail server error", err)

		case 503:
			return email.NewServiceError("Gmail service temporarily unavailable", err)

		case 504:
			return email.NewServiceError("Gmail API request timeout", err)

		default:
			return email.NewServiceError(fmt.Sprintf("Gmail API error (HTTP %d)", apiErr.Code), err)
		}
	}

	if strings.Contains(strings.ToLower(err.Error()), "context") &&
		strings.Contains(strings.ToLower(err.Error()), "deadline") {
		return email.NewServiceError("Request timeout", err)
	}

	if strings.Contains(strings.ToLower(err.Error()), "connection") ||
		strings.Contains(strings.ToLower(err.Error()), "network") {
		return email.NewServiceError("Network error", err)
	}

	return email.NewUnknownError("Gmail API error", err)
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
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockGmailService{
				sendMessageFunc: func(ctx context.Context, userID string, message *gmail.Message) (*gmail.Message, error) {
					// Verify the request is constructed correctly
					if userID != "me" {
						t.Errorf("expected userID 'me', got %s", userID)
					}
					if message.Raw == "" {
						t.Error("expected non-empty Raw message")
					}
					return &gmail.Message{Id: "test-message-id"}, nil
				},
			}

			sender := newTestGmailSender(mockService)
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
			mockService := &mockGmailService{}
			sender := newTestGmailSender(mockService)

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

func TestSendEmail_GmailAPIErrors(t *testing.T) {
	tests := []struct {
		name          string
		gmailError    error
		expectedError email.ErrorReason
	}{
		{
			name: "invalid recipient error",
			gmailError: &googleapi.Error{
				Code:    400,
				Message: "Invalid recipient email address",
			},
			expectedError: email.REASON_INVALID_EMAIL,
		},
		{
			name: "malformed message error",
			gmailError: &googleapi.Error{
				Code:    400,
				Message: "Malformed message content",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
		{
			name: "message too large error",
			gmailError: &googleapi.Error{
				Code:    400,
				Message: "Message too large",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
		{
			name: "authentication failed error",
			gmailError: &googleapi.Error{
				Code:    401,
				Message: "Authentication failed",
			},
			expectedError: email.REASON_VALIDATION_ERROR,
		},
		{
			name: "insufficient permissions error",
			gmailError: &googleapi.Error{
				Code:    403,
				Message: "Insufficient permissions to send email",
			},
			expectedError: email.REASON_UNVERIFIED_DOMAIN,
		},
		{
			name: "domain policy error",
			gmailError: &googleapi.Error{
				Code:    403,
				Message: "Domain policy prevents sending",
			},
			expectedError: email.REASON_UNVERIFIED_DOMAIN,
		},
		{
			name: "blocked sender error",
			gmailError: &googleapi.Error{
				Code:    403,
				Message: "Sender blocked by recipient",
			},
			expectedError: email.REASON_MESSAGE_REJECTED,
		},
		{
			name: "quota exceeded error",
			gmailError: &googleapi.Error{
				Code:    429,
				Message: "Gmail API quota exceeded",
			},
			expectedError: email.REASON_RATE_LIMITED,
		},
		{
			name: "rate limit exceeded error",
			gmailError: &googleapi.Error{
				Code:    429,
				Message: "Rate limit exceeded",
			},
			expectedError: email.REASON_RATE_LIMITED,
		},
		{
			name: "internal server error",
			gmailError: &googleapi.Error{
				Code:    500,
				Message: "Internal server error",
			},
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name: "service unavailable error",
			gmailError: &googleapi.Error{
				Code:    503,
				Message: "Service temporarily unavailable",
			},
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name: "request timeout error",
			gmailError: &googleapi.Error{
				Code:    504,
				Message: "Request timeout",
			},
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name: "unknown gmail error",
			gmailError: &googleapi.Error{
				Code:    418,
				Message: "I'm a teapot",
			},
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name:          "network error",
			gmailError:    errors.New("network connection failed"),
			expectedError: email.REASON_SERVICE_ERROR,
		},
		{
			name:          "context deadline error",
			gmailError:    errors.New("context deadline exceeded"),
			expectedError: email.REASON_SERVICE_ERROR,
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
			mockService := &mockGmailService{
				sendMessageFunc: func(ctx context.Context, userID string, message *gmail.Message) (*gmail.Message, error) {
					return nil, tt.gmailError
				},
			}

			sender := newTestGmailSender(mockService)
			err := sender.SendEmail(context.Background(), validEmail)

			if err == nil {
				t.Error("expected Gmail error, got nil")
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

func TestMessageCreation(t *testing.T) {
	tests := []struct {
		name  string
		email email.Email
	}{
		{
			name: "message with attachments",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test with attachments",
				TextBody:    "Hello World",
				Attachments: []email.Attachment{
					{
						FileName:    "test.txt",
						Content:     []byte("test content"),
						Description: "Test file",
						ContentType: "text/plain",
					},
				},
			},
		},
		{
			name: "multipart message with HTML and text",
			email: email.Email{
				FromAddress: "sender@example.com",
				ToAddresses: []string{"recipient@example.com"},
				Subject:     "Test multipart",
				HTMLBody:    "<p>Hello <strong>World</strong></p>",
				TextBody:    "Hello World",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockGmailService{
				sendMessageFunc: func(ctx context.Context, userID string, message *gmail.Message) (*gmail.Message, error) {
					if message.Raw == "" {
						t.Error("expected non-empty Raw message")
					}
					// Verify the message can be base64 decoded
					if _, err := base64.URLEncoding.DecodeString(message.Raw); err != nil {
						t.Errorf("invalid base64 encoding in Raw message: %v", err)
					}
					return &gmail.Message{Id: "test-id"}, nil
				},
			}

			sender := newTestGmailSender(mockService)
			err := sender.SendEmail(context.Background(), tt.email)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
