package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

func TestSaveIntegrationReportsRealConnectionErrorsAndPreservesSecret(t *testing.T) {
	oldToken := "existing-secret"
	repo := "RamanRed/SRE-DETECTION"
	jenkins := "https://jenkins.example.com"
	repository := &fakeIntegrationStore{integration: &domain.PlatformIntegration{
		UserID: "ramanred", GitHubToken: &oldToken, GitHubRepo: &repo, JenkinsURL: &jenkins,
	}}
	platform := &fakePlatform{githubTestErr: errors.New("unauthorized")}
	now := time.Date(2026, 8, 30, 13, 0, 0, 0, time.UTC)
	svc := NewIntegrationService(repository, platform, func() time.Time { return now }, IntegrationDefaults{})
	blankToken := " "
	username := "RamanRed"

	result, err := svc.SaveConfig(context.Background(), IntegrationConfigInput{
		UserID: "ramanred", Username: &username, GitHubToken: &blankToken,
	})
	if err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}
	if result.GitHubStatus == nil || *result.GitHubStatus != "ERROR" {
		t.Fatalf("GitHub status = %v, want ERROR", result.GitHubStatus)
	}
	if result.GitHubToken == nil || *result.GitHubToken != oldToken {
		t.Fatalf("blank update replaced stored token: %v", result.GitHubToken)
	}
	if result.JenkinsStatus == nil || *result.JenkinsStatus != "CONNECTED" {
		t.Fatalf("Jenkins status = %v, want CONNECTED", result.JenkinsStatus)
	}
	if result.LastSyncTime == nil || !result.LastSyncTime.Equal(now) {
		t.Fatalf("last sync = %v, want %v", result.LastSyncTime, now)
	}
}

func TestSyncIntegrationDoesNotClaimSuccessOnPlatformFailure(t *testing.T) {
	repo := "RamanRed/SRE-DETECTION"
	jenkins := "https://jenkins.example.com"
	repository := &fakeIntegrationStore{integration: &domain.PlatformIntegration{UserID: "ramanred", GitHubRepo: &repo, JenkinsURL: &jenkins}}
	platform := &fakePlatform{commits: 5, builds: 4, jenkinsSyncErr: errors.New("offline")}
	svc := NewIntegrationService(repository, platform, time.Now, IntegrationDefaults{})

	result, err := svc.Sync(context.Background(), "ramanred")
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if result.Success || result.GitHubStatus != "CONNECTED" || result.JenkinsStatus != "ERROR" || result.CommitsSynced != 5 || result.BuildsSynced != 0 {
		t.Fatalf("unexpected sync result: %+v", result)
	}
	if repository.saved == nil || repository.saved.JenkinsStatus == nil || *repository.saved.JenkinsStatus != "ERROR" {
		t.Fatalf("sync status was not persisted: %+v", repository.saved)
	}
}

func TestMissingIntegrationIsDisconnectedAndDoesNotFabricateSync(t *testing.T) {
	repository := &fakeIntegrationStore{}
	platform := &fakePlatform{}
	svc := NewIntegrationService(repository, platform, time.Now, IntegrationDefaults{
		Username: "RamanRed", GitHubRepo: "RamanRed/SRE-DETECTION", GitHubBranch: "main",
		JenkinsURL: "http://localhost:8080", JenkinsJob: "sre-copilot-pipeline",
	})

	config, err := svc.Config(context.Background(), "ramanred")
	if err != nil {
		t.Fatal(err)
	}
	if config.ID != nil || config.GitHubStatus == nil || *config.GitHubStatus != "DISCONNECTED" || config.JenkinsStatus == nil || *config.JenkinsStatus != "DISCONNECTED" {
		t.Fatalf("unexpected default config: %+v", config)
	}
	if _, err := svc.Sync(context.Background(), "ramanred"); !IsCode(err, CodeInvalid) {
		t.Fatalf("Sync() error = %v, want invalid for missing integration", err)
	}
}

func TestConnectValidatesProvidersAndBaselinesCurrentCommit(t *testing.T) {
	now := time.Date(2026, 8, 30, 13, 0, 0, 0, time.UTC)
	repository := &fakeIntegrationStore{}
	connector := &fakeConnector{commit: domain.CommitMetadata{SHA: "baseline-sha", Branch: "main"}}
	svc := NewIntegrationService(repository, &fakePlatform{}, func() time.Time { return now }, IntegrationDefaults{}, WithIntegrationConnector(connector))
	provider, repositoryURL, branch, repositoryToken := "github", "https://github.com/acme/widgets", "main", "repo-secret"
	engine, ciBaseURL, ciToken, job, cadence := "jenkins", "https://ci.example.com", "ci-secret", "deploy", "5m"
	result, err := svc.Connect(context.Background(), IntegrationConfigInput{
		UserID: "operator", RepositoryProvider: &provider, RepositoryURL: &repositoryURL,
		TargetBranch: &branch, RepositoryToken: &repositoryToken, PipelineEngine: &engine,
		CIBaseURL: &ciBaseURL, CIToken: &ciToken, JobName: &job, PollingCadence: &cadence,
		AutoRebuild: true, AutoAITriage: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Integration.ConnectionStatus == nil || *result.Integration.ConnectionStatus != "CONNECTED" || result.Integration.LastPolledCommit == nil || *result.Integration.LastPolledCommit != "baseline-sha" {
		t.Fatalf("connect result = %+v", result.Integration)
	}
	if result.Integration.PollingCadence == nil || *result.Integration.PollingCadence != "5_MINUTES" || result.Integration.NextPollTime == nil || !result.Integration.NextPollTime.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("cadence/baseline schedule = %+v", result.Integration)
	}
	if connector.repositoryTests != 1 || connector.ciTests != 1 || connector.commitReads != 1 {
		t.Fatalf("connector calls repo=%d ci=%d commit=%d", connector.repositoryTests, connector.ciTests, connector.commitReads)
	}
}

func TestConnectDoesNotReuseCredentialsAcrossProvidersOrEngines(t *testing.T) {
	oldProvider, oldEngine := "GITHUB", "JENKINS"
	oldRepositoryToken, oldCIToken := "github-secret", "jenkins-secret"
	repositoryURL, branch := "https://gitlab.com/acme/widgets", "main"
	ciBaseURL, job, cadence := "https://ci.example.com", "deploy", "15_MINUTES"
	repository := &fakeIntegrationStore{integration: &domain.PlatformIntegration{
		UserID: "operator", RepositoryProvider: &oldProvider, RepositoryToken: &oldRepositoryToken,
		GitHubToken: &oldRepositoryToken, PipelineEngine: &oldEngine, CIToken: &oldCIToken,
		JenkinsAPIToken: &oldCIToken,
	}}
	connector := &fakeConnector{commit: domain.CommitMetadata{SHA: "baseline-sha", Branch: branch}}
	svc := NewIntegrationService(repository, &fakePlatform{}, time.Now, IntegrationDefaults{}, WithIntegrationConnector(connector))

	newProvider := "GITLAB"
	_, err := svc.Connect(context.Background(), IntegrationConfigInput{
		UserID: "operator", RepositoryProvider: &newProvider, RepositoryURL: &repositoryURL,
		TargetBranch: &branch, PipelineEngine: &oldEngine, CIBaseURL: &ciBaseURL,
		JobName: &job, PollingCadence: &cadence, AutoRebuild: true,
	})
	if !IsCode(err, CodeInvalid) || connector.repositoryTests != 0 || connector.ciTests != 0 {
		t.Fatalf("provider switch error=%v connector calls=%d/%d", err, connector.repositoryTests, connector.ciTests)
	}

	newProvider = "GITHUB"
	newEngine := "KUBERNETES_JOB"
	repository.integration.RepositoryProvider = &newProvider
	repository.integration.PipelineEngine = &newEngine
	repository.integration.CIToken = &oldCIToken
	repository.integration.JenkinsAPIToken = &oldCIToken
	_, err = svc.Connect(context.Background(), IntegrationConfigInput{
		UserID: "operator", RepositoryProvider: &newProvider, RepositoryURL: &repositoryURL,
		TargetBranch: &branch, PipelineEngine: &oldEngine, CIBaseURL: &ciBaseURL,
		JobName: &job, PollingCadence: &cadence, AutoRebuild: true,
	})
	if !IsCode(err, CodeInvalid) || connector.repositoryTests != 0 || connector.ciTests != 0 {
		t.Fatalf("engine switch error=%v connector calls=%d/%d", err, connector.repositoryTests, connector.ciTests)
	}
}

func TestNormalizeCadenceAcceptsLegacyAndDocumentedForms(t *testing.T) {
	cases := map[string]string{
		"5m": "5_MINUTES", "5_MINUTES": "5_MINUTES", "15m": "15_MINUTES",
		"15_MINUTES": "15_MINUTES", "1h": "1_HOUR", "1_HOUR": "1_HOUR",
		"daily": "DAILY_CRON", "DAILY_CRON": "DAILY_CRON",
	}
	for input, want := range cases {
		got, err := NormalizeCadence(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeCadence(%q)=%q,%v want %q", input, got, err, want)
		}
	}
}

type fakeIntegrationStore struct {
	integration *domain.PlatformIntegration
	saved       *domain.PlatformIntegration
}

func (f *fakeIntegrationStore) GetIntegration(context.Context, string) (*domain.PlatformIntegration, error) {
	return f.integration, nil
}
func (f *fakeIntegrationStore) SaveIntegration(_ context.Context, integration domain.PlatformIntegration, _ time.Time) (domain.PlatformIntegration, error) {
	f.saved = &integration
	f.integration = &integration
	return integration, nil
}

type fakePlatform struct {
	githubTestErr  error
	jenkinsTestErr error
	githubSyncErr  error
	jenkinsSyncErr error
	commits        int
	builds         int
}

type fakeConnector struct {
	commit          domain.CommitMetadata
	repositoryErr   error
	ciErr           error
	commitErr       error
	repositoryTests int
	ciTests         int
	commitReads     int
}

func (f *fakeConnector) TestRepository(context.Context, domain.PlatformIntegration) error {
	f.repositoryTests++
	return f.repositoryErr
}
func (f *fakeConnector) TestCI(context.Context, domain.PlatformIntegration) error {
	f.ciTests++
	return f.ciErr
}
func (f *fakeConnector) LatestCommit(context.Context, domain.PlatformIntegration) (domain.CommitMetadata, error) {
	f.commitReads++
	return f.commit, f.commitErr
}

func (f *fakePlatform) TestGitHub(context.Context, domain.PlatformIntegration) error {
	return f.githubTestErr
}
func (f *fakePlatform) TestJenkins(context.Context, domain.PlatformIntegration) error {
	return f.jenkinsTestErr
}
func (f *fakePlatform) SyncGitHub(context.Context, domain.PlatformIntegration) (int, error) {
	return f.commits, f.githubSyncErr
}
func (f *fakePlatform) SyncJenkins(context.Context, domain.PlatformIntegration) (int, error) {
	return f.builds, f.jenkinsSyncErr
}
