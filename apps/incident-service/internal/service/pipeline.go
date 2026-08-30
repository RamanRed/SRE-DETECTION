package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/store"
)

type WebhookInput struct {
	PipelineName            string
	BuildNumber             *int
	CITool                  *string
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
	ExternalBuildID         *string
}

type DORAMetrics struct {
	DataAvailable       bool
	DeploymentFrequency string
	LeadTimeForChanges  string
	ChangeFailureRate   float64
	MeanTimeToRecovery  string
	TotalBuilds         int64
	SuccessfulBuilds    int64
	FailedBuilds        int64
	RecentBuilds        []domain.PipelineBuild
}

type PipelineService struct {
	store store.PipelineStore
	clock Clock
	ids   IDGenerator
}

func NewPipelineService(repository store.PipelineStore, clock Clock, ids IDGenerator, _, _ string) *PipelineService {
	return &PipelineService{store: repository, clock: clock, ids: ids}
}

func (s *PipelineService) RecordBuild(ctx context.Context, input WebhookInput) (domain.PipelineBuild, error) {
	now := s.clock()
	if strings.TrimSpace(input.PipelineName) == "" {
		return domain.PipelineBuild{}, errorWith(CodeInvalid, "pipelineName is required", nil)
	}
	if input.CITool == nil || strings.TrimSpace(*input.CITool) == "" {
		return domain.PipelineBuild{}, errorWith(CodeInvalid, "ciTool is required", nil)
	}
	ciTool := strings.ToUpper(strings.TrimSpace(*input.CITool))
	if input.BuildNumber == nil || *input.BuildNumber < 0 || (*input.BuildNumber == 0 && ciTool != "KUBERNETES_JOB") {
		return domain.PipelineBuild{}, errorWith(CodeInvalid, "buildNumber must be greater than zero (or zero for Kubernetes Jobs)", nil)
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	switch status {
	case "QUEUED", "RUNNING", "SUCCESS", "FAILURE", "UNSTABLE", "CANCELLED", "TIMED_OUT":
	default:
		return domain.PipelineBuild{}, errorWith(CodeInvalid, "status is not a supported CI build status", nil)
	}

	build := domain.PipelineBuild{
		ID:                      s.ids(),
		PipelineName:            input.PipelineName,
		BuildNumber:             *input.BuildNumber,
		CITool:                  ciTool,
		Status:                  status,
		GitCommit:               input.GitCommit,
		GitBranch:               input.GitBranch,
		CommitMessage:           input.CommitMessage,
		Author:                  input.Author,
		DurationSeconds:         input.DurationSeconds,
		TestsPassed:             input.TestsPassed,
		TestsFailed:             input.TestsFailed,
		VulnerabilitiesDetected: input.VulnerabilitiesDetected,
		Environment:             input.Environment,
		LogSnippet:              input.LogSnippet,
		BuildURL:                input.BuildURL,
		CreatedAt:               now,
		UpdatedAt:               now,
		ExternalBuildID:         input.ExternalBuildID,
	}
	created, err := s.store.CreatePipelineBuild(ctx, build)
	if err != nil {
		return domain.PipelineBuild{}, fmt.Errorf("record pipeline build: %w", err)
	}
	return created, nil
}

func (s *PipelineService) Builds(ctx context.Context, page, size int) (domain.Page[domain.PipelineBuild], error) {
	if err := validatePage(page, size); err != nil {
		return domain.Page[domain.PipelineBuild]{}, err
	}
	return s.store.ListPipelineBuilds(ctx, page, size)
}

func (s *PipelineService) Metrics(ctx context.Context) (DORAMetrics, error) {
	raw, err := s.store.DORAMetrics(ctx, s.clock().Add(-7*24*time.Hour))
	if err != nil {
		return DORAMetrics{}, fmt.Errorf("calculate DORA metrics: %w", err)
	}
	recent, err := s.store.ListPipelineBuilds(ctx, 0, 10)
	if err != nil {
		return DORAMetrics{}, fmt.Errorf("load recent builds: %w", err)
	}

	totalRecent := raw.SuccessfulBuilds + raw.FailedBuilds
	frequency := 0.0
	if totalRecent > 0 {
		frequency = float64(totalRecent) / 7
	}
	failureRate := 0.0
	if totalRecent > 0 {
		failureRate = float64(raw.FailedBuilds) / float64(totalRecent) * 100
	}
	leadTime := "n/a"
	if raw.AverageDuration != nil && *raw.AverageDuration > 0 {
		seconds := int(*raw.AverageDuration)
		leadTime = fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	return DORAMetrics{
		DataAvailable:       totalRecent > 0,
		DeploymentFrequency: fmt.Sprintf("%.1f deploys/day", frequency),
		LeadTimeForChanges:  leadTime,
		ChangeFailureRate:   math.Round(failureRate*10) / 10,
		MeanTimeToRecovery:  "n/a",
		TotalBuilds:         raw.TotalBuilds,
		SuccessfulBuilds:    raw.SuccessfulBuilds,
		FailedBuilds:        raw.FailedBuilds,
		RecentBuilds:        recent.Content,
	}, nil
}
