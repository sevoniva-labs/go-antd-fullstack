package audit

import "context"

// SIEMSink is the provider-neutral delivery boundary used by a reliable
// message consumer. Writer.Write never calls it inline; it first persists the
// audit event and its reliable forwarding record in one transaction.
type SIEMSink interface {
	Publish(context.Context, Event) error
	Provider() string
}
