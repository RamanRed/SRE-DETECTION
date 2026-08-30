package httpapi

import (
	"math"
	"strings"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

type createIncidentRequest struct {
	Title       string  `json:"title"`
	ServiceName string  `json:"serviceName"`
	RawLogs     *string `json:"rawLogs"`
	FiringRule  *string `json:"firingRule"`
	Environment *string `json:"environment"`
	CreatedBy   *string `json:"createdBy"`
}

type incidentResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	ServiceName string  `json:"serviceName"`
	RawLogs     *string `json:"rawLogs"`
	FiringRule  *string `json:"firingRule"`
	Environment string  `json:"environment"`
	Status      string  `json:"status"`
	Severity    *string `json:"severity"`
	CreatedBy   string  `json:"createdBy"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	ResolvedAt  *string `json:"resolvedAt"`
}

type triageResponse struct {
	IncidentID          string   `json:"incidentId"`
	RootCause           string   `json:"rootCause"`
	ImmediateMitigation string   `json:"immediateMitigation"`
	ConfidenceScore     string   `json:"confidenceScore"`
	Severity            string   `json:"severity"`
	AffectedComponents  []string `json:"affectedComponents"`
	IncidentStatus      string   `json:"incidentStatus"`
	UnifiedDiff         string   `json:"unifiedDiff"`
	VerificationPlan    string   `json:"verificationPlan"`
	RollbackPlan        string   `json:"rollbackPlan"`
	CitedSourcePaths    []string `json:"citedSourcePaths"`
	RepositoryURL       string   `json:"repositoryUrl"`
	CommitSHA           string   `json:"commitSha"`
}

type remediationResponse struct {
	RemediationID          string `json:"remediationId"`
	IncidentID             string `json:"incidentId"`
	ScriptType             string `json:"scriptType"`
	ExecutableScript       string `json:"executableScript"`
	RequiresManualApproval bool   `json:"requiresManualApproval"`
	ExecutionStatus        string `json:"executionStatus"`
	UnifiedDiff            string `json:"unifiedDiff"`
	VerificationPlan       string `json:"verificationPlan"`
	RollbackPlan           string `json:"rollbackPlan"`
}

type approveRequest struct {
	AppliedBy string `json:"appliedBy"`
}

type dashboardStatsResponse struct {
	OpenIncidents       int64 `json:"openIncidents"`
	AnalyzingIncidents  int64 `json:"analyzingIncidents"`
	ResolvedToday       int64 `json:"resolvedToday"`
	PendingRemediations int64 `json:"pendingRemediations"`
	AppliedRemediations int64 `json:"appliedRemediations"`
}

type loginRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
}

type authResponse struct {
	Authenticated bool   `json:"authenticated"`
	Token         string `json:"token"`
	UserID        string `json:"userId"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	AvatarURL     string `json:"avatarUrl"`
	Message       string `json:"message"`
}

type webhookRequest struct {
	PipelineName            string  `json:"pipelineName"`
	BuildNumber             *int    `json:"buildNumber"`
	CITool                  *string `json:"ciTool"`
	Status                  string  `json:"status"`
	GitCommit               *string `json:"gitCommit"`
	GitBranch               *string `json:"gitBranch"`
	CommitMessage           *string `json:"commitMessage"`
	Author                  *string `json:"author"`
	DurationSeconds         *int    `json:"durationSeconds"`
	TestsPassed             *int    `json:"testsPassed"`
	TestsFailed             *int    `json:"testsFailed"`
	VulnerabilitiesDetected *int    `json:"vulnerabilitiesDetected"`
	Environment             *string `json:"environment"`
	LogSnippet              *string `json:"logSnippet"`
	BuildURL                *string `json:"buildUrl"`
	ExternalBuildID         *string `json:"externalBuildId"`
}

type pipelineBuildResponse struct {
	ID                      string  `json:"id"`
	PipelineName            string  `json:"pipelineName"`
	BuildNumber             int     `json:"buildNumber"`
	CITool                  string  `json:"ciTool"`
	Status                  string  `json:"status"`
	GitCommit               *string `json:"gitCommit"`
	GitBranch               *string `json:"gitBranch"`
	CommitMessage           *string `json:"commitMessage"`
	Author                  *string `json:"author"`
	DurationSeconds         *int    `json:"durationSeconds"`
	TestsPassed             *int    `json:"testsPassed"`
	TestsFailed             *int    `json:"testsFailed"`
	VulnerabilitiesDetected *int    `json:"vulnerabilitiesDetected"`
	Environment             *string `json:"environment"`
	LogSnippet              *string `json:"logSnippet"`
	BuildURL                *string `json:"buildUrl"`
	CreatedAt               string  `json:"createdAt"`
	UpdatedAt               string  `json:"updatedAt"`
	ExternalBuildID         *string `json:"externalBuildId"`
}

type doraMetricsResponse struct {
	DataAvailable       bool                    `json:"dataAvailable"`
	DeploymentFrequency string                  `json:"deploymentFrequency"`
	LeadTimeForChanges  string                  `json:"leadTimeForChanges"`
	ChangeFailureRate   float64                 `json:"changeFailureRate"`
	MeanTimeToRecovery  string                  `json:"meanTimeToRecovery"`
	TotalBuilds         int64                   `json:"totalBuilds"`
	SuccessfulBuilds    int64                   `json:"successfulBuilds"`
	FailedBuilds        int64                   `json:"failedBuilds"`
	RecentBuilds        []pipelineBuildResponse `json:"recentBuilds"`
}

type integrationConfigRequest struct {
	UserID             string  `json:"userId"`
	Username           *string `json:"username"`
	GitHubToken        *string `json:"githubToken"`
	GitHubRepo         *string `json:"githubRepo"`
	GitHubBranch       *string `json:"githubBranch"`
	JenkinsURL         *string `json:"jenkinsUrl"`
	JenkinsUsername    *string `json:"jenkinsUsername"`
	JenkinsAPIToken    *string `json:"jenkinsApiToken"`
	JenkinsJobName     *string `json:"jenkinsJobName"`
	RepositoryProvider *string `json:"repositoryProvider"`
	RepositoryURL      *string `json:"repositoryUrl"`
	TargetBranch       *string `json:"targetBranch"`
	RepositoryToken    *string `json:"repositoryToken"`
	PipelineEngine     *string `json:"pipelineEngine"`
	CIBaseURL          *string `json:"ciBaseUrl"`
	CIUsername         *string `json:"ciUsername"`
	CIToken            *string `json:"ciToken"`
	JobName            *string `json:"jobName"`
	PollingCadence     *string `json:"pollingCadence"`
	AutoRebuild        bool    `json:"autoRebuild"`
	AutoAITriage       bool    `json:"autoAITriage"`
}

type integrationConfigResponse struct {
	ID                        *int64  `json:"id"`
	UserID                    string  `json:"userId"`
	Username                  *string `json:"username"`
	GitHubRepo                *string `json:"githubRepo"`
	GitHubBranch              string  `json:"githubBranch"`
	GitHubStatus              string  `json:"githubStatus"`
	GitHubTokenConfigured     bool    `json:"githubTokenConfigured"`
	JenkinsURL                *string `json:"jenkinsUrl"`
	JenkinsUsername           *string `json:"jenkinsUsername"`
	JenkinsJobName            string  `json:"jenkinsJobName"`
	JenkinsStatus             string  `json:"jenkinsStatus"`
	JenkinsTokenConfigured    bool    `json:"jenkinsTokenConfigured"`
	LastSyncTime              *string `json:"lastSyncTime"`
	Message                   *string `json:"message"`
	RepositoryProvider        string  `json:"repositoryProvider"`
	RepositoryURL             *string `json:"repositoryUrl"`
	TargetBranch              string  `json:"targetBranch"`
	PipelineEngine            string  `json:"pipelineEngine"`
	CIBaseURL                 *string `json:"ciBaseUrl"`
	CIUsername                *string `json:"ciUsername"`
	JobName                   string  `json:"jobName"`
	PollingCadence            string  `json:"pollingCadence"`
	AutoRebuild               bool    `json:"autoRebuild"`
	AutoAITriage              bool    `json:"autoAITriage"`
	Status                    string  `json:"status"`
	RepositoryTokenConfigured bool    `json:"repositoryTokenConfigured"`
	CITokenConfigured         bool    `json:"ciTokenConfigured"`
	LastPolledCommit          *string `json:"lastPolledCommit"`
}

type platformSyncResponse struct {
	Success       bool   `json:"success"`
	GitHubStatus  string `json:"githubStatus"`
	JenkinsStatus string `json:"jenkinsStatus"`
	CommitsSynced int    `json:"commitsSynced"`
	BuildsSynced  int    `json:"buildsSynced"`
	Message       string `json:"message"`
}

type sortEnvelope struct {
	Empty    bool `json:"empty"`
	Sorted   bool `json:"sorted"`
	Unsorted bool `json:"unsorted"`
}

type pageableEnvelope struct {
	PageNumber int          `json:"pageNumber"`
	PageSize   int          `json:"pageSize"`
	Sort       sortEnvelope `json:"sort"`
	Offset     int64        `json:"offset"`
	Paged      bool         `json:"paged"`
	Unpaged    bool         `json:"unpaged"`
}

type pageEnvelope[T any] struct {
	Content          []T              `json:"content"`
	Pageable         pageableEnvelope `json:"pageable"`
	Last             bool             `json:"last"`
	TotalPages       int              `json:"totalPages"`
	TotalElements    int64            `json:"totalElements"`
	Size             int              `json:"size"`
	Number           int              `json:"number"`
	Sort             sortEnvelope     `json:"sort"`
	First            bool             `json:"first"`
	NumberOfElements int              `json:"numberOfElements"`
	Empty            bool             `json:"empty"`
}

func incidentDTO(incident domain.Incident) incidentResponse {
	var severity *string
	if incident.Severity != nil {
		value := string(*incident.Severity)
		severity = &value
	}
	return incidentResponse{
		ID:          incident.ID,
		Title:       incident.Title,
		ServiceName: incident.ServiceName,
		RawLogs:     incident.RawLogs,
		FiringRule:  incident.FiringRule,
		Environment: incident.Environment,
		Status:      string(incident.Status),
		Severity:    severity,
		CreatedBy:   incident.CreatedBy,
		CreatedAt:   localTime(incident.CreatedAt),
		UpdatedAt:   localTime(incident.UpdatedAt),
		ResolvedAt:  optionalLocalTime(incident.ResolvedAt),
	}
}

func pipelineDTO(build domain.PipelineBuild) pipelineBuildResponse {
	return pipelineBuildResponse{
		ID: build.ID, PipelineName: build.PipelineName, BuildNumber: build.BuildNumber,
		CITool: build.CITool, Status: build.Status, GitCommit: build.GitCommit,
		GitBranch: build.GitBranch, CommitMessage: build.CommitMessage, Author: build.Author,
		DurationSeconds: build.DurationSeconds, TestsPassed: build.TestsPassed, TestsFailed: build.TestsFailed,
		VulnerabilitiesDetected: build.VulnerabilitiesDetected, Environment: build.Environment,
		LogSnippet: build.LogSnippet, BuildURL: build.BuildURL, CreatedAt: localTime(build.CreatedAt),
		UpdatedAt:       localTime(build.UpdatedAt),
		ExternalBuildID: build.ExternalBuildID,
	}
}

func doraDTO(metrics service.DORAMetrics) doraMetricsResponse {
	recent := make([]pipelineBuildResponse, 0, len(metrics.RecentBuilds))
	for _, build := range metrics.RecentBuilds {
		recent = append(recent, pipelineDTO(build))
	}
	return doraMetricsResponse{
		DataAvailable:       metrics.DataAvailable,
		DeploymentFrequency: metrics.DeploymentFrequency, LeadTimeForChanges: metrics.LeadTimeForChanges,
		ChangeFailureRate: metrics.ChangeFailureRate, MeanTimeToRecovery: metrics.MeanTimeToRecovery,
		TotalBuilds: metrics.TotalBuilds, SuccessfulBuilds: metrics.SuccessfulBuilds,
		FailedBuilds: metrics.FailedBuilds, RecentBuilds: recent,
	}
}

func integrationDTO(integration domain.PlatformIntegration) integrationConfigResponse {
	branch := "main"
	if integration.GitHubBranch != nil {
		branch = *integration.GitHubBranch
	}
	githubStatus := "DISCONNECTED"
	if integration.GitHubStatus != nil {
		githubStatus = *integration.GitHubStatus
	}
	job := "sre-copilot-pipeline"
	if integration.JenkinsJobName != nil {
		job = *integration.JenkinsJobName
	}
	jenkinsStatus := "DISCONNECTED"
	if integration.JenkinsStatus != nil {
		jenkinsStatus = *integration.JenkinsStatus
	}
	provider := valueOrString(integration.RepositoryProvider, "GITHUB")
	repositoryURL := integration.RepositoryURL
	if repositoryURL == nil && integration.GitHubRepo != nil {
		value := "https://github.com/" + *integration.GitHubRepo
		repositoryURL = &value
	}
	targetBranch := valueOrString(integration.TargetBranch, branch)
	engine := valueOrString(integration.PipelineEngine, "JENKINS")
	ciBaseURL := integration.CIBaseURL
	if ciBaseURL == nil {
		ciBaseURL = integration.JenkinsURL
	}
	ciUsername := integration.CIUsername
	if ciUsername == nil {
		ciUsername = integration.JenkinsUsername
	}
	newJob := valueOrString(integration.JobName, job)
	cadence := valueOrString(integration.PollingCadence, "15_MINUTES")
	if normalized, err := service.NormalizeCadence(cadence); err == nil {
		cadence = normalized
	}
	status := valueOrString(integration.ConnectionStatus, "DISCONNECTED")
	repositoryToken := integration.RepositoryToken
	if repositoryToken == nil {
		repositoryToken = integration.GitHubToken
	}
	ciToken := integration.CIToken
	if ciToken == nil {
		ciToken = integration.JenkinsAPIToken
	}
	return integrationConfigResponse{
		ID: integration.ID, UserID: integration.UserID, Username: integration.Username,
		GitHubRepo: integration.GitHubRepo, GitHubBranch: branch, GitHubStatus: githubStatus,
		GitHubTokenConfigured: integration.GitHubToken != nil && strings.TrimSpace(*integration.GitHubToken) != "",
		JenkinsURL:            integration.JenkinsURL, JenkinsUsername: integration.JenkinsUsername,
		JenkinsJobName: job, JenkinsStatus: jenkinsStatus,
		JenkinsTokenConfigured: integration.JenkinsAPIToken != nil && strings.TrimSpace(*integration.JenkinsAPIToken) != "",
		LastSyncTime:           optionalLocalTime(integration.LastSyncTime), Message: nil,
		RepositoryProvider: provider, RepositoryURL: repositoryURL, TargetBranch: targetBranch,
		PipelineEngine: engine, CIBaseURL: ciBaseURL, CIUsername: ciUsername, JobName: newJob,
		PollingCadence: cadence, AutoRebuild: integration.AutoRebuild, AutoAITriage: integration.AutoAITriage,
		Status: status, RepositoryTokenConfigured: repositoryToken != nil && strings.TrimSpace(*repositoryToken) != "",
		CITokenConfigured: ciToken != nil && strings.TrimSpace(*ciToken) != "",
		LastPolledCommit:  integration.LastPolledCommit,
	}
}

func valueOrString(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return *value
}

func pageDTO[T any](content []T, total int64, page, size int) pageEnvelope[T] {
	if content == nil {
		content = []T{}
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(size)))
	}
	sort := sortEnvelope{Empty: true, Sorted: false, Unsorted: true}
	return pageEnvelope[T]{
		Content:  content,
		Pageable: pageableEnvelope{PageNumber: page, PageSize: size, Sort: sort, Offset: int64(page * size), Paged: true, Unpaged: false},
		Last:     totalPages == 0 || page >= totalPages-1, TotalPages: totalPages, TotalElements: total,
		Size: size, Number: page, Sort: sort, First: page == 0,
		NumberOfElements: len(content), Empty: len(content) == 0,
	}
}

func localTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05.999999999")
}

func optionalLocalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := localTime(*value)
	return &formatted
}
