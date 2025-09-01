# Email Library

A Go library for sending emails with support for multiple providers. Includes AWS SES and Gmail API implementations with comprehensive error handling and validation.

## Features

- **Provider Abstraction**: Interface-based design allows easy switching between email providers
- **Multiple Providers**: Support for AWS SES and Gmail API
- **Rich Email Content**: Support for HTML and plain text email bodies
- **Attachments**: Full support for file attachments with proper MIME encoding
- **Comprehensive Recipients**: Support for To, CC, BCC, and Reply-To addresses  
- **Structured Error Handling**: Categorized error types with detailed error reasons
- **Input Validation**: Built-in validation for email addresses and required fields
- **Authentication**: Service account support for Gmail, AWS IAM for SES
- **Testable**: Mockable interfaces with comprehensive test coverage

## Installation

```bash
go get github.com/International-Combat-Archery-Alliance/email
```

## Usage

### AWS SES Example

```go
package main

import (
    "context"
    "log"

    "github.com/International-Combat-Archery-Alliance/email"
    "github.com/International-Combat-Archery-Alliance/email/awsses"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/sesv2"
)

func main() {
    // Load AWS configuration
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        log.Fatal(err)
    }

    // Create SES client and email sender
    sesClient := sesv2.NewFromConfig(cfg)
    sender := awsses.NewAWSSESSender(sesClient)

    // Create email
    email := email.Email{
        FromAddress: "sender@example.com",
        ToAddresses: []string{"recipient@example.com"},
        CCAddresses: []string{"cc@example.com"},
        Subject:     "Hello World",
        HTMLBody:    "<h1>Hello from Go!</h1><p>This is an HTML email.</p>",
        TextBody:    "Hello from Go!\n\nThis is a plain text email.",
    }

    // Send email
    err = sender.SendEmail(context.Background(), email)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Gmail API Example

```go
package main

import (
    "context"
    "log"
    "os"

    "github.com/International-Combat-Archery-Alliance/email"
    "github.com/International-Combat-Archery-Alliance/email/gmail"
)

func main() {
    // Load service account credentials
    credentialsJSON, err := os.ReadFile("path/to/service-account.json")
    if err != nil {
        log.Fatal(err)
    }

    // Create Gmail sender with service account
    sender, err := gmail.NewGmailSender(
        context.Background(),
        credentialsJSON,
        "user@yourdomain.com", // User to impersonate
    )
    if err != nil {
        log.Fatal(err)
    }

    // Create email with attachment
    email := email.Email{
        FromAddress: "sender@yourdomain.com",
        ToAddresses: []string{"recipient@example.com"},
        Subject:     "Hello from Gmail API",
        HTMLBody:    "<h1>Hello from Go!</h1><p>This email was sent via Gmail API.</p>",
        TextBody:    "Hello from Go!\n\nThis email was sent via Gmail API.",
        Attachments: []email.Attachment{
            {
                FileName:    "document.pdf",
                Content:     fileContent, // []byte
                ContentType: "application/pdf",
                Description: "Important document",
            },
        },
    }

    // Send email
    err = sender.SendEmail(context.Background(), email)
    if err != nil {
        log.Fatal(err)
    }
}
```

## Provider Setup

### AWS SES Setup
1. Configure AWS credentials via environment variables, AWS config file, or IAM roles
2. Verify your domain and email addresses in the AWS SES console
3. Ensure your account has the necessary SES sending permissions

### Gmail API Setup
1. Create a project in Google Cloud Console
2. Enable the Gmail API
3. Create a service account with domain-wide delegation
4. Download the service account JSON credentials
5. In Google Workspace Admin, authorize the service account with the scope: `https://www.googleapis.com/auth/gmail.send`

### Error Handling

The library provides structured error handling with specific error reasons:

```go
err := sender.SendEmail(ctx, email)
if err != nil {
    var emailErr *email.Error
    if errors.As(err, &emailErr) {
        switch emailErr.Reason {
        case email.REASON_RATE_LIMITED:
            // Handle rate limiting (Gmail quota, SES rate limits)
            log.Println("Rate limited, retry later")
        case email.REASON_INVALID_EMAIL:
            // Handle invalid email address
            log.Println("Invalid email address:", emailErr.Message)
        case email.REASON_UNVERIFIED_DOMAIN:
            // Handle unverified domain (SES) or permission issues (Gmail)
            log.Println("Domain verification or permission issue")
        case email.REASON_MESSAGE_REJECTED:
            // Handle rejected messages (spam filters, blocked senders)
            log.Println("Message rejected:", emailErr.Message)
        case email.REASON_SERVICE_ERROR:
            // Handle service unavailability
            log.Println("Service temporarily unavailable")
        default:
            log.Println("Email error:", emailErr.Error())
        }
    }
}
```

## Error Types

The library categorizes errors into the following types:

- **`REASON_VALIDATION_ERROR`**: Invalid input parameters (missing fields, etc.)
- **`REASON_INVALID_EMAIL`**: Malformed email addresses
- **`REASON_RATE_LIMITED`**: API rate limits or quota exceeded
- **`REASON_UNVERIFIED_DOMAIN`**: Domain not verified (SES) or insufficient permissions (Gmail)
- **`REASON_MESSAGE_REJECTED`**: Message rejected by filters or policies
- **`REASON_SERVICE_ERROR`**: Provider service temporarily unavailable
- **`REASON_UNKNOWN`**: Unexpected errors

## Testing

Run the test suite:

```bash
go test ./...
```

The library includes comprehensive unit tests with mock implementations for both AWS SES and Gmail API providers.

## License

This project is licensed under the GNU Affero General Public License v3.0. See [LICENSE](LICENSE) for details.
