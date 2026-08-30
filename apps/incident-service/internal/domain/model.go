package domain

import "time"

type IncidentStatus string

const (
	IncidentOpen      IncidentStatus = "OPEN"
	IncidentAnalyzing IncidentStatus = "ANALYZING"
	IncidentResolved  IncidentStatus = "RESOLVED"
	IncidentClosed    IncidentStatus = "CLOSED"
)

type IncidentSeverity string

const (
	SeverityLow      IncidentSeverity = "LOW"
	SeverityMedium   IncidentSeverity = "MEDIUM"
	SeverityHigh     IncidentSeverity = "HIGH"
	SeverityCritical IncidentSeverity = "CRITICAL"
)

func NormalizeSeverity(value string) IncidentSeverity {
	switch IncidentSeverity(value) {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return IncidentSeverity(value)
	default:
		return SeverityHigh
	}
}

type Incident struct {
	ID             string
	Title          string
	ServiceName    string
	RawLogs        *string
	FiringRule     *string
	Environment    string
	Status         IncidentStatus
	Severity       *IncidentSeverity
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ResolvedAt     *time.Time
	SourceEventKey *string
}

type IncidentAnalysis struct {
	ID                  string
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
	RepositoryURL       string
	CommitSHA           string
	CommitMessage       string
	RepositoryProvider  string
	TargetBranch        string
	CIProvider          string
	BuildURL            string
	CreatedAt           time.Time
}

type SourceSnippet struct {
	Path      string
	Content   string
	StartLine int32
	EndLine   int32
}

type CommitMetadata struct {
	Provider    string
	Repository  string
	Branch      string
	SHA         string
	Message     string
	Author      string
	URL         string
	CommittedAt *time.Time
}

type CIBuild struct {
	Provider        string
	Reference       string
	ID              string
	Number          int
	Status          string
	URL             string
	Logs            string
	DurationSeconds int
	StartedAt       *time.Time
	CompletedAt     *time.Time
}

type CommitProcessing struct {
	ShouldProcess bool
	Build         *CIBuild
}

func (b CIBuild) Terminal() bool {
	switch b.Status {
	case "SUCCESS", "FAILURE", "UNSTABLE", "CANCELLED", "TIMED_OUT":
		return true
	default:
		return false
	}
}

func (b CIBuild) Failed() bool {
	return b.Status == "FAILURE" || b.Status == "UNSTABLE" || b.Status == "CANCELLED" || b.Status == "TIMED_OUT"
}

type ExecutionStatus string

const (
	ExecutionPending   ExecutionStatus = "PENDING"
	ExecutionApproved  ExecutionStatus = "APPROVED"
	ExecutionExecuting ExecutionStatus = "EXECUTING"
	ExecutionApplied   ExecutionStatus = "APPLIED"
	ExecutionFailed    ExecutionStatus = "FAILED"
	ExecutionRejected  ExecutionStatus = "REJECTED"
)

type Remediation struct {
	ID               string
	IncidentID       string
	AIRootCause      *string
	SuggestedAction  *string
	ScriptType       *string
	ExecutableScript *string
	ConfidenceScore  *string
	AffectedParts    *string
	RequiresApproval bool
	AppliedBy        *string
	ExecutionStatus  ExecutionStatus
	CreatedAt        time.Time
	AppliedAt        *time.Time
	UnifiedDiff      *string
	VerificationPlan *string
	RollbackPlan     *string
}

type DashboardStats struct {
	OpenIncidents       int64
	AnalyzingIncidents  int64
	ResolvedToday       int64
	PendingRemediations int64
	AppliedRemediations int64
}

type PipelineBuild struct {
	ID                      string
	PipelineName            string
	BuildNumber             int
	CITool                  string
	Status                  string
	GitCommit               *string
	GitBranch               *string
	CommitMessage           *string
	Author                  *string
	DurationSeconds         *int
	TestsPassed             *int
	TestsFailed             *int
	VulnerabilitiesDetected *int
	Environment             *string
	LogSnippet              *string
	BuildURL                *string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ExternalBuildID         *string
}

type DORARawMetrics struct {
	TotalBuilds      int64
	SuccessfulBuilds int64
	FailedBuilds     int64
	AverageDuration  *float64
}

type PlatformIntegration struct {
	ID                 *int64
	UserID             string
	Username           *string
	GitHubToken        *string
	GitHubRepo         *string
	GitHubBranch       *string
	GitHubStatus       *string
	JenkinsURL         *string
	JenkinsUsername    *string
	JenkinsAPIToken    *string
	JenkinsJobName     *string
	JenkinsStatus      *string
	LastSyncTime       *time.Time
	CreatedAt          *time.Time
	UpdatedAt          *time.Time
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
	ConnectionStatus   *string
	LastPolledCommit   *string
	LastPollTime       *time.Time
	NextPollTime       *time.Time
}

type Page[T any] struct {
	Content       []T
	TotalElements int64
	Page          int
	Size          int
}
