package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sevoniva-labs/forge/internal/app/audit"
	"github.com/sevoniva-labs/forge/internal/platform/httpx"
)

var errReliableAuditUnavailable = errors.New("reliable audit unavailable")

func (s *Server) audited(ctx context.Context, event *audit.Event, operation func(context.Context) error) error {
	if s.db == nil {
		return errReliableAuditUnavailable
	}
	return s.db.WithinTx(ctx, func(txCtx context.Context) error {
		if err := operation(txCtx); err != nil {
			return err
		}
		return s.writeAudit(txCtx, *event)
	})
}

func (s *Server) writeAudit(ctx context.Context, event audit.Event) error {
	if s.audit == nil {
		return errReliableAuditUnavailable
	}
	if err := s.audit.Write(ctx, event); err != nil {
		return fmt.Errorf("%w: %v", errReliableAuditUnavailable, err)
	}
	return nil
}

func (s *Server) rejectAuditFailure(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, errReliableAuditUnavailable) {
		return false
	}
	s.log.Error("reliable audit transaction failed", "err", err, "request_id", RequestID(r), "trace_id", TraceID(r))
	httpx.Error(w, http.StatusServiceUnavailable, "RELIABLE_AUDIT_UNAVAILABLE", "关键操作审计暂不可用，请稍后重试", RequestID(r), TraceID(r))
	return true
}
