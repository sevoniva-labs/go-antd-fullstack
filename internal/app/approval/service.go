package approval

import (
	"bytes"
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
	identityapp "github.com/sevoniva-labs/forge/internal/app/identity"
	domain "github.com/sevoniva-labs/forge/internal/domain/approval"
	identitydomain "github.com/sevoniva-labs/forge/internal/domain/identity"
)

type Service struct{ repo *repository.ApprovalRepo }

var ErrAccessDenied = errors.New("approval access denied")

func NewService(repo *repository.ApprovalRepo) *Service { return &Service{repo: repo} }

type CreateInput struct {
	RequestType       string
	Action            string
	Resource          string
	ResourceID        string
	Summary           string
	PayloadJSON       string
	Mode              string
	RequiredApprovals int
	ApproverIDs       []string
	ExpiresIn         time.Duration
}

func (s *Service) Create(ctx context.Context, actor identitydomain.Principal, input CreateInput) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return domain.Request{}, err
	}
	input.RequestType = strings.TrimSpace(input.RequestType)
	input.Action = strings.TrimSpace(input.Action)
	input.Resource = strings.TrimSpace(input.Resource)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.Summary = strings.TrimSpace(input.Summary)
	input.Mode = strings.ToUpper(strings.TrimSpace(input.Mode))
	if len(input.PayloadJSON) == 0 || len(input.PayloadJSON) > 64*1024 || len(input.RequestType) > 100 || len(input.Action) > 160 || len(input.Resource) > 160 || len(input.ResourceID) > 160 || len(input.Summary) > 500 || input.ExpiresIn < time.Minute || input.ExpiresIn > 30*24*time.Hour {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	canonical, err := canonicalJSON([]byte(input.PayloadJSON))
	if err != nil {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	digestInput, _ := json.Marshal([]any{input.RequestType, input.Action, input.Resource, input.ResourceID, canonical})
	digest := sha256.Sum256(digestInput)
	now := time.Now().UTC()
	request := domain.Request{ID: uuid.NewString(), OrganizationID: actor.OrganizationID, RequestType: input.RequestType, Action: input.Action, Resource: input.Resource, ResourceID: input.ResourceID, Summary: input.Summary, RequestDigest: hex.EncodeToString(digest[:]), ApplicantID: actor.UserID, Mode: input.Mode, RequiredApprovals: input.RequiredApprovals, Status: domain.StatusPending, ExpiresAt: now.Add(input.ExpiresIn), CreatedAt: now, UpdatedAt: now}
	if err := domain.ValidateCreation(request, input.ApproverIDs); err != nil {
		return domain.Request{}, err
	}
	return s.repo.Create(ctx, request, input.ApproverIDs)
}

func (s *Service) Get(ctx context.Context, actor identitydomain.Principal, requestID string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	request, err := s.repo.ByID(ctx, actor.OrganizationID, strings.TrimSpace(requestID))
	if err != nil {
		return domain.Request{}, err
	}
	if !repository.IsApprovalParticipant(request, actor.UserID) && !actor.HasRole("system_admin", "auditor") {
		return domain.Request{}, ErrAccessDenied
	}
	return request, nil
}

func (s *Service) Decide(ctx context.Context, actor identitydomain.Principal, requestID, decision, comment string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return domain.Request{}, err
	}
	decision = strings.ToUpper(strings.TrimSpace(decision))
	if (decision != domain.DecisionApprove && decision != domain.DecisionReject) || len(comment) > 1000 {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	return s.repo.Decide(ctx, actor.OrganizationID, strings.TrimSpace(requestID), actor.UserID, decision, strings.TrimSpace(comment))
}

func (s *Service) Transfer(ctx context.Context, actor identitydomain.Principal, requestID, newAssigneeID, comment string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if err := identityapp.RequireRecentMFA(actor); err != nil {
		return domain.Request{}, err
	}
	if newAssigneeID = strings.TrimSpace(newAssigneeID); newAssigneeID == "" || len(comment) > 1000 {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	return s.repo.Transfer(ctx, actor.OrganizationID, strings.TrimSpace(requestID), actor.UserID, newAssigneeID, strings.TrimSpace(comment))
}

func (s *Service) Withdraw(ctx context.Context, actor identitydomain.Principal, requestID, comment string) (domain.Request, error) {
	if err := requireActor(actor); err != nil {
		return domain.Request{}, err
	}
	if len(comment) > 1000 {
		return domain.Request{}, domain.ErrInvalidRequest
	}
	return s.repo.Withdraw(ctx, actor.OrganizationID, strings.TrimSpace(requestID), actor.UserID, strings.TrimSpace(comment))
}

func requireActor(actor identitydomain.Principal) error {
	if actor.Type != "USER" || actor.UserID == "" || actor.OrganizationID == "" {
		return identityapp.ErrInteractiveSessionRequired
	}
	return nil
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, domain.ErrInvalidRequest
		}
		return nil, err
	}
	return json.Marshal(value)
}
