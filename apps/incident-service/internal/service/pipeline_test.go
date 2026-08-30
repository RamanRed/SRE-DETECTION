package service

import (
	"context"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
)

func TestPipelinePersistsOnlySuppliedValuesAndCalculatesDORAMetrics(t *testing.T) {
	now := time.Date(2026, 8, 30, 14, 0, 0, 0, time.UTC)
	average := 125.9
	repository := &fakePipelineStore{
		raw:  domain.DORARawMetrics{TotalBuilds: 10, SuccessfulBuilds: 6, FailedBuilds: 1, AverageDuration: &average},
		page: domain.Page[domain.PipelineBuild]{Content: []domain.PipelineBuild{{ID: "recent"}}, TotalElements: 10, Page: 0, Size: 10},
	}
	svc := NewPipelineService(repository, func() time.Time { return now }, sequenceIDs("build-1"), "http://jenkins", "sre-copilot-pipeline")

	buildNumber := 8
	ciTool := "github_actions"
	build, err := svc.RecordBuild(context.Background(), WebhookInput{
		PipelineName: "deploy", BuildNumber: &buildNumber, CITool: &ciTool, Status: "success",
	})
	if err != nil {
		t.Fatal(err)
	}
	if build.BuildNumber != 8 || build.CITool != "GITHUB_ACTIONS" || build.Status != "SUCCESS" || build.DurationSeconds != nil || build.GitCommit != nil {
		t.Fatalf("unexpected stored build: %+v", build)
	}
	metrics, err := svc.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metrics.DeploymentFrequency != "1.0 deploys/day" || metrics.LeadTimeForChanges != "2m 5s" || metrics.ChangeFailureRate != 14.3 || metrics.MeanTimeToRecovery != "n/a" {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if !repository.after.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("DORA window starts at %v", repository.after)
	}
}

func TestPipelineRejectsFabricatedDefaults(t *testing.T) {
	svc := NewPipelineService(&fakePipelineStore{}, time.Now, sequenceIDs("1"), "", "")
	if _, err := svc.RecordBuild(context.Background(), WebhookInput{PipelineName: "deploy", Status: "SUCCESS"}); !IsCode(err, CodeInvalid) {
		t.Fatalf("RecordBuild() error = %v, want invalid", err)
	}
}

type fakePipelineStore struct {
	created domain.PipelineBuild
	page    domain.Page[domain.PipelineBuild]
	raw     domain.DORARawMetrics
	after   time.Time
}

func (f *fakePipelineStore) CreatePipelineBuild(_ context.Context, build domain.PipelineBuild) (domain.PipelineBuild, error) {
	f.created = build
	return build, nil
}
func (f *fakePipelineStore) ListPipelineBuilds(_ context.Context, _, _ int) (domain.Page[domain.PipelineBuild], error) {
	return f.page, nil
}
func (f *fakePipelineStore) DORAMetrics(_ context.Context, after time.Time) (domain.DORARawMetrics, error) {
	f.after = after
	return f.raw, nil
}
