package notification

import "context"

type Message struct {
	From     string
	To       []string
	Subject  string
	TextBody string
	HTMLBody string
}

// Sender is the provider-neutral notification boundary. Callers should place
// retryable notification requests in the reliable message flow instead of
// sending mail inside a business transaction.
type Sender interface {
	Send(context.Context, Message) error
	Provider() string
}
