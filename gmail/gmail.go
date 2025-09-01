package gmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/mail"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/International-Combat-Archery-Alliance/email"
)

type GmailSender struct {
	service *gmail.Service
	userID  string
}

func NewGmailSender(ctx context.Context, credentialsJSON []byte, userEmail string) (*GmailSender, error) {
	config, err := google.JWTConfigFromJSON(credentialsJSON, gmail.GmailSendScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse service account file: %v", err)
	}

	config.Subject = userEmail

	service, err := gmail.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Gmail client: %v", err)
	}

	return &GmailSender{
		service: service,
		userID:  "me",
	}, nil
}

func (g *GmailSender) SendEmail(ctx context.Context, e email.Email) error {
	if err := g.validateEmail(e); err != nil {
		return err
	}

	message, err := g.createMessage(e)
	if err != nil {
		return email.NewValidationError("Failed to create message", err)
	}

	_, err = g.service.Users.Messages.Send(g.userID, message).Context(ctx).Do()
	if err != nil {
		return g.mapGmailError(err)
	}

	return nil
}

func (g *GmailSender) createMessage(e email.Email) (*gmail.Message, error) {
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

func (g *GmailSender) createMessageWithAttachments(headers []string, body string, attachments []email.Attachment) (*gmail.Message, error) {
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

func containsContentType(headers []string) bool {
	for _, header := range headers {
		if strings.HasPrefix(header, "Content-Type:") {
			return true
		}
	}
	return false
}

func (g *GmailSender) validateEmail(e email.Email) error {
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

func (g *GmailSender) mapGmailError(err error) error {
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
