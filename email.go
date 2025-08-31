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
	TextBody string
}

type Sender interface {
	SendEmail(ctx context.Context, e Email) error
}
