package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
)

const unknownRootCause = "Unknown anomaly detected"

type CopilotAnalysisRequest struct {
	IncidentID     string
	ServiceName    string
	ErrorLogs      string
	FiringRule     string
	Environment    string
	Commit         domain.CommitMetadata
	SourceSnippets []domain.SourceSnippet
	CIProvider     string
	BuildURL       string
}

type CopilotAnalysisResult struct {
	IncidentID          string
	RootCause           string
	ImmediateMitigation string
	ConfidenceScore     string
	Severity            string
	AffectedComponents  []string
	UnifiedDiff         string
	VerificationPlan    string
	RollbackPlan        string
	CitedSourcePaths    []string
}

type CopilotRemediationRequest struct {
	IncidentID   string
	RootCause    string
	TargetSystem string
}

type CopilotRemediationResult struct {
	ScriptType             string
	ExecutableScript       string
	RequiresManualApproval bool
	UnifiedDiff            string
	VerificationPlan       string
	RollbackPlan           string
}

type CopilotClient interface {
	AnalyzeIncident(context.Context, CopilotAnalysisRequest) (CopilotAnalysisResult, error)
	GenerateRemediation(context.Context, CopilotRemediationRequest) (CopilotRemediationResult, error)
}

type CreateIncidentInput struct {
	Title          string
	ServiceName    string
	RawLogs        *string
	FiringRule     *string
	Environment    *string
	CreatedBy      *string
	SourceEventKey *string
}

type ApproveRemediationInput struct {
	IncidentID    string
	RemediationID string
	AppliedBy     string
}

type TriageResult struct {
	IncidentID          string
	RootCause           string
	ImmediateMitigation string
	ConfidenceScore     string
	Severity            string
	AffectedComponents  []string
	IncidentStatus      domain.IncidentStatus
	UnifiedDiff         string
	VerificationPlan    string
	RollbackPlan        string
	CitedSourcePaths    []string
	RepositoryURL       string
	CommitSHA           string
}

type RemediationResult struct {
	RemediationID          string
	IncidentID             string
	ScriptType             string
	ExecutableScript       string
	RequiresManualApproval bool
	ExecutionStatus        domain.ExecutionStatus
	UnifiedDiff            string
	VerificationPlan       string
	RollbackPlan           string
}

// TriageEvidence is optional, read-only context collected from a configured
// repository and CI run. Secrets are deliberately excluded.
type TriageEvidence struct {
	Commit         domain.CommitMetadata
	SourceSnippets []domain.SourceSnippet
	CIProvider     string
	BuildURL       string
}

type IncidentService struct {
	store      store.IncidentStore
	copilot    CopilotClient
	clock      Clock
	ids        IDGenerator
	rpcTimeout time.Duration
}

func NewIncidentService(repository store.IncidentStore, copilot CopilotClient, clock Clock, ids IDGenerator, rpcTimeout time.Duration) *IncidentService {
	return &IncidentService{store: repository, copilot: copilot, clock: clock, ids: ids, rpcTimeout: rpcTimeout}
}

func (s *IncidentService) CreateIncident(ctx context.Context, input CreateIncidentInput) (domain.Incident, error) {
	now := s.clock()
	environment := "production"
	if input.Environment != nil {
		environment = *input.Environment
	}
	createdBy := "system"
	if input.CreatedBy != nil {
		createdBy = *input.CreatedBy
	}
	incident := domain.Incident{
		ID:             s.ids(),
		Title:          input.Title,
		ServiceName:    input.ServiceName,
		RawLogs:        input.RawLogs,
		FiringRule:     input.FiringRule,
		Environment:    environment,
		Status:         domain.IncidentOpen,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceEventKey: input.SourceEventKey,
	}
	created, err := s.store.CreateIncident(ctx, incident)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("create incident: %w", err)
	}
	return created, nil
}

func (s *IncidentService) ListIncidents(ctx context.Context, page, size int) (domain.Page[domain.Incident], error) {
	if err := validatePage(page, size); err != nil {
		return domain.Page[domain.Incident]{}, err
	}
	return s.store.ListIncidents(ctx, page, size)
}

func (s *IncidentService) ActiveIncidents(ctx context.Context) ([]domain.Incident, error) {
	return s.store.ListActiveIncidents(ctx)
}

func (s *IncidentService) Incident(ctx context.Context, id string) (domain.Incident, error) {
	incident, err := s.store.GetIncident(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return domain.Incident{}, errorWith(CodeNotFound, "Incident not found: "+id, err)
	}
	if err != nil {
		return domain.Incident{}, fmt.Errorf("get incident: %w", err)
	}
	return incident, nil
}

func (s *IncidentService) LatestAnalysis(ctx context.Context, incidentID string) (*domain.IncidentAnalysis, error) {
	if _, err := s.Incident(ctx, incidentID); err != nil {
		return nil, err
	}
	analysis, err := s.store.LatestAnalysis(ctx, incidentID)
	if err != nil {
		return nil, fmt.Errorf("load latest triage: %w", err)
	}
	return analysis, nil
}

func (s *IncidentService) Triage(ctx context.Context, incidentID string) (TriageResult, error) {
	return s.TriageWithEvidence(ctx, incidentID, TriageEvidence{})
}

func (s *IncidentService) TriageWithEvidence(ctx context.Context, incidentID string, evidence TriageEvidence) (TriageResult, error) {
	incident, err := s.Incident(ctx, incidentID)
	if err != nil {
		return TriageResult{}, err
	}
	if err := s.store.SetIncidentStatus(ctx, incidentID, domain.IncidentAnalyzing, s.clock()); err != nil {
		return TriageResult{}, fmt.Errorf("mark incident analyzing: %w", err)
	}

	errorLogs := ""
	if incident.RawLogs != nil {
		errorLogs = *incident.RawLogs
	}
	firingRule := "ManualTriage"
	if incident.FiringRule != nil {
		firingRule = *incident.FiringRule
	}
	rpcContext, cancel := context.WithTimeout(ctx, s.rpcTimeout)
	result, rpcErr := s.copilot.AnalyzeIncident(rpcContext, CopilotAnalysisRequest{
		IncidentID: incident.ID, ServiceName: incident.ServiceName, ErrorLogs: errorLogs,
		FiringRule: firingRule, Environment: incident.Environment, Commit: evidence.Commit,
		SourceSnippets: append([]domain.SourceSnippet(nil), evidence.SourceSnippets...),
		CIProvider:     evidence.CIProvider, BuildURL: evidence.BuildURL,
	})
	cancel()
	if rpcErr != nil {
		s.restoreStatus(ctx, incident.ID, incident.Status)
		return TriageResult{}, errorWith(CodeUnavailable,
			"AI Copilot service is unreachable. Ensure ai-copilot-service is running on port 9090.", rpcErr)
	}

	now := s.clock()
	analysis := domain.IncidentAnalysis{
		ID:                  s.ids(),
		IncidentID:          incident.ID,
		RootCause:           result.RootCause,
		ImmediateMitigation: result.ImmediateMitigation,
		ConfidenceScore:     result.ConfidenceScore,
		Severity:            result.Severity,
		AffectedComponents:  append([]string(nil), result.AffectedComponents...),
		UnifiedDiff:         result.UnifiedDiff,
		VerificationPlan:    result.VerificationPlan,
		RollbackPlan:        result.RollbackPlan,
		CitedSourcePaths:    append([]string(nil), result.CitedSourcePaths...),
		RepositoryURL:       evidence.Commit.Repository,
		CommitSHA:           evidence.Commit.SHA,
		CommitMessage:       evidence.Commit.Message,
		RepositoryProvider:  evidence.Commit.Provider,
		TargetBranch:        evidence.Commit.Branch,
		CIProvider:          evidence.CIProvider,
		BuildURL:            evidence.BuildURL,
		CreatedAt:           now,
	}
	if err := s.store.CompleteTriage(ctx, analysis, domain.NormalizeSeverity(result.Severity), now); err != nil {
		s.restoreStatus(ctx, incident.ID, incident.Status)
		return TriageResult{}, fmt.Errorf("persist triage result: %w", err)
	}
	return TriageResult{
		IncidentID:          result.IncidentID,
		RootCause:           result.RootCause,
		ImmediateMitigation: result.ImmediateMitigation,
		ConfidenceScore:     result.ConfidenceScore,
		Severity:            result.Severity,
		AffectedComponents:  append([]string(nil), result.AffectedComponents...),
		IncidentStatus:      domain.IncidentAnalyzing,
		UnifiedDiff:         result.UnifiedDiff,
		VerificationPlan:    result.VerificationPlan,
		RollbackPlan:        result.RollbackPlan,
		CitedSourcePaths:    append([]string(nil), result.CitedSourcePaths...),
		RepositoryURL:       evidence.Commit.Repository,
		CommitSHA:           evidence.Commit.SHA,
	}, nil
}

func (s *IncidentService) GenerateRemediation(ctx context.Context, incidentID string) (RemediationResult, error) {
	incident, err := s.Incident(ctx, incidentID)
	if err != nil {
		return RemediationResult{}, err
	}
	rootCause := unknownRootCause
	analysis, err := s.store.LatestAnalysis(ctx, incidentID)
	if err != nil {
		return RemediationResult{}, fmt.Errorf("load latest triage: %w", err)
	}
	if analysis != nil && analysis.RootCause != "" {
		rootCause = analysis.RootCause
	}

	rpcContext, cancel := context.WithTimeout(ctx, s.rpcTimeout)
	result, rpcErr := s.copilot.GenerateRemediation(rpcContext, CopilotRemediationRequest{
		IncidentID:   incidentID,
		RootCause:    rootCause,
		TargetSystem: incident.ServiceName,
	})
	cancel()
	if rpcErr != nil {
		return RemediationResult{}, errorWith(CodeUnavailable,
			"AI Copilot service is unavailable. Cannot generate remediation script.", rpcErr)
	}

	now := s.clock()
	remediation := domain.Remediation{
		ID:               s.ids(),
		IncidentID:       incidentID,
		AIRootCause:      stringPointer(rootCause),
		SuggestedAction:  stringPointer(result.ExecutableScript),
		ScriptType:       stringPointer(result.ScriptType),
		ExecutableScript: stringPointer(result.ExecutableScript),
		RequiresApproval: result.RequiresManualApproval,
		ExecutionStatus:  domain.ExecutionPending,
		CreatedAt:        now,
		UnifiedDiff:      stringPointer(result.UnifiedDiff),
		VerificationPlan: stringPointer(result.VerificationPlan),
		RollbackPlan:     stringPointer(result.RollbackPlan),
	}
	if result.UnifiedDiff == "" && analysis != nil {
		remediation.UnifiedDiff = stringPointer(analysis.UnifiedDiff)
	}
	if result.VerificationPlan == "" && analysis != nil {
		remediation.VerificationPlan = stringPointer(analysis.VerificationPlan)
	}
	if result.RollbackPlan == "" && analysis != nil {
		remediation.RollbackPlan = stringPointer(analysis.RollbackPlan)
	}
	saved, err := s.store.SaveRemediation(ctx, remediation)
	if err != nil {
		return RemediationResult{}, fmt.Errorf("save remediation: %w", err)
	}
	return remediationResult(saved), nil
}

func (s *IncidentService) ApproveRemediation(ctx context.Context, input ApproveRemediationInput) (RemediationResult, error) {
	remediation, err := s.store.ApproveRemediation(ctx, input.IncidentID, input.RemediationID, input.AppliedBy, s.clock())
	if errors.Is(err, store.ErrNotFound) {
		return RemediationResult{}, errorWith(CodeNotFound, "Remediation record not found: "+input.RemediationID, err)
	}
	if errors.Is(err, store.ErrIncidentMismatch) {
		return RemediationResult{}, errorWith(CodeInvalid,
			"Remediation record does not belong to incident: "+input.IncidentID, err)
	}
	if err != nil {
		return RemediationResult{}, fmt.Errorf("apply remediation: %w", err)
	}
	return remediationResult(remediation), nil
}

func (s *IncidentService) Stats(ctx context.Context) (domain.DashboardStats, error) {
	now := s.clock()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return s.store.DashboardStats(ctx, start, now)
}

func (s *IncidentService) restoreStatus(parent context.Context, incidentID string, status domain.IncidentStatus) {
	restoreContext, cancel := context.WithTimeout(context.WithoutCancel(parent), 3*time.Second)
	defer cancel()
	_ = s.store.SetIncidentStatus(restoreContext, incidentID, status, s.clock())
}

func remediationResult(remediation domain.Remediation) RemediationResult {
	result := RemediationResult{
		RemediationID:          remediation.ID,
		IncidentID:             remediation.IncidentID,
		RequiresManualApproval: remediation.RequiresApproval,
		ExecutionStatus:        remediation.ExecutionStatus,
	}
	if remediation.ScriptType != nil {
		result.ScriptType = *remediation.ScriptType
	}
	if remediation.ExecutableScript != nil {
		result.ExecutableScript = *remediation.ExecutableScript
	}
	if remediation.UnifiedDiff != nil {
		result.UnifiedDiff = *remediation.UnifiedDiff
	}
	if remediation.VerificationPlan != nil {
		result.VerificationPlan = *remediation.VerificationPlan
	}
	if remediation.RollbackPlan != nil {
		result.RollbackPlan = *remediation.RollbackPlan
	}
	return result
}

func validatePage(page, size int) error {
	if page < 0 {
		return errorWith(CodeInvalid, "page must be greater than or equal to 0", nil)
	}
	if size < 1 || size > 200 {
		return errorWith(CodeInvalid, "size must be between 1 and 200", nil)
	}
	return nil
}
