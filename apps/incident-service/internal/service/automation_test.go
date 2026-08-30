package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

func TestAutomationRetriesCommitAfterTransientTriggerFailure(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	id := int64(7)
	status, cadence, engine := "CONNECTED", "5_MINUTES", "JENKINS"
	integration := domain.PlatformIntegration{
		ID: &id, UserID: "operator", ConnectionStatus: &status, PollingCadence: &cadence,
		PipelineEngine: &engine, AutoRebuild: true,
	}
	repository := newFakeAutomationRepository(integration)
	platform := &fakeAutomationPlatform{
		commit:        domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/repo", Branch: "main", SHA: "new-commit"},
		triggerErrors: []error{errors.New("Jenkins temporarily unavailable"), nil},
		build:         domain.CIBuild{Provider: "JENKINS", ID: "42", Number: 42, Status: "SUCCESS", URL: "https://ci.example/42"},
	}
	pipelines := &fakeAutomationPipelines{}
	runner := NewAutomationRunner(repository, platform, &fakeAutomationIncidents{}, pipelines,
		func() time.Time { return now }, AutomationConfig{BuildPollInterval: time.Millisecond, BuildTimeout: time.Second}, discardLogger())

	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if repository.processingState != "FAILED" || platform.triggerCalls != 1 || len(pipelines.inputs) != 0 {
		t.Fatalf("first attempt state=%s triggers=%d builds=%d", repository.processingState, platform.triggerCalls, len(pipelines.inputs))
	}
	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if repository.processingState != "PROCESSED" || platform.triggerCalls != 2 || len(pipelines.inputs) != 1 {
		t.Fatalf("retry state=%s triggers=%d builds=%d", repository.processingState, platform.triggerCalls, len(pipelines.inputs))
	}
}

func TestAutomationBaselinesExistingCommitWithoutHistoricalRebuild(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	id := int64(8)
	status, cadence, sha := "CONNECTED", "15_MINUTES", "baseline-sha"
	integration := domain.PlatformIntegration{
		ID: &id, UserID: "operator", ConnectionStatus: &status, PollingCadence: &cadence,
		LastPolledCommit: &sha, AutoRebuild: true,
	}
	repository := newFakeAutomationRepository(integration)
	platform := &fakeAutomationPlatform{commit: domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/repo", Branch: "main", SHA: sha}}
	runner := NewAutomationRunner(repository, platform, &fakeAutomationIncidents{}, &fakeAutomationPipelines{},
		func() time.Time { return now }, AutomationConfig{}, discardLogger())

	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if platform.triggerCalls != 0 || repository.processingState != "PROCESSED" || repository.lastStatus != "CONNECTED" {
		t.Fatalf("baseline triggered old build: triggers=%d state=%s status=%s", platform.triggerCalls, repository.processingState, repository.lastStatus)
	}
}

func TestAutomationIngestsFailureLogsAndSourceEvidence(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	id := int64(9)
	status, cadence, engine := "CONNECTED", "15_MINUTES", "GITHUB_ACTIONS"
	integration := domain.PlatformIntegration{
		ID: &id, UserID: "operator", ConnectionStatus: &status, PollingCadence: &cadence,
		PipelineEngine: &engine, AutoRebuild: true, AutoAITriage: true,
	}
	repository := newFakeAutomationRepository(integration)
	platform := &fakeAutomationPlatform{
		commit: domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/repo", Branch: "main", SHA: "bad-sha", Message: "break build"},
		build:  domain.CIBuild{Provider: "GITHUB_ACTIONS", ID: "77", Number: 77, Status: "FAILURE", URL: "https://github.com/acme/repo/actions/runs/77", Logs: "/github/workspace/apps/api/main.go:42: undefined symbol"},
	}
	incidents := &fakeAutomationIncidents{}
	pipelines := &fakeAutomationPipelines{}
	runner := NewAutomationRunner(repository, platform, incidents, pipelines,
		func() time.Time { return now }, AutomationConfig{MaxSourceFiles: 5}, discardLogger())

	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(incidents.created) != 1 || incidents.created[0].RawLogs == nil || len(incidents.triageEvidence.SourceSnippets) != 1 {
		t.Fatalf("incident/evidence not ingested: created=%+v evidence=%+v", incidents.created, incidents.triageEvidence)
	}
	if incidents.triageEvidence.SourceSnippets[0].Path != "apps/api/main.go" || incidents.triageEvidence.Commit.SHA != "bad-sha" {
		t.Fatalf("unexpected triage evidence: %+v", incidents.triageEvidence)
	}
	if len(pipelines.inputs) != 1 || pipelines.inputs[0].Status != "FAILURE" {
		t.Fatalf("failed build not persisted: %+v", pipelines.inputs)
	}
}

func TestAutomationRetryAfterCompletionWriteDoesNotDuplicateBuildOrIncident(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	id := int64(10)
	status, cadence, engine := "CONNECTED", "15_MINUTES", "JENKINS"
	repository := newFakeAutomationRepository(domain.PlatformIntegration{
		ID: &id, UserID: "operator", ConnectionStatus: &status, PollingCadence: &cadence,
		PipelineEngine: &engine, AutoRebuild: true, AutoAITriage: true,
	})
	repository.finishSuccessErrors = []error{errors.New("transient commit completion failure"), nil}
	platform := &fakeAutomationPlatform{
		commit: domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/repo", Branch: "main", SHA: "idempotent-sha"},
		build:  domain.CIBuild{Provider: "JENKINS", ID: "55", Number: 55, Status: "FAILURE", Logs: "src/main.go:5: failed"},
	}
	incidents := &fakeAutomationIncidents{}
	pipelines := &fakeAutomationPipelines{}
	runner := NewAutomationRunner(repository, platform, incidents, pipelines, func() time.Time { return now }, AutomationConfig{}, discardLogger())
	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if platform.triggerCalls != 1 || len(pipelines.inputs) != 1 || len(incidents.created) != 1 || incidents.triageCalls != 1 {
		t.Fatalf("retry duplicated effects: triggers=%d builds=%d incidents=%d triages=%d", platform.triggerCalls, len(pipelines.inputs), len(incidents.created), incidents.triageCalls)
	}
}

func TestAutomationRetriesTransientTriageFailureWithoutDuplicateEffects(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	id := int64(11)
	status, cadence, engine := "CONNECTED", "15_MINUTES", "JENKINS"
	repository := newFakeAutomationRepository(domain.PlatformIntegration{
		ID: &id, UserID: "operator", ConnectionStatus: &status, PollingCadence: &cadence,
		PipelineEngine: &engine, AutoRebuild: true, AutoAITriage: true,
	})
	platform := &fakeAutomationPlatform{
		commit:       domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/repo", Branch: "main", SHA: "triage-retry-sha"},
		initialBuild: &domain.CIBuild{Provider: "JENKINS", ID: "queue-56", Number: 56, Status: "QUEUED"},
		build:        domain.CIBuild{Provider: "JENKINS", ID: "56", Number: 56, Status: "FAILURE", Logs: "src/main.go:5: failed"},
	}
	incidents := &fakeAutomationIncidents{triageErrors: []error{errors.New("AI service temporarily unavailable"), nil}}
	pipelines := &fakeAutomationPipelines{}
	runner := NewAutomationRunner(repository, platform, incidents, pipelines, func() time.Time { return now }, AutomationConfig{}, discardLogger())

	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if repository.processingState != "FAILED" || platform.triggerCalls != 1 || platform.pollCalls != 1 || len(pipelines.inputs) != 1 || len(incidents.created) != 1 || incidents.triageCalls != 1 {
		t.Fatalf("first attempt state=%s triggers=%d polls=%d builds=%d incidents=%d triages=%d", repository.processingState, platform.triggerCalls, platform.pollCalls, len(pipelines.inputs), len(incidents.created), incidents.triageCalls)
	}
	if repository.processingBuild == nil || repository.processingBuild.Status != "FAILURE" {
		t.Fatalf("terminal build was not saved for retry: %+v", repository.processingBuild)
	}

	if err := runner.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if repository.processingState != "PROCESSED" || platform.triggerCalls != 1 || platform.pollCalls != 1 || len(pipelines.inputs) != 1 || len(incidents.created) != 1 || incidents.triageCalls != 2 {
		t.Fatalf("retry state=%s triggers=%d polls=%d builds=%d incidents=%d triages=%d", repository.processingState, platform.triggerCalls, platform.pollCalls, len(pipelines.inputs), len(incidents.created), incidents.triageCalls)
	}
	if pipelines.inputs[0].ExternalBuildID == nil || !strings.HasPrefix(*pipelines.inputs[0].ExternalBuildID, "integration:11:commit:") {
		t.Fatalf("automated build identity is not integration scoped: %+v", pipelines.inputs[0].ExternalBuildID)
	}
}

func TestAutomationEventKeysAreIntegrationScoped(t *testing.T) {
	fingerprint := CommitFingerprint(domain.CommitMetadata{Provider: "GITHUB", Repository: "https://github.com/acme/repo", Branch: "main", SHA: "same-sha"})
	first := automationEventKey(1, fingerprint)
	second := automationEventKey(2, fingerprint)
	if first == second || len(first) > 128 || len(second) > 128 {
		t.Fatalf("event keys are not safely integration-scoped: %q %q", first, second)
	}
}

func TestExtractSourcePathsNormalizesCIWorkspaceAndDeduplicates(t *testing.T) {
	logs := "/github/workspace/apps/api/main.go:42 error\napps/api/main.go:43 error\n(src/config.ts:9)"
	paths := ExtractSourcePaths(logs, 5)
	if len(paths) != 2 || paths[0] != "apps/api/main.go" || paths[1] != "src/config.ts" {
		t.Fatalf("paths = %#v", paths)
	}
}

type fakeAutomationRepository struct {
	integration         domain.PlatformIntegration
	processingState     string
	processingBuild     *domain.CIBuild
	lastStatus          string
	finishSuccessErrors []error
	finishSuccessCalls  int
}

func newFakeAutomationRepository(integration domain.PlatformIntegration) *fakeAutomationRepository {
	return &fakeAutomationRepository{integration: integration}
}

func (f *fakeAutomationRepository) GetIntegration(context.Context, string) (*domain.PlatformIntegration, error) {
	copy := f.integration
	return &copy, nil
}
func (f *fakeAutomationRepository) SaveIntegration(_ context.Context, integration domain.PlatformIntegration, _ time.Time) (domain.PlatformIntegration, error) {
	f.integration = integration
	return integration, nil
}
func (f *fakeAutomationRepository) ClaimDueIntegrations(context.Context, time.Time, time.Duration, int) ([]domain.PlatformIntegration, error) {
	return []domain.PlatformIntegration{f.integration}, nil
}
func (f *fakeAutomationRepository) RecordCommitFingerprint(context.Context, int64, string, domain.CommitMetadata, time.Time) (bool, error) {
	f.processingState = "PROCESSED"
	return true, nil
}
func (f *fakeAutomationRepository) BeginCommitProcessing(context.Context, int64, string, domain.CommitMetadata, time.Time, time.Duration) (domain.CommitProcessing, error) {
	if f.processingState == "PROCESSED" {
		return domain.CommitProcessing{}, nil
	}
	f.processingState = "PROCESSING"
	return domain.CommitProcessing{ShouldProcess: true, Build: f.processingBuild}, nil
}
func (f *fakeAutomationRepository) SaveCommitBuild(_ context.Context, _ int64, _ string, build domain.CIBuild, _ time.Time) error {
	copy := build
	f.processingBuild = &copy
	return nil
}
func (f *fakeAutomationRepository) FinishCommitProcessing(_ context.Context, _ int64, _ string, success bool, _ string, _ time.Time) error {
	if success {
		index := f.finishSuccessCalls
		f.finishSuccessCalls++
		if index < len(f.finishSuccessErrors) && f.finishSuccessErrors[index] != nil {
			return f.finishSuccessErrors[index]
		}
		f.processingState = "PROCESSED"
	} else {
		f.processingState = "FAILED"
	}
	return nil
}
func (f *fakeAutomationRepository) CompleteIntegrationPoll(_ context.Context, _ string, status string, commit *string, completed, next time.Time) error {
	f.lastStatus = status
	f.integration.ConnectionStatus = &status
	f.integration.LastPolledCommit = commit
	f.integration.LastPollTime = &completed
	f.integration.NextPollTime = &next
	return nil
}

type fakeAutomationPlatform struct {
	commit        domain.CommitMetadata
	build         domain.CIBuild
	initialBuild  *domain.CIBuild
	triggerErrors []error
	triggerCalls  int
	pollCalls     int
}

func (f *fakeAutomationPlatform) LatestCommit(context.Context, domain.PlatformIntegration) (domain.CommitMetadata, error) {
	return f.commit, nil
}
func (f *fakeAutomationPlatform) TriggerBuild(context.Context, domain.PlatformIntegration, domain.CommitMetadata) (domain.CIBuild, error) {
	index := f.triggerCalls
	f.triggerCalls++
	if index < len(f.triggerErrors) && f.triggerErrors[index] != nil {
		return domain.CIBuild{}, f.triggerErrors[index]
	}
	if f.initialBuild != nil {
		return *f.initialBuild, nil
	}
	return f.build, nil
}
func (f *fakeAutomationPlatform) LatestBuild(context.Context, domain.PlatformIntegration, domain.CommitMetadata) (domain.CIBuild, error) {
	return f.build, nil
}
func (f *fakeAutomationPlatform) PollBuild(context.Context, domain.PlatformIntegration, domain.CIBuild) (domain.CIBuild, error) {
	f.pollCalls++
	return f.build, nil
}
func (f *fakeAutomationPlatform) FetchSource(_ context.Context, _ domain.PlatformIntegration, _ string, path string, _ int) (domain.SourceSnippet, error) {
	return domain.SourceSnippet{Path: path, Content: "package main", StartLine: 1, EndLine: 1}, nil
}

type fakeAutomationIncidents struct {
	created        []CreateIncidentInput
	triageEvidence TriageEvidence
	analysis       *domain.IncidentAnalysis
	triageErrors   []error
	triageCalls    int
}

func (f *fakeAutomationIncidents) CreateIncident(_ context.Context, input CreateIncidentInput) (domain.Incident, error) {
	if input.SourceEventKey != nil {
		for _, existing := range f.created {
			if existing.SourceEventKey != nil && *existing.SourceEventKey == *input.SourceEventKey {
				return domain.Incident{ID: "incident-1"}, nil
			}
		}
	}
	f.created = append(f.created, input)
	return domain.Incident{ID: "incident-1"}, nil
}
func (f *fakeAutomationIncidents) TriageWithEvidence(_ context.Context, _ string, evidence TriageEvidence) (TriageResult, error) {
	f.triageEvidence = evidence
	index := f.triageCalls
	f.triageCalls++
	if index < len(f.triageErrors) && f.triageErrors[index] != nil {
		return TriageResult{}, f.triageErrors[index]
	}
	f.analysis = &domain.IncidentAnalysis{ID: "analysis-1"}
	return TriageResult{}, nil
}
func (f *fakeAutomationIncidents) LatestAnalysis(context.Context, string) (*domain.IncidentAnalysis, error) {
	return f.analysis, nil
}

type fakeAutomationPipelines struct{ inputs []WebhookInput }

func (f *fakeAutomationPipelines) RecordBuild(_ context.Context, input WebhookInput) (domain.PipelineBuild, error) {
	if input.ExternalBuildID != nil {
		for _, existing := range f.inputs {
			if existing.ExternalBuildID != nil && *existing.ExternalBuildID == *input.ExternalBuildID {
				return domain.PipelineBuild{}, nil
			}
		}
	}
	f.inputs = append(f.inputs, input)
	return domain.PipelineBuild{}, nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
