package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
)

func TestTriagePersistsAnalysisAndNormalizesStoredSeverity(t *testing.T) {
	rawLogs := "connection refused"
	firingRule := "HighErrorRate"
	repository := &fakeIncidentStore{incident: domain.Incident{
		ID: "incident-1", ServiceName: "payments", RawLogs: &rawLogs, FiringRule: &firingRule,
		Environment: "production", Status: domain.IncidentOpen,
	}}
	copilot := &fakeCopilot{analysis: CopilotAnalysisResult{
		IncidentID: "incident-1", RootCause: "bad pool", ImmediateMitigation: "restart",
		ConfidenceScore: "0.94", Severity: "unexpected", AffectedComponents: []string{"postgres"},
		UnifiedDiff: "--- a/pool.go", VerificationPlan: "go test ./...", RollbackPlan: "git revert",
		CitedSourcePaths: []string{"pool.go"},
	}}
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := NewIncidentService(repository, copilot, func() time.Time { return now }, sequenceIDs("analysis-1"), time.Second)

	evidence := TriageEvidence{
		Commit:         domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/payments", Branch: "main", SHA: "abc123", Message: "pool change"},
		SourceSnippets: []domain.SourceSnippet{{Path: "pool.go", Content: "package pool", StartLine: 1, EndLine: 1}},
		CIProvider:     "GITHUB_ACTIONS", BuildURL: "https://github.com/acme/payments/actions/runs/1",
	}
	result, err := service.TriageWithEvidence(context.Background(), "incident-1", evidence)
	if err != nil {
		t.Fatalf("Triage() error = %v", err)
	}
	if result.RootCause != "bad pool" || result.Severity != "unexpected" || result.IncidentStatus != domain.IncidentAnalyzing {
		t.Fatalf("unexpected triage result: %+v", result)
	}
	if !copilot.analysisHadDeadline {
		t.Fatal("AnalyzeIncident context did not have an RPC deadline")
	}
	if copilot.analysisRequest.ErrorLogs != rawLogs || copilot.analysisRequest.FiringRule != firingRule {
		t.Fatalf("unexpected copilot request: %+v", copilot.analysisRequest)
	}
	if copilot.analysisRequest.Commit.SHA != "abc123" || len(copilot.analysisRequest.SourceSnippets) != 1 {
		t.Fatalf("source evidence was not sent to copilot: %+v", copilot.analysisRequest)
	}
	if repository.completedAnalysis == nil || repository.completedAnalysis.RootCause != "bad pool" || repository.completedAnalysis.ConfidenceScore != "0.94" {
		t.Fatalf("triage analysis was not persisted: %+v", repository.completedAnalysis)
	}
	if repository.completedAnalysis.UnifiedDiff == "" || repository.completedAnalysis.RepositoryURL != evidence.Commit.Repository || repository.completedAnalysis.CommitSHA != "abc123" {
		t.Fatalf("source-aware analysis was not persisted: %+v", repository.completedAnalysis)
	}
	if result.RepositoryURL != evidence.Commit.Repository || result.CommitSHA != "abc123" || result.UnifiedDiff == "" {
		t.Fatalf("source-aware triage response = %+v", result)
	}
	if repository.completedSeverity != domain.SeverityHigh {
		t.Fatalf("stored severity = %q, want HIGH", repository.completedSeverity)
	}
	if len(repository.statusChanges) != 1 || repository.statusChanges[0] != domain.IncidentAnalyzing {
		t.Fatalf("status changes = %v", repository.statusChanges)
	}
}

func TestTriageFailureRestoresPreviousStatus(t *testing.T) {
	repository := &fakeIncidentStore{incident: domain.Incident{ID: "incident-1", ServiceName: "api", Environment: "production", Status: domain.IncidentResolved}}
	copilot := &fakeCopilot{analysisErr: errors.New("offline")}
	service := NewIncidentService(repository, copilot, time.Now, sequenceIDs("unused"), 10*time.Millisecond)

	_, err := service.Triage(context.Background(), "incident-1")
	if !IsCode(err, CodeUnavailable) {
		t.Fatalf("Triage() error = %v, want unavailable", err)
	}
	if len(repository.statusChanges) != 2 || repository.statusChanges[0] != domain.IncidentAnalyzing || repository.statusChanges[1] != domain.IncidentResolved {
		t.Fatalf("status changes = %v, want ANALYZING then RESOLVED", repository.statusChanges)
	}
}

func TestGenerateRemediationUsesPersistedTriageRootCause(t *testing.T) {
	repository := &fakeIncidentStore{
		incident:       domain.Incident{ID: "incident-1", ServiceName: "orders", Environment: "production", Status: domain.IncidentAnalyzing},
		latestAnalysis: &domain.IncidentAnalysis{RootCause: "connection pool exhausted"},
	}
	copilot := &fakeCopilot{remediation: CopilotRemediationResult{
		ScriptType: "BASH", ExecutableScript: "restart orders", RequiresManualApproval: true,
		UnifiedDiff: "--- a/deploy.yaml", VerificationPlan: "kubectl rollout status", RollbackPlan: "kubectl rollout undo",
	}}
	service := NewIncidentService(repository, copilot, time.Now, sequenceIDs("remediation-1"), time.Second)

	result, err := service.GenerateRemediation(context.Background(), "incident-1")
	if err != nil {
		t.Fatalf("GenerateRemediation() error = %v", err)
	}
	if copilot.remediationRequest.RootCause != "connection pool exhausted" {
		t.Fatalf("root cause sent to copilot = %q", copilot.remediationRequest.RootCause)
	}
	if repository.savedRemediation == nil || repository.savedRemediation.AIRootCause == nil || *repository.savedRemediation.AIRootCause != "connection pool exhausted" {
		t.Fatalf("saved remediation = %+v", repository.savedRemediation)
	}
	if repository.savedRemediation.UnifiedDiff == nil || *repository.savedRemediation.UnifiedDiff == "" || result.RollbackPlan == "" {
		t.Fatalf("structured remediation was not persisted: saved=%+v result=%+v", repository.savedRemediation, result)
	}
	if result.ExecutionStatus != domain.ExecutionPending || result.ExecutableScript != "restart orders" {
		t.Fatalf("unexpected remediation result: %+v", result)
	}
}

func TestApproveRemediationReturnsApprovedWithoutClaimingExecution(t *testing.T) {
	repository := &fakeIncidentStore{incident: domain.Incident{ID: "incident-1"}}
	repository.applyResult = domain.Remediation{
		ID: "remediation-1", IncidentID: "incident-1", ScriptType: stringPointer("BASH"),
		ExecutableScript: stringPointer("echo ok"), RequiresApproval: true, ExecutionStatus: domain.ExecutionApproved,
	}
	service := NewIncidentService(repository, &fakeCopilot{}, time.Now, sequenceIDs(), time.Second)

	result, err := service.ApproveRemediation(context.Background(), ApproveRemediationInput{
		IncidentID: "incident-1", RemediationID: "remediation-1", AppliedBy: "RamanRed",
	})
	if err != nil {
		t.Fatalf("ApproveRemediation() error = %v", err)
	}
	if result.ExecutionStatus != domain.ExecutionApproved || repository.appliedBy != "RamanRed" {
		t.Fatalf("result=%+v appliedBy=%q", result, repository.appliedBy)
	}
}

type fakeCopilot struct {
	analysis            CopilotAnalysisResult
	analysisErr         error
	analysisRequest     CopilotAnalysisRequest
	analysisHadDeadline bool
	remediation         CopilotRemediationResult
	remediationErr      error
	remediationRequest  CopilotRemediationRequest
}

func (f *fakeCopilot) AnalyzeIncident(ctx context.Context, request CopilotAnalysisRequest) (CopilotAnalysisResult, error) {
	f.analysisRequest = request
	_, f.analysisHadDeadline = ctx.Deadline()
	return f.analysis, f.analysisErr
}

func (f *fakeCopilot) GenerateRemediation(_ context.Context, request CopilotRemediationRequest) (CopilotRemediationResult, error) {
	f.remediationRequest = request
	return f.remediation, f.remediationErr
}

type fakeIncidentStore struct {
	incident          domain.Incident
	statusChanges     []domain.IncidentStatus
	completedAnalysis *domain.IncidentAnalysis
	completedSeverity domain.IncidentSeverity
	latestAnalysis    *domain.IncidentAnalysis
	savedRemediation  *domain.Remediation
	applyResult       domain.Remediation
	applyErr          error
	appliedBy         string
}

func (f *fakeIncidentStore) CreateIncident(_ context.Context, incident domain.Incident) (domain.Incident, error) {
	f.incident = incident
	return incident, nil
}
func (f *fakeIncidentStore) GetIncident(_ context.Context, id string) (domain.Incident, error) {
	if f.incident.ID != id {
		return domain.Incident{}, store.ErrNotFound
	}
	return f.incident, nil
}
func (f *fakeIncidentStore) ListIncidents(_ context.Context, page, size int) (domain.Page[domain.Incident], error) {
	return domain.Page[domain.Incident]{Content: []domain.Incident{f.incident}, TotalElements: 1, Page: page, Size: size}, nil
}
func (f *fakeIncidentStore) ListActiveIncidents(context.Context) ([]domain.Incident, error) {
	return []domain.Incident{f.incident}, nil
}
func (f *fakeIncidentStore) SetIncidentStatus(_ context.Context, _ string, status domain.IncidentStatus, _ time.Time) error {
	f.statusChanges = append(f.statusChanges, status)
	return nil
}
func (f *fakeIncidentStore) CompleteTriage(_ context.Context, analysis domain.IncidentAnalysis, severity domain.IncidentSeverity, _ time.Time) error {
	f.completedAnalysis = &analysis
	f.completedSeverity = severity
	return nil
}
func (f *fakeIncidentStore) LatestAnalysis(context.Context, string) (*domain.IncidentAnalysis, error) {
	return f.latestAnalysis, nil
}
func (f *fakeIncidentStore) SaveRemediation(_ context.Context, remediation domain.Remediation) (domain.Remediation, error) {
	f.savedRemediation = &remediation
	return remediation, nil
}
func (f *fakeIncidentStore) ApproveRemediation(_ context.Context, _, _, appliedBy string, _ time.Time) (domain.Remediation, error) {
	f.appliedBy = appliedBy
	return f.applyResult, f.applyErr
}
func (f *fakeIncidentStore) DashboardStats(context.Context, time.Time, time.Time) (domain.DashboardStats, error) {
	return domain.DashboardStats{}, nil
}

func sequenceIDs(values ...string) IDGenerator {
	index := 0
	return func() string {
		if index >= len(values) {
			return "generated-id"
		}
		value := values[index]
		index++
		return value
	}
}
