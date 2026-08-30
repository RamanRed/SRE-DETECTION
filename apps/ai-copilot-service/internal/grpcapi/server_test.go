package grpcapi

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/metrics"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/remediation"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/triage"
	"github.com/RamanRed/SRE-DETECTION/gen/copilotpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type grpcProviderFunc func(context.Context, string) (string, error)

func (function grpcProviderFunc) Complete(ctx context.Context, prompt string) (string, error) {
	return function(ctx, prompt)
}

func TestGRPCWireMapping(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	latency := metrics.NewInferenceHistogram("ai-copilot-service")
	service := NewServer(
		triage.NewAnalyzer(nil, logger),
		remediation.NewGenerator("sre-copilot"),
		latency,
		logger,
	)

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	copilotpb.RegisterIncidentCopilotServiceServer(grpcServer, service)
	go func() {
		_ = grpcServer.Serve(listener)
	}()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = listener.Close()
	})

	ctx := context.Background()
	connection, err := grpc.DialContext(
		ctx,
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer connection.Close()
	client := copilotpb.NewIncidentCopilotServiceClient(connection)

	analysis, err := client.AnalyzeIncident(ctx, &copilotpb.IncidentAnalysisRequest{
		IncidentId: "inc-1", ServiceName: "incident-service",
		ErrorLogs:  "pgxpool: failed to connect to PostgreSQL: connection reset by peer",
		FiringRule: "DatabaseDown", Environment: "production",
		Provider: "github", Repository: "https://github.example/acme/sre.git",
		Branch: "main", CommitSha: "abc123", CommitMessage: "change database pool settings",
		CiProvider: "github-actions", BuildUrl: "https://ci.example/build/42",
		SourceSnippets: []*copilotpb.SourceSnippet{{
			Path: "apps/incident-service/internal/store/postgres.go", Content: "pool, err := pgxpool.New(ctx, dsn)",
			StartLine: 20, EndLine: 20,
		}},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if analysis.GetIncidentId() != "inc-1" || analysis.GetSeverity() != "CRITICAL" || analysis.GetConfidenceScore() != "0.96" {
		t.Fatalf("unexpected analysis response: %+v", analysis)
	}
	if len(analysis.GetAffectedComponents()) != 3 {
		t.Fatalf("affected components = %#v", analysis.GetAffectedComponents())
	}
	if analysis.GetAffectedComponents()[1] != "PostgreSQL Connection Pool (pgxpool)" || !strings.Contains(analysis.GetRootCause(), "pgx connection pool") {
		t.Fatalf("Go database failure was not diagnosed correctly: %+v", analysis)
	}
	if analysis.GetUnifiedDiff() != "" || !strings.Contains(analysis.GetVerificationPlan(), "https://ci.example/build/42") ||
		!strings.Contains(analysis.GetRollbackPlan(), "abc123") {
		t.Fatalf("source-aware fallback fields were not mapped: %+v", analysis)
	}
	if len(analysis.GetCitedSourcePaths()) != 1 || analysis.GetCitedSourcePaths()[0] != "apps/incident-service/internal/store/postgres.go" {
		t.Fatalf("cited source paths = %#v", analysis.GetCitedSourcePaths())
	}

	remediationResponse, err := client.GenerateRemediationScript(ctx, &copilotpb.RemediationRequest{
		IncidentId: "inc-1", RootCause: analysis.GetRootCause(), TargetSystem: "incident-service",
	})
	if err != nil {
		t.Fatalf("remediate: %v", err)
	}
	if remediationResponse.GetScriptType() != "KUBECTL_ROLLBACK" || !remediationResponse.GetRequiresManualApproval() {
		t.Fatalf("unexpected remediation response: %+v", remediationResponse)
	}
	if !strings.Contains(remediationResponse.GetExecutableScript(), "-n sre-copilot") {
		t.Fatalf("remediation namespace missing: %s", remediationResponse.GetExecutableScript())
	}
	if remediationResponse.GetVerificationPlan() == "" || remediationResponse.GetRollbackPlan() == "" || remediationResponse.GetUnifiedDiff() != "" {
		t.Fatalf("structured remediation fields were not mapped: %+v", remediationResponse)
	}
	if count := latency.Snapshot().Count; count != 2 {
		t.Fatalf("metric count = %d, want 2", count)
	}
}

func TestGRPCMapsProviderUnifiedDiffAndPlans(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider := grpcProviderFunc(func(context.Context, string) (string, error) {
		return `{
  "root_cause": "The handler omits the required database timeout.",
  "confidence_score": "0.98",
  "severity": "critical",
  "affected_components": ["database client"],
  "immediate_mitigation": "Roll back the canary.",
  "unified_diff": "--- a/internal/handler.go\n+++ b/internal/handler.go\n@@ -1 +1 @@\n-old\n+new",
  "verification_plan": "Run go test ./...",
  "rollback_plan": "Revert abc123",
  "cited_source_paths": ["internal/handler.go"]
}`, nil
	})
	service := NewServer(
		triage.NewAnalyzer(provider, logger), remediation.NewGenerator("sre-copilot"),
		metrics.NewInferenceHistogram("ai-copilot-service"), logger,
	)
	response, err := service.AnalyzeIncident(context.Background(), &copilotpb.IncidentAnalysisRequest{
		IncidentId: "inc-structured", ServiceName: "api", ErrorLogs: "handler failed",
		FiringRule: "RequestFailure", CommitSha: "abc123",
		SourceSnippets: []*copilotpb.SourceSnippet{{Path: "internal/handler.go", Content: "old"}},
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if !strings.Contains(response.GetUnifiedDiff(), "+++ b/internal/handler.go") ||
		response.GetVerificationPlan() != "Run go test ./..." || response.GetRollbackPlan() != "Revert abc123" {
		t.Fatalf("structured response fields were not mapped: %+v", response)
	}
	if len(response.GetCitedSourcePaths()) != 1 || response.GetCitedSourcePaths()[0] != "internal/handler.go" {
		t.Fatalf("cited source paths = %#v", response.GetCitedSourcePaths())
	}
}

func TestProtoAdditionsPreserveEstablishedFieldNumbers(t *testing.T) {
	assertFields := func(t *testing.T, fields protoreflect.FieldDescriptors, expected map[string]int) {
		t.Helper()
		for name, number := range expected {
			field := fields.ByName(protoreflect.Name(name))
			if field == nil {
				t.Errorf("protobuf field %q is missing", name)
				continue
			}
			if got := int(field.Number()); got != number {
				t.Errorf("protobuf field %q number = %d, want %d", name, got, number)
			}
		}
	}

	assertFields(t, (&copilotpb.IncidentAnalysisRequest{}).ProtoReflect().Descriptor().Fields(), map[string]int{
		"incident_id": 1, "service_name": 2, "error_logs": 3, "firing_rule": 4, "environment": 5,
		"provider": 6, "repository": 7, "branch": 8, "commit_sha": 9, "commit_message": 10,
		"source_snippets": 11, "ci_provider": 12, "build_url": 13,
	})
	assertFields(t, (&copilotpb.SourceSnippet{}).ProtoReflect().Descriptor().Fields(), map[string]int{
		"path": 1, "content": 2, "start_line": 3, "end_line": 4,
	})
	assertFields(t, (&copilotpb.IncidentAnalysisResponse{}).ProtoReflect().Descriptor().Fields(), map[string]int{
		"incident_id": 1, "root_cause": 2, "confidence_score": 3, "affected_components": 4,
		"immediate_mitigation": 5, "severity": 6, "unified_diff": 7, "verification_plan": 8,
		"rollback_plan": 9, "cited_source_paths": 10,
	})
	assertFields(t, (&copilotpb.RemediationResponse{}).ProtoReflect().Descriptor().Fields(), map[string]int{
		"script_type": 1, "executable_script": 2, "requires_manual_approval": 3,
		"unified_diff": 4, "verification_plan": 5, "rollback_plan": 6,
	})
}

func TestGRPCServerRejectsNilRequests(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewServer(
		triage.NewAnalyzer(nil, logger),
		remediation.NewGenerator("sre-copilot"),
		metrics.NewInferenceHistogram("ai-copilot-service"),
		logger,
	)
	if _, err := service.AnalyzeIncident(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("AnalyzeIncident error = %v", err)
	}
	if _, err := service.GenerateRemediationScript(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("GenerateRemediationScript error = %v", err)
	}
}
