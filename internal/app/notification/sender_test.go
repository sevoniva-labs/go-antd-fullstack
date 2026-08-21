package notification

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEmailRequestBindsStableIDAndRoundTrips(t *testing.T) {
	request, err := NewEmailRequest("email-1", Message{From: "sender@example.com", To: []string{"receiver@example.com"}, Subject: "notice", TextBody: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if request.Message.ID != request.ID {
		t.Fatalf("message ID was not bound: %#v", request)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEmailRequest(body)
	if err != nil || decoded.ID != "email-1" {
		t.Fatalf("decoded request = %#v, err=%v", decoded, err)
	}
}

func TestEmailRequestRejectsMalformedOrMismatchedInput(t *testing.T) {
	valid := Message{From: "sender@example.com", To: []string{"receiver@example.com"}, Subject: "notice", TextBody: "body"}
	for name, id := range map[string]string{"empty": "", "newline": "email\n1"} {
		if _, err := NewEmailRequest(id, valid); err == nil {
			t.Errorf("%s ID was accepted", name)
		}
	}
	valid.ID = "other"
	if _, err := NewEmailRequest("email-1", valid); err == nil {
		t.Fatal("mismatched message ID was accepted")
	}
	if _, err := DecodeEmailRequest([]byte(`{"id":"email-1","message":{"from":"","to":[],"subject":"","textBody":""}}`)); err == nil {
		t.Fatal("malformed email payload was accepted")
	}
	if strings.Contains(valid.Subject, "\n") {
		t.Fatal("test fixture unexpectedly contains a newline")
	}
}
