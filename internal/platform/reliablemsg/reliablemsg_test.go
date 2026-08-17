package reliablemsg

import (
	"database/sql"
	"testing"
	"time"
)

func TestPendingMessagePreservesReliableEnvelope(t *testing.T) {
	deliverAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	p := pending{
		ID: "event-1", OrganizationID: sql.NullString{String: "org-1", Valid: true}, Topic: "business-events",
		Key: "account-1", Type: "account.updated", OrderingKey: "account-1", Tag: "ACCOUNT",
		Payload: `{"balance":100}`, Headers: `{"correlation-id":"request-1"}`,
		DeliverAt: sql.NullTime{Time: deliverAt, Valid: true},
	}
	message, err := p.message()
	if err != nil {
		t.Fatalf("message() error = %v", err)
	}
	if message.ID != p.ID || message.OrganizationID != "org-1" || message.Type != p.Type {
		t.Fatalf("message identity = %#v", message)
	}
	if message.Headers["correlation-id"] != "request-1" || message.OrderingKey != "account-1" {
		t.Fatalf("message metadata = %#v", message)
	}
	if !message.DeliverAt.Equal(deliverAt) {
		t.Fatalf("DeliverAt = %v, want %v", message.DeliverAt, deliverAt)
	}
}

func TestPendingMessageRejectsMalformedHeaders(t *testing.T) {
	_, err := (pending{Headers: `{`}).message()
	if err == nil {
		t.Fatal("message() accepted malformed persisted headers")
	}
}
