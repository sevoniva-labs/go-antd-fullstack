package datapolicy

import (
	"context"
	"errors"
	"strings"

	"github.com/sevoniva-labs/forge/internal/adapters/repository"
	identitydomain "github.com/sevoniva-labs/forge/internal/domain/identity"
	securitypolicy "github.com/sevoniva-labs/forge/internal/platform/security/datapolicy"
)

var (
	ErrInvalidPolicy        = errors.New("invalid data field policy")
	ErrNoExportFields       = errors.New("at least one data field is required for export")
	ErrOrganizationRequired = errors.New("organization is required")
)

type Service struct {
	repo *repository.DataPolicyRepo
}

func NewService(repo *repository.DataPolicyRepo) *Service { return &Service{repo: repo} }

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
	for i := range keys {
		keys[i] = strings.TrimSpace(keys[i])
	}
	return catalog.AuthorizeExport(keys, request)
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
