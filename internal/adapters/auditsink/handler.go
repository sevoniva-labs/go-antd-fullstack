package auditsink

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sevoniva-labs/forge/internal/app/audit"
	"github.com/sevoniva-labs/forge/internal/platform/messageworker"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
)

type EventHandler struct {
	sink audit.SIEMSink
}

func NewEventHandler(sink audit.SIEMSink) (*EventHandler, error) {
	if sink == nil {
		return nil, errors.New("SIEM sink is required")
	}
	return &EventHandler{sink: sink}, nil
}

func (h *EventHandler) Handle(ctx context.Context, _ *sql.Tx, message messaging.Message) error {
	if message.Type != "audit.event" {
		return fmt.Errorf("unexpected audit message type %q", message.Type)
	}
	var event audit.Event
	if err := json.Unmarshal(message.Body, &event); err != nil {
		return fmt.Errorf("decode audit event: %w", err)
	}
	if event.ID == "" || message.ID != "audit."+event.ID {
		return errors.New("audit message identity does not match payload")
	}
	return h.sink.Publish(ctx, event)
}

func (h *EventHandler) MessageHandler() messageworker.Handler { return h.Handle }

var _ messageworker.Handler = (*EventHandler)(nil).Handle
