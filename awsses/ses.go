package awsses

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/International-Combat-Archery-Alliance/email"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"
)

var _ email.Sender = &AWSSESSender{}

type SESClient interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

type AWSSESSender struct {
	sesClient SESClient
}

func NewAWSSESSender(client SESClient) *AWSSESSender {
	return &AWSSESSender{
		sesClient: client,
	}
}

func (a *AWSSESSender) SendEmail(ctx context.Context, e email.Email) error {
	if err := validateEmail(e); err != nil {
		return err
	}

	_, err := a.sesClient.SendEmail(ctx, &sesv2.SendEmailInput{
		Content: &types.EmailContent{
			Simple: &types.Message{
				Body: &types.Body{
					Html: htmlContentFromEmail(e),
					Text: textContentFromEmail(e),
				},
				Subject:     utf8Content(e.Subject),
				Attachments: attachmentsToAWS(e.Attachments),
			},
		},
		Destination: &types.Destination{
			ToAddresses:  e.ToAddresses,
			CcAddresses:  e.CCAddresses,
			BccAddresses: e.BCCAddresses,
		},
		FromEmailAddress: aws.String(e.FromAddress),
		ReplyToAddresses: e.ReplyToAddresses,
	})

	if err != nil {
		return categorizeAWSError(err)
	}

	return nil
}

func attachmentsToAWS(attachments []email.Attachment) []types.Attachment {
	awsAttachments := make([]types.Attachment, len(attachments))

	for i, a := range attachments {
		awsAttachments[i] = attachmentToAWS(a)
	}

	return awsAttachments
}

func attachmentToAWS(attachment email.Attachment) types.Attachment {
	return types.Attachment{
		FileName:           aws.String(attachment.FileName),
		RawContent:         attachment.Content,
		ContentType:        aws.String(attachment.ContentType),
		ContentDescription: aws.String(attachment.Description),
		ContentDisposition: types.AttachmentContentDispositionAttachment,
	}
}

func htmlContentFromEmail(e email.Email) *types.Content {
	if e.HTMLBody == "" {
		return nil
	}

	return utf8Content(e.HTMLBody)
}

func textContentFromEmail(e email.Email) *types.Content {
	if e.TextBody == "" {
		return nil
	}

	return utf8Content(e.TextBody)
}

func utf8Content(s string) *types.Content {
	return &types.Content{
		Data:    aws.String(s),
		Charset: aws.String("UTF-8"),
	}
}

func validateEmail(e email.Email) error {
	if e.FromAddress == "" {
		return email.NewValidationError("from address is required", nil)
	}

	if !isValidEmailAddress(e.FromAddress) {
		return email.NewInvalidEmailError("invalid from address format", nil)
	}

	if len(e.ToAddresses)+len(e.CCAddresses)+len(e.BCCAddresses) == 0 {
		return email.NewValidationError("at least one recipient is required", nil)
	}

	allAddresses := append(append(e.ToAddresses, e.CCAddresses...), e.BCCAddresses...)
	for _, addr := range allAddresses {
		if !isValidEmailAddress(addr) {
			return email.NewInvalidEmailError(fmt.Sprintf("invalid recipient address: %s", addr), nil)
		}
	}

	if e.Subject == "" {
		return email.NewValidationError("subject is required", nil)
	}

	if e.HTMLBody == "" && e.TextBody == "" {
		return email.NewValidationError("email body is required (HTML or text)", nil)
	}

	return nil
}

func categorizeAWSError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "TooManyRequestsException":
			return email.NewRateLimitedError("sending rate limit exceeded", err)
		case "MessageRejected":
			return email.NewMessageRejectedError("message rejected by SES", err)
		case "MailFromDomainNotVerifiedException":
			return email.NewUnverifiedDomainError("sender domain not verified", err)
		case "InvalidParameterValueException":
			return email.NewInvalidEmailError("invalid email parameter", err)
		case "ServiceUnavailableException", "InternalServiceErrorException":
			return email.NewServiceError("AWS SES service error", err)
		}
	}

	return email.NewUnknownError("failed to send email", err)
}

func isValidEmailAddress(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
