package notificationdelivery

import (
	"context"
	"database/sql"
	"testing"

	"github.com/sevoniva-labs/forge/internal/app/notification"
	"github.com/sevoniva-labs/forge/internal/platform/messaging"
)

type recordingSender struct {
	message notification.Message
}

func (s *recordingSender) Send(_ context.Context, message notification.Message) error {
	s.message = message
	return nil
}

func (s *recordingSender) Provider() string { return "test" }

func TestEmailHandlerValidatesEnvelopeAndDelegates(t *testing.T) {
	sender := &recordingSender{}
	handler, err := NewEmailHandler(sender)
	if err != nil {
		t.Fatal(err)
	}
	request, err := notification.NewEmailRequest("email-1", notification.Message{From: "sender@example.com", To: []string{"receiver@example.com"}, Subject: "notice", TextBody: "body"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeEmailRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), (*sql.Tx)(nil), messaging.Message{ID: "email-1", Type: notification.EmailMessageType, Body: body}); err != nil {
		t.Fatal(err)
	}
	if sender.message.ID != "email-1" || sender.message.Subject != "notice" {
		t.Fatalf("sender received unexpected message: %#v", sender.message)
	}
}

func TestEmailHandlerRejectsUnknownTypeAndIdentityConflict(t *testing.T) {
	handler, err := NewEmailHandler(&recordingSender{})
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), nil, messaging.Message{Type: "unknown"}); err == nil {
		t.Fatal("unknown notification type was accepted")
	}
	request, err := notification.NewEmailRequest("email-1", notification.Message{From: "sender@example.com", To: []string{"receiver@example.com"}, Subject: "notice", TextBody: "body"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := EncodeEmailRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), nil, messaging.Message{ID: "email-2", Type: notification.EmailMessageType, Body: body}); err == nil {
		t.Fatal("notification identity conflict was accepted")
	}
}
