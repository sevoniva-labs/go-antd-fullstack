package datapolicy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sevoniva-labs/forge/internal/adapters/repository"
	identitydomain "github.com/sevoniva-labs/forge/internal/domain/identity"
	securitypolicy "github.com/sevoniva-labs/forge/internal/platform/security/datapolicy"
)

var (
	ErrInvalidPolicy             = errors.New("invalid data field policy")
	ErrNoExportFields            = errors.New("at least one data field is required for export")
	ErrOrganizationRequired      = errors.New("organization is required")
	ErrExportDownloadUnavailable = errors.New("export download control is unavailable")
	ErrExportActorRequired       = errors.New("interactive export actor is required")
)

type Service struct {
	repo      *repository.DataPolicyRepo
	downloads *securitypolicy.DownloadController
}

func NewService(repo *repository.DataPolicyRepo) *Service { return &Service{repo: repo} }

func (s *Service) ConfigureDownloadController(controller *securitypolicy.DownloadController) {
	s.downloads = controller
}

func (s *Service) List(ctx context.Context, actor identitydomain.Principal) ([]securitypolicy.Record, error) {
	if actor.OrganizationID == "" {
		return nil, ErrOrganizationRequired
	}
	return s.repo.List(ctx, actor.OrganizationID)
}

func (s *Service) Upsert(ctx context.Context, actor identitydomain.Principal, policy securitypolicy.FieldPolicy) (securitypolicy.Record, error) {
	if actor.OrganizationID == "" {
		return securitypolicy.Record{}, ErrOrganizationRequired
	}
	policy = normalize(policy)
	if err := s.Validate(policy); err != nil {
		return securitypolicy.Record{}, err
	}
	return s.repo.Upsert(ctx, actor.OrganizationID, policy)
}

func (s *Service) Validate(policy securitypolicy.FieldPolicy) error {
	policy = normalize(policy)
	if _, err := securitypolicy.NewCatalog([]securitypolicy.FieldPolicy{policy}); err != nil {
		return errors.Join(ErrInvalidPolicy, err)
	}
	return nil
}

func (s *Service) AuthorizeExport(ctx context.Context, actor identitydomain.Principal, keys []string, request securitypolicy.ExportRequest) error {
	if actor.OrganizationID == "" {
		return ErrOrganizationRequired
	}
	if len(keys) == 0 {
		return ErrNoExportFields
	}
	catalog, err := s.catalog(ctx, actor.OrganizationID)
	if err != nil {
		return err
	}
	for i := range keys {
		keys[i] = strings.TrimSpace(keys[i])
	}
	return catalog.AuthorizeExport(keys, request)
}

// RenderExport loads the actor's organization-scoped catalog before rendering
// an explicit business projection. The caller remains responsible for applying
// data-scope authorization to rows and for binding options to a one-time
// approval execution record.
func (s *Service) RenderExport(ctx context.Context, actor identitydomain.Principal, options securitypolicy.ExportOptions, rows []securitypolicy.ExportRow) (securitypolicy.ExportArtifact, error) {
	if actor.OrganizationID == "" {
		return securitypolicy.ExportArtifact{}, ErrOrganizationRequired
	}
	catalog, err := s.catalog(ctx, actor.OrganizationID)
	if err != nil {
		return securitypolicy.ExportArtifact{}, err
	}
	return catalog.RenderExport(options, rows)
}

// PublishExport renders an explicit server-side projection and creates a
// one-time download ticket. Domain modules must obtain rows through their own
// authorized query path; this method never accepts client-provided records.
func (s *Service) PublishExport(ctx context.Context, actor identitydomain.Principal, options securitypolicy.ExportOptions, rows []securitypolicy.ExportRow, expiresAt time.Time) (securitypolicy.DownloadArtifact, error) {
	if actor.OrganizationID == "" {
		return securitypolicy.DownloadArtifact{}, ErrOrganizationRequired
	}
	if actor.UserID == "" {
		return securitypolicy.DownloadArtifact{}, ErrExportActorRequired
	}
	if s.downloads == nil {
		return securitypolicy.DownloadArtifact{}, ErrExportDownloadUnavailable
	}
	artifact, err := s.RenderExport(ctx, actor, options, rows)
	if err != nil {
		return securitypolicy.DownloadArtifact{}, err
	}
	return s.downloads.Publish(ctx, actor.OrganizationID, actor.UserID, options.ApprovalID, artifact.ContentType, artifact.Content, expiresAt)
}

func (s *Service) OpenExport(ctx context.Context, actor identitydomain.Principal, artifactID string) (securitypolicy.DownloadArtifact, io.ReadCloser, error) {
	if actor.OrganizationID == "" {
		return securitypolicy.DownloadArtifact{}, nil, ErrOrganizationRequired
	}
	if actor.UserID == "" {
		return securitypolicy.DownloadArtifact{}, nil, ErrExportActorRequired
	}
	if s.downloads == nil {
		return securitypolicy.DownloadArtifact{}, nil, ErrExportDownloadUnavailable
	}
	return s.downloads.Open(ctx, actor.OrganizationID, artifactID, actor.UserID)
}

func (s *Service) RevokeExport(ctx context.Context, actor identitydomain.Principal, artifactID, reason string) error {
	if actor.OrganizationID == "" {
		return ErrOrganizationRequired
	}
	if actor.UserID == "" {
		return ErrExportActorRequired
	}
	if s.downloads == nil {
		return ErrExportDownloadUnavailable
	}
	return s.downloads.Revoke(ctx, actor.OrganizationID, artifactID, actor.UserID, reason)
}

func (s *Service) catalog(ctx context.Context, organizationID string) (*securitypolicy.Catalog, error) {
	records, err := s.repo.List(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	policies := make([]securitypolicy.FieldPolicy, 0, len(records))
	for _, record := range records {
		policies = append(policies, record.FieldPolicy)
	}
	return securitypolicy.NewCatalog(policies)
}

func (s *Service) ListDeletionEvidence(ctx context.Context, actor identitydomain.Principal) ([]securitypolicy.DeletionEvidence, error) {
	if actor.OrganizationID == "" {
		return nil, ErrOrganizationRequired
	}
	return s.repo.ListDeletionEvidence(ctx, actor.OrganizationID)
}

func (s *Service) ValidateDeletionEvidence(ctx context.Context, actor identitydomain.Principal, evidence securitypolicy.DeletionEvidence) error {
	if actor.OrganizationID == "" {
		return ErrOrganizationRequired
	}
	evidence = normalizeEvidence(evidence)
	if evidence.ResourceType == "" || len(evidence.ResourceType) > 160 || len(evidence.ResourceDigest) != 64 || evidence.Reason == "" || len(evidence.Reason) > 500 || evidence.RecordsDeleted < 0 || evidence.DeletedAt.IsZero() || evidence.DeletedAt.After(time.Now().UTC().Add(5*time.Minute)) || len(evidence.FieldKeys) == 0 {
		return ErrInvalidPolicy
	}
	if _, err := hex.DecodeString(evidence.ResourceDigest); err != nil {
		return ErrInvalidPolicy
	}
	records, err := s.repo.List(ctx, actor.OrganizationID)
	if err != nil {
		return err
	}
	policies := make([]securitypolicy.FieldPolicy, 0, len(records))
	for _, record := range records {
		policies = append(policies, record.FieldPolicy)
	}
	catalog, err := securitypolicy.NewCatalog(policies)
	if err != nil {
		return err
	}
	for _, key := range evidence.FieldKeys {
		if _, err := catalog.Policy(key); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) RecordDeletionEvidence(ctx context.Context, actor identitydomain.Principal, evidence securitypolicy.DeletionEvidence) (securitypolicy.DeletionEvidence, error) {
	evidence = normalizeEvidence(evidence)
	if err := s.ValidateDeletionEvidence(ctx, actor, evidence); err != nil {
		return securitypolicy.DeletionEvidence{}, err
	}
	evidence.ID = newEvidenceID()
	evidence.OrganizationID = actor.OrganizationID
	evidence.CreatedAt = time.Now().UTC()
	payload, err := json.Marshal([]any{evidence.OrganizationID, evidence.ResourceType, evidence.ResourceDigest, evidence.FieldKeys, evidence.Reason, evidence.RecordsDeleted, evidence.DeletedAt.UTC().Format(time.RFC3339Nano), evidence.ID})
	if err != nil {
		return securitypolicy.DeletionEvidence{}, err
	}
	digest := sha256.Sum256(payload)
	evidence.EvidenceHash = hex.EncodeToString(digest[:])
	return s.repo.RecordDeletionEvidence(ctx, evidence)
}

func normalize(policy securitypolicy.FieldPolicy) securitypolicy.FieldPolicy {
	policy.Key = strings.TrimSpace(policy.Key)
	policy.Owner = strings.TrimSpace(policy.Owner)
	policy.Purpose = strings.TrimSpace(policy.Purpose)
	policy.Residency = strings.TrimSpace(policy.Residency)
	policy.Mask = securitypolicy.MaskStrategy(strings.TrimSpace(string(policy.Mask)))
	tags := make([]string, 0, len(policy.Tags))
	for _, tag := range policy.Tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	policy.Tags = tags
	return policy
}

func normalizeEvidence(evidence securitypolicy.DeletionEvidence) securitypolicy.DeletionEvidence {
	evidence.ResourceType = strings.TrimSpace(evidence.ResourceType)
	evidence.ResourceDigest = strings.ToLower(strings.TrimSpace(evidence.ResourceDigest))
	evidence.Reason = strings.TrimSpace(evidence.Reason)
	keys := make([]string, 0, len(evidence.FieldKeys))
	seen := make(map[string]struct{}, len(evidence.FieldKeys))
	for _, key := range evidence.FieldKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	// Keep request order for operator display; the repository hash includes the
	// normalized list so callers cannot create duplicate keys in one proof.
	evidence.FieldKeys = keys
	return evidence
}

func newEvidenceID() string {
	return uuid.NewString()
}
