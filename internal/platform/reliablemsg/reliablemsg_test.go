package reliablemsg

import (
	"errors"
	"strings"
	"testing"
)

func TestBatchErrorRetainsFailureCountAndCauses(t *testing.T) {
	providerErr := errors.New("rocketmq unavailable")
	err := newBatchError(3, []error{providerErr})
	var batchErr *BatchError
	if !errors.As(err, &batchErr) {
		t.Fatalf("newBatchError() type = %T, want *BatchError", err)
	}
	if batchErr.Failed != 3 || !errors.Is(err, providerErr) {
		t.Fatalf("BatchError = %#v, unwrap provider error = %t", batchErr, errors.Is(err, providerErr))
	}
	if !strings.Contains(err.Error(), "3 reliable message(s) failed") {
		t.Fatalf("BatchError message = %q", err.Error())
	}
}

func TestBatchErrorDetailsAreBounded(t *testing.T) {
	failures := make([]error, 0)
	for index := 0; index < maxBatchErrorDetails+5; index++ {
		failures = appendBatchFailure(failures, errors.New("failure"))
	}
	if len(failures) != maxBatchErrorDetails {
		t.Fatalf("failure details = %d, want %d", len(failures), maxBatchErrorDetails)
	}
	if err := newBatchError(0, failures); err != nil {
		t.Fatalf("zero failures returned error %v", err)
	}
}
