package email

import "context"

type Email struct {
	FromAddress      string
	ToAddresses      []string
	CCAddresses      []string
	BCCAddresses     []string
	ReplyToAddresses []string
	Subject          string
	HTMLBody         string
	// The email body for recipients with non-HTML email clients.
	TextBody    string
	Attachments []Attachment
}

type Attachment struct {
	FileName    string
	Content     []byte
	Description string
	// MimeType of the content
	ContentType string
}

type Sender interface {
	SendEmail(ctx context.Context, e Email) error
}
