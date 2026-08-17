package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sevoniva-labs/forge/internal/platform/database"
)

type Event struct {
	ID             string         `json:"id"`
	OccurredAt     time.Time      `json:"occurred_at"`
	RequestID      string         `json:"request_id"`
	OrganizationID string         `json:"organization_id,omitempty"`
	ActorID        string         `json:"actor_id,omitempty"`
	ActorName      string         `json:"actor_name,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resource_type,omitempty"`
	ResourceID     string         `json:"resource_id,omitempty"`
	Result         string         `json:"result"`
	ClientIP       string         `json:"client_ip,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}

type Writer struct{ db *database.DB }

func NewWriter(db *database.DB) *Writer { return &Writer{db: db} }

func (w *Writer) Write(ctx context.Context, e Event) error {
	if e.Result == "" {
		e.Result = "SUCCESS"
	}
	raw, err := json.Marshal(e.Details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}
	_, err = w.db.ExecContext(ctx, w.db.Rebind(`INSERT INTO audit_logs(id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`), uuid.NewString(), time.Now().UTC(), e.RequestID, nullIfEmpty(e.OrganizationID), nullIfEmpty(e.ActorID), e.ActorName, e.Action, e.ResourceType, e.ResourceID, e.Result, e.ClientIP, string(raw))
	return err
}

func (w *Writer) List(ctx context.Context, orgID string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, err := w.db.QueryContext(ctx, w.db.Rebind(`SELECT id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json FROM audit_logs WHERE organization_id=? ORDER BY occurred_at DESC LIMIT ?`), orgID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]Event, 0, limit)
	for rows.Next() {
		var e Event
		var org, actor sql.NullString
		var raw string
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &org, &actor, &e.ActorName, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.ClientIP, &raw); err != nil {
			return nil, err
		}
		if org.Valid {
			e.OrganizationID = org.String
		}
		if actor.Valid {
			e.ActorID = actor.String
		}
		_ = json.Unmarshal([]byte(raw), &e.Details)
		out = append(out, e)
	}
	return out, rows.Err()
}

// PurgeExpired deletes audit events older than retentionDays.
// If retentionDays <= 0, it is treated as no-op for explicit keep-all mode.
func (w *Writer) PurgeExpired(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	res, err := w.db.ExecContext(ctx, w.db.Rebind(`DELETE FROM audit_logs WHERE occurred_at < ?`), cutoff)
	if err != nil {
		return 0, err
	}
	c, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return c, nil
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
