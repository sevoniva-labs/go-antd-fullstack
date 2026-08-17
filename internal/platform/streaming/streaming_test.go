package streaming

import (
	"context"
	"testing"

	"github.com/sevoniva-labs/forge/internal/platform/config"
)

func TestPrepareRecordNormalizesHeadersWithoutBusinessEnvelope(t *testing.T) {
	record, headers, err := prepareRecord(context.Background(), Record{
		Stream: " ledger-postings ", Key: []byte("account-1"), Value: []byte("entry"),
		Headers: map[string]string{"Correlation-ID": "request-1"},
	})
	if err != nil {
		t.Fatalf("prepareRecord() error = %v", err)
	}
	if record.Stream != "ledger-postings" || headers["correlation-id"] != "request-1" {
		t.Fatalf("record = %#v, headers = %#v", record, headers)
	}
	if _, exists := headers["x-forge-event-id"]; exists {
		t.Fatal("stream record unexpectedly received a business-message identity")
	}
}

func TestDisabledStreamingDoesNotSilentlyDrop(t *testing.T) {
	producer, err := New(config.Streaming{Provider: "disabled"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := producer.Append(context.Background(), Record{Stream: "events"}); err == nil {
		t.Fatal("disabled streaming provider accepted a record")
	}
}
