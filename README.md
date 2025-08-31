# Email Library

A Go library for sending emails with support for multiple providers. Currently includes AWS SES implementation with comprehensive error handling and validation.

## Features

- **Provider Abstraction**: Interface-based design allows easy switching between email providers
- **AWS SES Support**: Full integration with Amazon Simple Email Service (SES) v2
- **Rich Email Content**: Support for HTML and plain text email bodies
- **Comprehensive Recipients**: Support for To, CC, BCC, and Reply-To addresses  
- **Structured Error Handling**: Categorized error types with detailed error reasons
- **Input Validation**: Built-in validation for email addresses and required fields
- **Testable**: Mockable interfaces for easy testing

## Installation

```bash
go get github.com/International-Combat-Archery-Alliance/email
```

## Usage

### Basic Example

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

### Error Handling

The library provides structured error handling with specific error reasons:

```go
err := sender.SendEmail(ctx, email)
if err != nil {
    var emailErr *email.Error
    if errors.As(err, &emailErr) {
        switch emailErr.Reason {
        case email.REASON_RATE_LIMITED:
            // Handle rate limiting
            log.Println("Rate limited, retry later")
        case email.REASON_INVALID_EMAIL:
            // Handle invalid email address
            log.Println("Invalid email address:", emailErr.Message)
        case email.REASON_UNVERIFIED_DOMAIN:
            // Handle unverified domain
            log.Println("Domain not verified with SES")
        default:
            log.Println("Email error:", emailErr.Error())
        }
    }
}
```

## Testing

Run the test suite:

```bash
go test ./...
```

## License

This project is licensed under the GNU Affero General Public License v3.0. See [LICENSE](LICENSE) for details.
