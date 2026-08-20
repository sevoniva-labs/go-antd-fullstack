package audit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strconv"
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
	SequenceNo     int64          `json:"sequence_no,omitempty"`
	PrevHash       string         `json:"prev_hash,omitempty"`
	EventHash      string         `json:"event_hash,omitempty"`
}

type Writer struct{ db *database.DB }

var ErrIntegrityViolation = errors.New("audit integrity violation")

func NewWriter(db *database.DB) *Writer { return &Writer{db: db} }

func (w *Writer) Write(ctx context.Context, e Event) error {
	if e.Result == "" {
		e.Result = "SUCCESS"
	}
	raw, err := json.Marshal(e.Details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}
	return w.db.WithinTx(ctx, func(txCtx context.Context) error {
		scope := e.OrganizationID
		if scope == "" {
			scope = "global"
		}
		insertHead := `INSERT INTO audit_chain_heads(scope,sequence_no,head_hash) VALUES(?,?,?)`
		if w.db.Provider == "postgres" {
			insertHead += ` ON CONFLICT (scope) DO NOTHING`
		} else {
			insertHead = `INSERT IGNORE INTO audit_chain_heads(scope,sequence_no,head_hash) VALUES(?,?,?)`
		}
		if _, err := w.db.ExecContext(txCtx, w.db.Rebind(insertHead), scope, int64(0), ""); err != nil {
			return err
		}
		var sequence int64
		var previous string
		if err := w.db.QueryRowContext(txCtx, w.db.Rebind(`SELECT sequence_no,head_hash FROM audit_chain_heads WHERE scope=? FOR UPDATE`), scope).Scan(&sequence, &previous); err != nil {
			return err
		}
		e.ID = uuid.NewString()
		e.OccurredAt = time.Now().UTC()
		e.SequenceNo = sequence + 1
		e.PrevHash = previous
		e.EventHash = auditEventHash(e, string(raw))
		if _, err := w.db.ExecContext(txCtx, w.db.Rebind(`INSERT INTO audit_logs(id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), e.ID, e.OccurredAt, e.RequestID, nullIfEmpty(e.OrganizationID), nullIfEmpty(e.ActorID), e.ActorName, e.Action, e.ResourceType, e.ResourceID, e.Result, e.ClientIP, string(raw), e.SequenceNo, e.PrevHash, e.EventHash); err != nil {
			return err
		}
		_, err := w.db.ExecContext(txCtx, w.db.Rebind(`UPDATE audit_chain_heads SET sequence_no=?,head_hash=? WHERE scope=?`), e.SequenceNo, e.EventHash, scope)
		return err
	})
}

func (w *Writer) List(ctx context.Context, orgID string, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	rows, err := w.db.QueryContext(ctx, w.db.Rebind(`SELECT id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash FROM audit_logs WHERE organization_id=? ORDER BY occurred_at DESC LIMIT ?`), orgID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]Event, 0, limit)
	for rows.Next() {
		var e Event
		var org, actor sql.NullString
		var raw string
		var sequence sql.NullInt64
		var previous, eventHash sql.NullString
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &org, &actor, &e.ActorName, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.ClientIP, &raw, &sequence, &previous, &eventHash); err != nil {
			return nil, err
		}
		if sequence.Valid {
			e.SequenceNo = sequence.Int64
		}
		e.PrevHash, e.EventHash = previous.String, eventHash.String
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

func (w *Writer) VerifyIntegrity(ctx context.Context, orgID string) error {
	rows, err := w.db.QueryContext(ctx, w.db.Rebind(`SELECT id,occurred_at,request_id,organization_id,actor_id,actor_name,action,resource_type,resource_id,result,client_ip,details_json,sequence_no,prev_hash,event_hash FROM audit_logs WHERE organization_id=? ORDER BY sequence_no ASC,id ASC`), orgID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	var expectedSequence int64 = 1
	previous := ""
	for rows.Next() {
		var e Event
		var organizationID, actorID sql.NullString
		var raw string
		var sequence sql.NullInt64
		var previousHash, eventHash sql.NullString
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.RequestID, &organizationID, &actorID, &e.ActorName, &e.Action, &e.ResourceType, &e.ResourceID, &e.Result, &e.ClientIP, &raw, &sequence, &previousHash, &eventHash); err != nil {
			return err
		}
		if !sequence.Valid || !eventHash.Valid {
			return fmt.Errorf("%w: legacy audit event has no integrity proof", ErrIntegrityViolation)
		}
		e.OrganizationID, e.ActorID = organizationID.String, actorID.String
		e.SequenceNo, e.PrevHash, e.EventHash = sequence.Int64, previousHash.String, eventHash.String
		if e.SequenceNo != expectedSequence || e.PrevHash != previous || e.EventHash != auditEventHash(e, raw) {
			return fmt.Errorf("%w at sequence %d", ErrIntegrityViolation, e.SequenceNo)
		}
		previous = e.EventHash
		expectedSequence++
	}
	return rows.Err()
}

// PurgeExpired deletes audit events older than retentionDays.
// If retentionDays <= 0, it is treated as no-op for explicit keep-all mode.
func (w *Writer) PurgeExpired(ctx context.Context, retentionDays int) (int64, error) {
	_ = ctx
	_ = retentionDays
	return 0, errors.New("audit purge requires verified WORM archive adapter")
}

func auditEventHash(e Event, raw string) string {
	digest := sha256.New()
	writeAuditHashPart(digest, e.ID)
	writeAuditHashPart(digest, e.OccurredAt.UTC().Format(time.RFC3339Nano))
	writeAuditHashPart(digest, e.RequestID)
	writeAuditHashPart(digest, e.OrganizationID)
	writeAuditHashPart(digest, e.ActorID)
	writeAuditHashPart(digest, e.ActorName)
	writeAuditHashPart(digest, e.Action)
	writeAuditHashPart(digest, e.ResourceType)
	writeAuditHashPart(digest, e.ResourceID)
	writeAuditHashPart(digest, e.Result)
	writeAuditHashPart(digest, e.ClientIP)
	writeAuditHashPart(digest, raw)
	writeAuditHashPart(digest, strconv.FormatInt(e.SequenceNo, 10))
	writeAuditHashPart(digest, e.PrevHash)
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func writeAuditHashPart(digest hash.Hash, value string) {
	_, _ = digest.Write([]byte(strconv.Itoa(len(value))))
	_, _ = digest.Write([]byte(":"))
	_, _ = digest.Write([]byte(value))
	_, _ = digest.Write([]byte("|"))
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
