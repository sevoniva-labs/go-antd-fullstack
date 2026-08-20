package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sevoniva-labs/forge/internal/platform/database"
	securitypolicy "github.com/sevoniva-labs/forge/internal/platform/security/datapolicy"
)

type DataPolicyRepo struct{ db *database.DB }

func NewDataPolicyRepo(db *database.DB) *DataPolicyRepo { return &DataPolicyRepo{db: db} }

func (r *DataPolicyRepo) List(ctx context.Context, organizationID string) ([]securitypolicy.Record, error) {
	rows, err := r.db.QueryContext(ctx, r.db.Rebind(`SELECT id,organization_id,field_key,classification,owner,purpose,residency,retention_days,tags_json,mask_strategy,export_approval,watermark,created_at,updated_at FROM data_field_policies WHERE organization_id=? ORDER BY field_key`), organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]securitypolicy.Record, 0)
	for rows.Next() {
		item, err := scanDataPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *DataPolicyRepo) Upsert(ctx context.Context, organizationID string, policy securitypolicy.FieldPolicy) (securitypolicy.Record, error) {
	tagsJSON, err := json.Marshal(policy.Tags)
	if err != nil {
		return securitypolicy.Record{}, err
	}
	now := time.Now().UTC()
	var result securitypolicy.Record
	err = r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var id string
		err := tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id FROM data_field_policies WHERE organization_id=? AND field_key=? FOR UPDATE`), organizationID, policy.Key).Scan(&id)
		switch {
		case err == nil:
			_, err = tx.ExecContext(ctx, r.db.Rebind(`UPDATE data_field_policies SET classification=?,owner=?,purpose=?,residency=?,retention_days=?,tags_json=?,mask_strategy=?,export_approval=?,watermark=?,updated_at=? WHERE id=? AND organization_id=?`), policy.Classification, policy.Owner, policy.Purpose, policy.Residency, policy.RetentionDays, string(tagsJSON), policy.Mask, policy.ExportApproval, policy.Watermark, now, id, organizationID)
		case err == sql.ErrNoRows:
			id = uuid.NewString()
			_, err = tx.ExecContext(ctx, r.db.Rebind(`INSERT INTO data_field_policies(id,organization_id,field_key,classification,owner,purpose,residency,retention_days,tags_json,mask_strategy,export_approval,watermark,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`), id, organizationID, policy.Key, policy.Classification, policy.Owner, policy.Purpose, policy.Residency, policy.RetentionDays, string(tagsJSON), policy.Mask, policy.ExportApproval, policy.Watermark, now, now)
		default:
			return err
		}
		if err != nil {
			return err
		}
		result, err = scanDataPolicy(tx.QueryRowContext(ctx, r.db.Rebind(`SELECT id,organization_id,field_key,classification,owner,purpose,residency,retention_days,tags_json,mask_strategy,export_approval,watermark,created_at,updated_at FROM data_field_policies WHERE id=? AND organization_id=?`), id, organizationID))
		return err
	})
	return result, err
}

type dataPolicyScanner interface {
	Scan(dest ...any) error
}

func scanDataPolicy(scanner dataPolicyScanner) (securitypolicy.Record, error) {
	var item securitypolicy.Record
	var tagsJSON string
	var classification, mask string
	if err := scanner.Scan(&item.ID, &item.OrganizationID, &item.Key, &classification, &item.Owner, &item.Purpose, &item.Residency, &item.RetentionDays, &tagsJSON, &mask, &item.ExportApproval, &item.Watermark, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return securitypolicy.Record{}, err
	}
	item.Classification = securitypolicy.Classification(classification)
	item.Mask = securitypolicy.MaskStrategy(mask)
	if err := json.Unmarshal([]byte(tagsJSON), &item.Tags); err != nil {
		return securitypolicy.Record{}, fmt.Errorf("decode data policy tags: %w", err)
	}
	return item, nil
}

func validDataPolicyKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, "\r\n")
}
