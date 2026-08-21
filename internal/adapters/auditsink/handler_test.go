package auditsink

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/sevoniva-labs/forge/internal/app/audit"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
)

type recordingSIEM struct {
	event audit.Event
}

func (s *recordingSIEM) Publish(_ context.Context, event audit.Event) error {
	s.event = event
	return nil
}

func (s *recordingSIEM) Provider() string { return "test-siem" }

func TestEventHandlerBindsReliableIDAndPublishes(t *testing.T) {
	sink := &recordingSIEM{}
	handler, err := NewEventHandler(sink)
	if err != nil {
		t.Fatal(err)
	}
	event := audit.Event{ID: "event-1", Action: "user.update", Result: "SUCCESS"}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), (*sql.Tx)(nil), messaging.Message{ID: "audit.event-1", Type: "audit.event", Body: body}); err != nil {
		t.Fatal(err)
	}
	if sink.event.ID != event.ID || sink.event.Action != event.Action {
		t.Fatalf("unexpected SIEM event: %#v", sink.event)
	}
}

func TestEventHandlerRejectsUnknownTypeAndIdentityConflict(t *testing.T) {
	handler, err := NewEventHandler(&recordingSIEM{})
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), nil, messaging.Message{Type: "unknown"}); err == nil {
		t.Fatal("unknown audit message type was accepted")
	}
	event := audit.Event{ID: "event-1", Action: "user.update"}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), nil, messaging.Message{ID: "audit.other", Type: "audit.event", Body: body}); err == nil {
		t.Fatal("audit identity conflict was accepted")
	}
}
