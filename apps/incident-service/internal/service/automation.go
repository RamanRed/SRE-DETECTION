package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
)

type AutomationPlatform interface {
	LatestCommit(context.Context, domain.PlatformIntegration) (domain.CommitMetadata, error)
	TriggerBuild(context.Context, domain.PlatformIntegration, domain.CommitMetadata) (domain.CIBuild, error)
	LatestBuild(context.Context, domain.PlatformIntegration, domain.CommitMetadata) (domain.CIBuild, error)
	PollBuild(context.Context, domain.PlatformIntegration, domain.CIBuild) (domain.CIBuild, error)
	FetchSource(context.Context, domain.PlatformIntegration, string, string, int) (domain.SourceSnippet, error)
}

type AutomationIncidentService interface {
	CreateIncident(context.Context, CreateIncidentInput) (domain.Incident, error)
	TriageWithEvidence(context.Context, string, TriageEvidence) (TriageResult, error)
	LatestAnalysis(context.Context, string) (*domain.IncidentAnalysis, error)
}

type AutomationPipelineService interface {
	RecordBuild(context.Context, WebhookInput) (domain.PipelineBuild, error)
}

type AutomationConfig struct {
	SweepInterval     time.Duration
	BuildPollInterval time.Duration
	BuildTimeout      time.Duration
	MaxSourceFiles    int
	MaxLogBytes       int
	MaxSourceBytes    int
	ClaimLimit        int
}

type AutomationRunner struct {
	store     AutomationRepository
	platform  AutomationPlatform
	incidents AutomationIncidentService
	pipelines AutomationPipelineService
	clock     Clock
	config    AutomationConfig
	logger    *slog.Logger
	processMu sync.Mutex
}

type AutomationRepository interface {
	store.AutomationStore
	store.IntegrationStore
}

type AutomationOutcome struct {
	CommitDiscovered bool
	BuildRecorded    bool
}

func NewAutomationRunner(repository AutomationRepository, platform AutomationPlatform, incidents AutomationIncidentService, pipelines AutomationPipelineService, clock Clock, config AutomationConfig, logger *slog.Logger) *AutomationRunner {
	if config.SweepInterval <= 0 {
		config.SweepInterval = 30 * time.Second
	}
	if config.BuildPollInterval <= 0 {
		config.BuildPollInterval = 10 * time.Second
	}
	if config.BuildTimeout <= 0 {
		config.BuildTimeout = 10 * time.Minute
	}
	if config.MaxSourceFiles <= 0 {
		config.MaxSourceFiles = 5
	}
	if config.MaxLogBytes <= 0 {
		config.MaxLogBytes = 64 << 10
	}
	if config.MaxSourceBytes <= 0 {
		config.MaxSourceBytes = 32 << 10
	}
	if config.ClaimLimit <= 0 {
		config.ClaimLimit = 1
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AutomationRunner{store: repository, platform: platform, incidents: incidents, pipelines: pipelines, clock: clock, config: config, logger: logger}
}

func (r *AutomationRunner) Run(ctx context.Context) {
	if err := r.Sweep(ctx); err != nil && ctx.Err() == nil {
		r.logger.Error("automation sweep failed", "error", err)
	}
	ticker := time.NewTicker(r.config.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Sweep(ctx); err != nil && ctx.Err() == nil {
				r.logger.Error("automation sweep failed", "error", err)
			}
		}
	}
}

func (r *AutomationRunner) Sweep(ctx context.Context) error {
	now := r.clock()
	lease := r.config.BuildTimeout + 2*r.config.BuildPollInterval
	integrations, err := r.store.ClaimDueIntegrations(ctx, now, lease, r.config.ClaimLimit)
	if err != nil {
		return fmt.Errorf("claim due integrations: %w", err)
	}
	for _, integration := range integrations {
		r.processMu.Lock()
		_, processErr := r.process(ctx, integration)
		r.processMu.Unlock()
		if processErr != nil {
			r.logger.Warn("integration poll failed", "user_id", integration.UserID, "provider", valueOr(integration.RepositoryProvider, ""), "error", processErr)
		}
	}
	return nil
}

func (r *AutomationRunner) SyncNow(ctx context.Context, userID string) (PlatformSyncResult, error) {
	integration, err := r.store.GetIntegration(ctx, userID)
	if err != nil {
		return PlatformSyncResult{}, fmt.Errorf("load integration: %w", err)
	}
	if integration == nil || strings.ToUpper(valueOr(integration.ConnectionStatus, "DISCONNECTED")) == "DISCONNECTED" {
		return PlatformSyncResult{}, errorWith(CodeInvalid, "No connected integration exists for this user", store.ErrNotFound)
	}
	r.processMu.Lock()
	outcome, err := r.process(ctx, *integration)
	r.processMu.Unlock()
	if err != nil {
		return PlatformSyncResult{
			Success: false, GitHubStatus: "ERROR", JenkinsStatus: "ERROR",
			Message: "Integration poll failed: " + err.Error(),
		}, nil
	}
	commits, builds := 0, 0
	if outcome.CommitDiscovered {
		commits = 1
	}
	if outcome.BuildRecorded {
		builds = 1
	}
	return PlatformSyncResult{
		Success: true, GitHubStatus: "CONNECTED", JenkinsStatus: "CONNECTED",
		CommitsSynced: commits, BuildsSynced: builds,
		Message: "Integration poll completed using configured repository and CI providers",
	}, nil
}

func (r *AutomationRunner) process(ctx context.Context, integration domain.PlatformIntegration) (AutomationOutcome, error) {
	var outcome AutomationOutcome
	var fingerprint string
	var processingStarted bool
	nextPoll := func(now time.Time) time.Time {
		return now.Add(CadenceDuration(valueOr(integration.PollingCadence, "15_MINUTES")))
	}
	fail := func(cause error) error {
		now := r.clock()
		cleanupContext, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelCleanup()
		if processingStarted && integration.ID != nil {
			_ = r.store.FinishCommitProcessing(cleanupContext, *integration.ID, fingerprint, false, cause.Error(), now)
		}
		_ = r.store.CompleteIntegrationPoll(cleanupContext, integration.UserID, "ERROR", nil, now, nextPoll(now))
		return cause
	}
	commit, err := r.platform.LatestCommit(ctx, integration)
	if err != nil {
		return outcome, fail(fmt.Errorf("read latest commit: %w", err))
	}
	fingerprint = CommitFingerprint(commit)
	if integration.ID == nil {
		return outcome, fail(fmt.Errorf("integration record has no ID"))
	}
	if integration.LastPolledCommit != nil && *integration.LastPolledCommit == commit.SHA {
		_, err := r.store.RecordCommitFingerprint(ctx, *integration.ID, fingerprint, commit, r.clock())
		if err != nil {
			return outcome, fail(fmt.Errorf("record baseline commit fingerprint: %w", err))
		}
		now := r.clock()
		return outcome, r.store.CompleteIntegrationPoll(ctx, integration.UserID, "CONNECTED", &commit.SHA, now, nextPoll(now))
	}
	processing, err := r.store.BeginCommitProcessing(ctx, *integration.ID, fingerprint, commit, r.clock(), r.config.BuildTimeout+2*r.config.BuildPollInterval)
	if err != nil {
		return outcome, fail(fmt.Errorf("begin commit processing: %w", err))
	}
	if !processing.ShouldProcess {
		now := r.clock()
		return outcome, r.store.CompleteIntegrationPoll(ctx, integration.UserID, "CONNECTED", &commit.SHA, now, nextPoll(now))
	}
	processingStarted = true
	outcome.CommitDiscovered = true

	var build domain.CIBuild
	if processing.Build != nil {
		build = *processing.Build
	} else {
		if integration.AutoRebuild {
			build, err = r.platform.TriggerBuild(ctx, integration, commit)
		} else {
			build, err = r.platform.LatestBuild(ctx, integration, commit)
		}
		if err == nil {
			err = r.store.SaveCommitBuild(ctx, *integration.ID, fingerprint, build, r.clock())
		}
	}
	if err != nil {
		return outcome, fail(fmt.Errorf("start or locate CI build: %w", err))
	}
	build, err = r.waitForBuild(ctx, integration, build)
	if err != nil {
		return outcome, fail(err)
	}
	if err := r.store.SaveCommitBuild(ctx, *integration.ID, fingerprint, build, r.clock()); err != nil {
		return outcome, fail(fmt.Errorf("persist terminal CI build: %w", err))
	}
	eventKey := automationEventKey(*integration.ID, fingerprint)
	if _, err := r.recordBuild(ctx, integration, commit, build, eventKey); err != nil {
		return outcome, fail(err)
	}
	outcome.BuildRecorded = true
	if build.Failed() {
		if err := r.ingestFailure(ctx, integration, commit, build, eventKey); err != nil {
			return outcome, fail(err)
		}
	}
	now := r.clock()
	if err := r.store.FinishCommitProcessing(ctx, *integration.ID, fingerprint, true, "", now); err != nil {
		return outcome, fail(fmt.Errorf("complete commit processing: %w", err))
	}
	processingStarted = false
	return outcome, r.store.CompleteIntegrationPoll(ctx, integration.UserID, "CONNECTED", &commit.SHA, now, nextPoll(now))
}

func (r *AutomationRunner) waitForBuild(ctx context.Context, integration domain.PlatformIntegration, build domain.CIBuild) (domain.CIBuild, error) {
	if build.Terminal() {
		return build, nil
	}
	buildContext, cancel := context.WithTimeout(ctx, r.config.BuildTimeout)
	defer cancel()
	ticker := time.NewTicker(r.config.BuildPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-buildContext.Done():
			if ctx.Err() != nil {
				return domain.CIBuild{}, ctx.Err()
			}
			build.Status = "TIMED_OUT"
			return build, nil
		case <-ticker.C:
			polled, err := r.platform.PollBuild(buildContext, integration, build)
			if err != nil {
				return domain.CIBuild{}, fmt.Errorf("poll CI build: %w", err)
			}
			build = polled
			if build.Terminal() {
				return build, nil
			}
		}
	}
}

func (r *AutomationRunner) recordBuild(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata, build domain.CIBuild, eventKey string) (domain.PipelineBuild, error) {
	name := integrationString(integration.JobName, integration.JenkinsJobName, "automated-ci")
	logs := truncateBytes(build.Logs, r.config.MaxLogBytes)
	duration := build.DurationSeconds
	externalBuildID := eventKey
	input := WebhookInput{
		PipelineName: name, BuildNumber: &build.Number, CITool: &build.Provider,
		Status: build.Status, GitCommit: &commit.SHA, GitBranch: &commit.Branch,
		CommitMessage: &commit.Message, Author: &commit.Author, DurationSeconds: &duration,
		Environment: stringPointer("production"), LogSnippet: &logs, BuildURL: &build.URL,
		ExternalBuildID: &externalBuildID,
	}
	result, err := r.pipelines.RecordBuild(ctx, input)
	if err != nil {
		return domain.PipelineBuild{}, fmt.Errorf("persist CI build: %w", err)
	}
	return result, nil
}

func automationEventKey(integrationID int64, fingerprint string) string {
	return fmt.Sprintf("integration:%d:commit:%s", integrationID, fingerprint)
}

func (r *AutomationRunner) ingestFailure(ctx context.Context, integration domain.PlatformIntegration, commit domain.CommitMetadata, build domain.CIBuild, eventKey string) error {
	logs := truncateBytes(build.Logs, r.config.MaxLogBytes)
	if strings.TrimSpace(logs) == "" {
		logs = fmt.Sprintf("%s build %s failed; logs were unavailable from the provider", build.Provider, build.ID)
	}
	snippets := make([]domain.SourceSnippet, 0, r.config.MaxSourceFiles)
	for _, sourcePath := range ExtractSourcePaths(logs, r.config.MaxSourceFiles) {
		snippet, err := r.platform.FetchSource(ctx, integration, commit.SHA, sourcePath, r.config.MaxSourceBytes)
		if err != nil {
			r.logger.Debug("source snippet unavailable", "user_id", integration.UserID, "path", sourcePath, "error", err)
			continue
		}
		snippets = append(snippets, snippet)
	}
	job := integrationString(integration.JobName, integration.JenkinsJobName, "CI pipeline")
	title := fmt.Sprintf("%s failed for commit %s", job, shortSHA(commit.SHA))
	firingRule := "CI_BUILD_FAILURE"
	createdBy := "integration-poller"
	incident, err := r.incidents.CreateIncident(ctx, CreateIncidentInput{
		Title: title, ServiceName: job, RawLogs: &logs, FiringRule: &firingRule,
		Environment: stringPointer("production"), CreatedBy: &createdBy, SourceEventKey: &eventKey,
	})
	if err != nil {
		return fmt.Errorf("create CI failure incident: %w", err)
	}
	if integration.AutoAITriage {
		existing, analysisErr := r.incidents.LatestAnalysis(ctx, incident.ID)
		if analysisErr != nil {
			return fmt.Errorf("check existing automated triage: %w", analysisErr)
		}
		if existing == nil {
			_, err = r.incidents.TriageWithEvidence(ctx, incident.ID, TriageEvidence{
				Commit: commit, SourceSnippets: snippets, CIProvider: build.Provider, BuildURL: build.URL,
			})
			if err != nil {
				return fmt.Errorf("automatic AI triage incident %s: %w", incident.ID, err)
			}
		}
	}
	return nil
}

func CommitFingerprint(commit domain.CommitMetadata) string {
	value := strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(commit.Provider)), strings.TrimSpace(commit.Repository),
		strings.TrimSpace(commit.Branch), strings.TrimSpace(commit.SHA),
	}, "\x00")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

var sourcePathPattern = regexp.MustCompile(`(?i)((?:[a-z]:)?(?:[./\\a-z0-9_-]+/|[./\\a-z0-9_-]*\\)*[a-z0-9_.-]+\.(?:go|java|kt|py|js|jsx|ts|tsx|rb|rs|c|cc|cpp|h|hpp|cs|php|scala|sh|yaml|yml|json|tf))(?:[:(]\d+)?`)

func ExtractSourcePaths(logs string, limit int) []string {
	if limit <= 0 {
		return []string{}
	}
	seen := make(map[string]struct{})
	paths := make([]string, 0, limit)
	for _, match := range sourcePathPattern.FindAllStringSubmatch(logs, -1) {
		candidate := filepath.ToSlash(strings.TrimSpace(match[1]))
		candidate = strings.TrimPrefix(candidate, "./")
		if strings.HasPrefix(candidate, "/") {
			candidate = repositoryRelativeAbsolutePath(candidate)
		}
		if candidate == "" || strings.HasPrefix(candidate, "../") || strings.HasPrefix(candidate, "/") || strings.Contains(candidate, "/../") {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		paths = append(paths, candidate)
		if len(paths) == limit {
			break
		}
	}
	return paths
}

func repositoryRelativeAbsolutePath(candidate string) string {
	markers := []string{"/github/workspace/", "/workspace/", "/build/", "/src/"}
	for _, marker := range markers {
		if index := strings.LastIndex(candidate, marker); index >= 0 {
			return strings.TrimPrefix(candidate[index+len(marker):], "/")
		}
	}
	return ""
}

func truncateBytes(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func shortSHA(value string) string {
	if len(value) > 8 {
		return value[:8]
	}
	return value
}

func integrationString(primary, legacy *string, fallback string) string {
	if primary != nil && strings.TrimSpace(*primary) != "" {
		return strings.TrimSpace(*primary)
	}
	if legacy != nil && strings.TrimSpace(*legacy) != "" {
		return strings.TrimSpace(*legacy)
	}
	return fallback
}
