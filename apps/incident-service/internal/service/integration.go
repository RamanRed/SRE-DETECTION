package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
)

type PlatformClient interface {
	TestGitHub(context.Context, domain.PlatformIntegration) error
	TestJenkins(context.Context, domain.PlatformIntegration) error
	SyncGitHub(context.Context, domain.PlatformIntegration) (int, error)
	SyncJenkins(context.Context, domain.PlatformIntegration) (int, error)
}

type IntegrationConnector interface {
	TestRepository(context.Context, domain.PlatformIntegration) error
	TestCI(context.Context, domain.PlatformIntegration) error
	LatestCommit(context.Context, domain.PlatformIntegration) (domain.CommitMetadata, error)
}

type IntegrationSynchronizer interface {
	SyncNow(context.Context, string) (PlatformSyncResult, error)
}

type IntegrationConfigInput struct {
	UserID             string
	Username           *string
	GitHubToken        *string
	GitHubRepo         *string
	GitHubBranch       *string
	JenkinsURL         *string
	JenkinsUsername    *string
	JenkinsAPIToken    *string
	JenkinsJobName     *string
	RepositoryProvider *string
	RepositoryURL      *string
	TargetBranch       *string
	RepositoryToken    *string
	PipelineEngine     *string
	CIBaseURL          *string
	CIUsername         *string
	CIToken            *string
	JobName            *string
	PollingCadence     *string
	AutoRebuild        bool
	AutoAITriage       bool
}

type ConnectResult struct {
	Integration domain.PlatformIntegration
	Message     string
}

type PlatformSyncResult struct {
	Success       bool
	GitHubStatus  string
	JenkinsStatus string
	CommitsSynced int
	BuildsSynced  int
	Message       string
}

type IntegrationDefaults struct {
	Username     string
	GitHubRepo   string
	GitHubBranch string
	JenkinsURL   string
	JenkinsJob   string
}

type IntegrationService struct {
	store        store.IntegrationStore
	platform     PlatformClient
	clock        Clock
	defaults     IntegrationDefaults
	connector    IntegrationConnector
	synchronizer IntegrationSynchronizer
}

type IntegrationOption func(*IntegrationService)

func WithIntegrationConnector(connector IntegrationConnector) IntegrationOption {
	return func(service *IntegrationService) { service.connector = connector }
}

func WithIntegrationSynchronizer(synchronizer IntegrationSynchronizer) IntegrationOption {
	return func(service *IntegrationService) { service.synchronizer = synchronizer }
}

func NewIntegrationService(repository store.IntegrationStore, platform PlatformClient, clock Clock, defaults IntegrationDefaults, options ...IntegrationOption) *IntegrationService {
	result := &IntegrationService{store: repository, platform: platform, clock: clock, defaults: defaults}
	for _, option := range options {
		option(result)
	}
	return result
}

func (s *IntegrationService) Config(ctx context.Context, userID string) (domain.PlatformIntegration, error) {
	integration, err := s.store.GetIntegration(ctx, userID)
	if err != nil {
		return domain.PlatformIntegration{}, fmt.Errorf("load integration config: %w", err)
	}
	if integration != nil {
		return *integration, nil
	}
	disconnected := "DISCONNECTED"
	provider := "GITHUB"
	engine := "JENKINS"
	cadence := "15_MINUTES"
	repositoryURL := "https://github.com/" + s.defaults.GitHubRepo
	return domain.PlatformIntegration{
		UserID:             userID,
		Username:           stringPointer(s.defaults.Username),
		GitHubRepo:         stringPointer(s.defaults.GitHubRepo),
		GitHubBranch:       stringPointer(s.defaults.GitHubBranch),
		GitHubStatus:       &disconnected,
		JenkinsURL:         stringPointer(s.defaults.JenkinsURL),
		JenkinsJobName:     stringPointer(s.defaults.JenkinsJob),
		JenkinsStatus:      &disconnected,
		RepositoryProvider: &provider,
		RepositoryURL:      &repositoryURL,
		TargetBranch:       stringPointer(s.defaults.GitHubBranch),
		PipelineEngine:     &engine,
		CIBaseURL:          stringPointer(s.defaults.JenkinsURL),
		JobName:            stringPointer(s.defaults.JenkinsJob),
		PollingCadence:     &cadence,
		ConnectionStatus:   &disconnected,
	}, nil
}

func (s *IntegrationService) SaveConfig(ctx context.Context, input IntegrationConfigInput) (domain.PlatformIntegration, error) {
	existing, err := s.store.GetIntegration(ctx, input.UserID)
	if err != nil {
		return domain.PlatformIntegration{}, fmt.Errorf("load integration config: %w", err)
	}
	integration := domain.PlatformIntegration{UserID: input.UserID}
	if existing != nil {
		integration = *existing
	}
	// The compatibility endpoint treats username as replaceable, including null.
	integration.Username = input.Username
	assignNonBlank(&integration.GitHubToken, input.GitHubToken)
	assignNonBlank(&integration.GitHubRepo, input.GitHubRepo)
	assignNonBlank(&integration.GitHubBranch, input.GitHubBranch)
	assignNonBlank(&integration.JenkinsURL, input.JenkinsURL)
	assignNonBlank(&integration.JenkinsUsername, input.JenkinsUsername)
	assignNonBlank(&integration.JenkinsAPIToken, input.JenkinsAPIToken)
	assignNonBlank(&integration.JenkinsJobName, input.JenkinsJobName)
	s.mergeAutonomousFields(&integration, input)

	if integration.GitHubRepo != nil && strings.TrimSpace(*integration.GitHubRepo) != "" {
		status := "CONNECTED"
		if err := s.platform.TestGitHub(ctx, integration); err != nil {
			status = "ERROR"
		}
		integration.GitHubStatus = &status
	}
	if integration.JenkinsURL != nil && strings.TrimSpace(*integration.JenkinsURL) != "" {
		status := "CONNECTED"
		if err := s.platform.TestJenkins(ctx, integration); err != nil {
			status = "ERROR"
		}
		integration.JenkinsStatus = &status
	}
	now := s.clock()
	integration.LastSyncTime = &now
	saved, err := s.store.SaveIntegration(ctx, integration, now)
	if err != nil {
		return domain.PlatformIntegration{}, fmt.Errorf("save integration config: %w", err)
	}
	return saved, nil
}

func (s *IntegrationService) Connect(ctx context.Context, input IntegrationConfigInput) (ConnectResult, error) {
	if s.connector == nil {
		return ConnectResult{}, errorWith(CodeUnavailable, "Integration connectors are not available", nil)
	}
	provider := strings.ToUpper(strings.TrimSpace(valueOr(input.RepositoryProvider, "GITHUB")))
	if provider != "GITHUB" && provider != "GITLAB" && provider != "BITBUCKET" {
		return ConnectResult{}, errorWith(CodeInvalid, "repositoryProvider must be GITHUB, GITLAB, or BITBUCKET", nil)
	}
	engine := strings.ToUpper(strings.TrimSpace(valueOr(input.PipelineEngine, "JENKINS")))
	if engine != "JENKINS" && engine != "GITHUB_ACTIONS" && engine != "KUBERNETES_JOB" {
		return ConnectResult{}, errorWith(CodeInvalid, "pipelineEngine must be JENKINS, GITHUB_ACTIONS, or KUBERNETES_JOB", nil)
	}
	if engine == "GITHUB_ACTIONS" && provider != "GITHUB" {
		return ConnectResult{}, errorWith(CodeInvalid, "GITHUB_ACTIONS requires repositoryProvider GITHUB", nil)
	}
	if engine == "KUBERNETES_JOB" && !input.AutoRebuild {
		return ConnectResult{}, errorWith(CodeInvalid, "KUBERNETES_JOB requires autoRebuild=true", nil)
	}
	if input.RepositoryURL == nil || strings.TrimSpace(*input.RepositoryURL) == "" {
		return ConnectResult{}, errorWith(CodeInvalid, "repositoryUrl is required", nil)
	}
	if input.CIBaseURL == nil || strings.TrimSpace(*input.CIBaseURL) == "" {
		return ConnectResult{}, errorWith(CodeInvalid, "ciBaseUrl is required", nil)
	}
	if len(strings.TrimSpace(*input.RepositoryURL)) > 1024 || len(strings.TrimSpace(*input.CIBaseURL)) > 1024 {
		return ConnectResult{}, errorWith(CodeInvalid, "integration URLs must not exceed 1024 bytes", nil)
	}
	cadence, err := NormalizeCadence(valueOr(input.PollingCadence, "15_MINUTES"))
	if err != nil {
		return ConnectResult{}, err
	}
	existing, err := s.store.GetIntegration(ctx, input.UserID)
	if err != nil {
		return ConnectResult{}, fmt.Errorf("load integration config: %w", err)
	}
	integration := domain.PlatformIntegration{UserID: input.UserID}
	if existing != nil {
		integration = *existing
		// Credentials are scoped to their provider/engine. Never silently reuse
		// a GitHub PAT for GitLab/Bitbucket or a Jenkins token for Kubernetes.
		if !strings.EqualFold(valueOr(existing.RepositoryProvider, "GITHUB"), provider) {
			integration.RepositoryToken = nil
			integration.GitHubToken = nil
		}
		if !strings.EqualFold(valueOr(existing.PipelineEngine, "JENKINS"), engine) {
			integration.CIToken = nil
			integration.JenkinsAPIToken = nil
			integration.CIUsername = nil
			integration.JenkinsUsername = nil
		}
	}
	integration.Username = input.Username
	input.RepositoryProvider = stringPointer(provider)
	input.PipelineEngine = stringPointer(engine)
	input.PollingCadence = stringPointer(cadence)
	s.mergeAutonomousFields(&integration, input)
	// Maintain legacy aliases for existing callers and storage columns.
	integration.GitHubToken = integration.RepositoryToken
	integration.GitHubBranch = integration.TargetBranch
	integration.JenkinsURL = integration.CIBaseURL
	integration.JenkinsUsername = integration.CIUsername
	integration.JenkinsAPIToken = integration.CIToken
	integration.JenkinsJobName = integration.JobName
	if integration.RepositoryToken == nil || strings.TrimSpace(*integration.RepositoryToken) == "" {
		return ConnectResult{}, errorWith(CodeInvalid, "repositoryToken is required for the selected repository provider", nil)
	}
	if engine == "JENKINS" && (integration.CIToken == nil || strings.TrimSpace(*integration.CIToken) == "") {
		return ConnectResult{}, errorWith(CodeInvalid, "ciToken is required for Jenkins", nil)
	}
	if provider == "GITHUB" {
		if repo := repositorySlug(*integration.RepositoryURL); repo != "" {
			integration.GitHubRepo = &repo
		}
	}

	repositoryErr := s.connector.TestRepository(ctx, integration)
	ciErr := s.connector.TestCI(ctx, integration)
	var baseline domain.CommitMetadata
	var baselineErr error
	if repositoryErr == nil && ciErr == nil {
		baseline, baselineErr = s.connector.LatestCommit(ctx, integration)
	}
	status := "CONNECTED"
	message := "Repository and CI connections validated successfully"
	if repositoryErr != nil || ciErr != nil || baselineErr != nil {
		status = "ERROR"
		message = "Connection validation failed"
	}
	integration.ConnectionStatus = &status
	integration.GitHubStatus = stringPointer(status)
	integration.JenkinsStatus = stringPointer(status)
	now := s.clock()
	integration.LastSyncTime = &now
	if status == "CONNECTED" {
		integration.LastPolledCommit = &baseline.SHA
		nextPoll := now.Add(CadenceDuration(cadence))
		integration.NextPollTime = &nextPoll
	} else {
		integration.NextPollTime = nil
	}
	saved, err := s.store.SaveIntegration(ctx, integration, now)
	if err != nil {
		return ConnectResult{}, fmt.Errorf("save connected integration: %w", err)
	}
	if repositoryErr != nil {
		message += ": repository validation failed"
	}
	if ciErr != nil {
		message += ": CI validation failed"
	}
	if baselineErr != nil {
		message += ": repository baseline failed"
	}
	return ConnectResult{Integration: saved, Message: message}, nil
}

func (s *IntegrationService) Sync(ctx context.Context, userID string) (PlatformSyncResult, error) {
	if s.synchronizer != nil {
		return s.synchronizer.SyncNow(ctx, userID)
	}
	integration, err := s.store.GetIntegration(ctx, userID)
	if err != nil {
		return PlatformSyncResult{}, fmt.Errorf("load integration config: %w", err)
	}
	if integration == nil {
		return PlatformSyncResult{}, errorWith(CodeInvalid, "No connected integration exists for this user", store.ErrNotFound)
	}

	commits, githubErr := s.platform.SyncGitHub(ctx, *integration)
	builds, jenkinsErr := s.platform.SyncJenkins(ctx, *integration)
	githubStatus := "CONNECTED"
	if githubErr != nil {
		githubStatus = "ERROR"
		commits = 0
	}
	jenkinsStatus := "CONNECTED"
	if jenkinsErr != nil {
		jenkinsStatus = "ERROR"
		builds = 0
	}
	now := s.clock()
	integration.GitHubStatus = &githubStatus
	integration.JenkinsStatus = &jenkinsStatus
	integration.LastSyncTime = &now
	if _, err := s.store.SaveIntegration(ctx, *integration, now); err != nil {
		return PlatformSyncResult{}, fmt.Errorf("save integration sync status: %w", err)
	}

	success := githubErr == nil && jenkinsErr == nil
	message := "Successfully synchronized telemetry from GitHub (" + valueOr(integration.GitHubRepo, "") + ") and Jenkins CI!"
	if !success {
		message = "Platform synchronization completed with one or more connection errors"
	}
	return PlatformSyncResult{
		Success:       success,
		GitHubStatus:  githubStatus,
		JenkinsStatus: jenkinsStatus,
		CommitsSynced: commits,
		BuildsSynced:  builds,
		Message:       message,
	}, nil
}

func assignNonBlank(destination **string, source *string) {
	if source != nil && strings.TrimSpace(*source) != "" {
		value := *source
		*destination = &value
	}
}

func (s *IntegrationService) mergeAutonomousFields(integration *domain.PlatformIntegration, input IntegrationConfigInput) {
	if input.RepositoryProvider == nil && input.GitHubRepo != nil {
		input.RepositoryProvider = stringPointer("GITHUB")
	}
	if input.RepositoryURL == nil && input.GitHubRepo != nil && strings.TrimSpace(*input.GitHubRepo) != "" {
		input.RepositoryURL = stringPointer("https://github.com/" + *input.GitHubRepo)
	}
	if input.TargetBranch == nil {
		input.TargetBranch = input.GitHubBranch
	}
	if input.RepositoryToken == nil {
		input.RepositoryToken = input.GitHubToken
	}
	if input.PipelineEngine == nil && input.JenkinsURL != nil {
		input.PipelineEngine = stringPointer("JENKINS")
	}
	if input.CIBaseURL == nil {
		input.CIBaseURL = input.JenkinsURL
	}
	if input.CIUsername == nil {
		input.CIUsername = input.JenkinsUsername
	}
	if input.CIToken == nil {
		input.CIToken = input.JenkinsAPIToken
	}
	if input.JobName == nil {
		input.JobName = input.JenkinsJobName
	}
	assignNonBlank(&integration.RepositoryProvider, input.RepositoryProvider)
	assignNonBlank(&integration.RepositoryURL, input.RepositoryURL)
	assignNonBlank(&integration.TargetBranch, input.TargetBranch)
	assignNonBlank(&integration.RepositoryToken, input.RepositoryToken)
	assignNonBlank(&integration.PipelineEngine, input.PipelineEngine)
	assignNonBlank(&integration.CIBaseURL, input.CIBaseURL)
	assignNonBlank(&integration.CIUsername, input.CIUsername)
	assignNonBlank(&integration.CIToken, input.CIToken)
	assignNonBlank(&integration.JobName, input.JobName)
	assignNonBlank(&integration.PollingCadence, input.PollingCadence)
	integration.AutoRebuild = input.AutoRebuild
	integration.AutoAITriage = input.AutoAITriage
}

func NormalizeCadence(value string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "5M", "5_MINUTES":
		return "5_MINUTES", nil
	case "15M", "15_MINUTES":
		return "15_MINUTES", nil
	case "1H", "1_HOUR":
		return "1_HOUR", nil
	case "DAILY", "24H", "DAILY_CRON":
		return "DAILY_CRON", nil
	default:
		return "", errorWith(CodeInvalid, "pollingCadence must be 5_MINUTES, 15_MINUTES, 1_HOUR, or DAILY_CRON", nil)
	}
}

func CadenceDuration(value string) time.Duration {
	switch value {
	case "5m", "5_MINUTES":
		return 5 * time.Minute
	case "1h", "1_HOUR":
		return time.Hour
	case "daily", "24h", "DAILY_CRON":
		return 24 * time.Hour
	default:
		return 15 * time.Minute
	}
}

func repositorySlug(repositoryURL string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repositoryURL), ".git")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}
