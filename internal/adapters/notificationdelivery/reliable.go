package notificationdelivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sevoniva-labs/forge/internal/app/notification"
	"github.com/sevoniva-labs/forge/internal/platform/database"
	"github.com/sevoniva-labs/forge/internal/platform/messageworker"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
	"github.com/sevoniva-labs/forge/internal/platform/reliablemsg"
)

type EmailQueue struct {
	topic string
}

func NewEmailQueue(topic string) (*EmailQueue, error) {
	if topic == "" {
		return nil, errors.New("notification email topic is required")
	}
	return &EmailQueue{topic: topic}, nil
}

// EnqueueEmailTx must run in the same database transaction as the business
// mutation that triggers the notification. Delivery remains at-least-once.
func (q *EmailQueue) EnqueueEmailTx(ctx context.Context, db *database.DB, tx *sql.Tx, organizationID, id string, message notification.Message) (string, error) {
	request, err := notification.NewEmailRequest(id, message)
	if err != nil {
		return "", err
	}
	return reliablemsg.EnqueueTx(ctx, db, tx, reliablemsg.Event{
		ID: id, OrganizationID: organizationID, Topic: q.topic, Key: id, Type: notification.EmailMessageType,
		Payload: request, OrderingKey: organizationID,
		Headers: map[string]string{"x-forge-notification-provider": "email"},
	})
}

type EmailHandler struct {
	sender notification.Sender
}

func NewEmailHandler(sender notification.Sender) (*EmailHandler, error) {
	if sender == nil {
		return nil, errors.New("notification email sender is required")
	}
	return &EmailHandler{sender: sender}, nil
}

func (h *EmailHandler) Handle(ctx context.Context, _ *sql.Tx, message messaging.Message) error {
	if message.Type != notification.EmailMessageType {
		return fmt.Errorf("unexpected notification message type %q", message.Type)
	}
	request, err := notification.DecodeEmailRequest(message.Body)
	if err != nil {
		return err
	}
	if message.ID != "" && request.ID != message.ID {
		return errors.New("notification message ID does not match payload ID")
	}
	return h.sender.Send(ctx, request.Message)
}

func (h *EmailHandler) MessageHandler() messageworker.Handler { return h.Handle }

func EncodeEmailRequest(request notification.EmailRequest) ([]byte, error) {
	validated, err := notification.NewEmailRequest(request.ID, request.Message)
	if err != nil {
		return nil, err
	}
	return json.Marshal(validated)
}

var _ messageworker.Handler = (*EmailHandler)(nil).Handle
