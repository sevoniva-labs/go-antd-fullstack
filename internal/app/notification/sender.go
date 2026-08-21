package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const EmailMessageType = "notification.email"

type Message struct {
	ID       string
	From     string
	To       []string
	Subject  string
	TextBody string
	HTMLBody string
}

type EmailRequest struct {
	ID      string  `json:"id"`
	Message Message `json:"message"`
}

func NewEmailRequest(id string, message Message) (EmailRequest, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 200 || strings.ContainsAny(id, "\r\n") {
		return EmailRequest{}, errors.New("notification email id is invalid")
	}
	if message.ID != "" && message.ID != id {
		return EmailRequest{}, errors.New("notification email message id does not match request id")
	}
	message.ID = id
	if strings.TrimSpace(message.From) == "" || len(message.To) == 0 || strings.TrimSpace(message.Subject) == "" || (message.TextBody == "" && message.HTMLBody == "") {
		return EmailRequest{}, errors.New("notification email requires sender, recipients, subject and body")
	}
	return EmailRequest{ID: id, Message: message}, nil
}

func DecodeEmailRequest(body []byte) (EmailRequest, error) {
	var request EmailRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return EmailRequest{}, fmt.Errorf("decode notification email: %w", err)
	}
	validated, err := NewEmailRequest(request.ID, request.Message)
	if err != nil {
		return EmailRequest{}, err
	}
	return validated, nil
}

// Sender is the provider-neutral notification boundary. Callers should place
// retryable notification requests in the reliable message flow instead of
// sending mail inside a business transaction.
type Sender interface {
	Send(context.Context, Message) error
	Provider() string
}
