package store

import (
	"context"
	"errors"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrIncidentMismatch = errors.New("remediation does not belong to incident")
)

type HealthStore interface {
	Ping(context.Context) error
}

type IncidentStore interface {
	CreateIncident(context.Context, domain.Incident) (domain.Incident, error)
	GetIncident(context.Context, string) (domain.Incident, error)
	ListIncidents(context.Context, int, int) (domain.Page[domain.Incident], error)
	ListActiveIncidents(context.Context) ([]domain.Incident, error)
	SetIncidentStatus(context.Context, string, domain.IncidentStatus, time.Time) error
	CompleteTriage(context.Context, domain.IncidentAnalysis, domain.IncidentSeverity, time.Time) error
	LatestAnalysis(context.Context, string) (*domain.IncidentAnalysis, error)
	SaveRemediation(context.Context, domain.Remediation) (domain.Remediation, error)
	ApproveRemediation(context.Context, string, string, string, time.Time) (domain.Remediation, error)
	DashboardStats(context.Context, time.Time, time.Time) (domain.DashboardStats, error)
}

type PipelineStore interface {
	CreatePipelineBuild(context.Context, domain.PipelineBuild) (domain.PipelineBuild, error)
	ListPipelineBuilds(context.Context, int, int) (domain.Page[domain.PipelineBuild], error)
	DORAMetrics(context.Context, time.Time) (domain.DORARawMetrics, error)
}

type IntegrationStore interface {
	GetIntegration(context.Context, string) (*domain.PlatformIntegration, error)
	SaveIntegration(context.Context, domain.PlatformIntegration, time.Time) (domain.PlatformIntegration, error)
}

type AutomationStore interface {
	ClaimDueIntegrations(context.Context, time.Time, time.Duration, int) ([]domain.PlatformIntegration, error)
	RecordCommitFingerprint(context.Context, int64, string, domain.CommitMetadata, time.Time) (bool, error)
	BeginCommitProcessing(context.Context, int64, string, domain.CommitMetadata, time.Time, time.Duration) (domain.CommitProcessing, error)
	SaveCommitBuild(context.Context, int64, string, domain.CIBuild, time.Time) error
	FinishCommitProcessing(context.Context, int64, string, bool, string, time.Time) error
	CompleteIntegrationPoll(context.Context, string, string, *string, time.Time, time.Time) error
}
