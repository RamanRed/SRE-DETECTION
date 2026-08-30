package grpcapi

import (
	"context"
	"log/slog"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/metrics"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/remediation"
	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/triage"
	"github.com/RamanRed/SRE-DETECTION/gen/copilotpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	copilotpb.UnimplementedIncidentCopilotServiceServer

	analyzer    *triage.Analyzer
	remediation *remediation.Generator
	latency     *metrics.InferenceHistogram
	logger      *slog.Logger
}

func NewServer(
	analyzer *triage.Analyzer,
	remediationGenerator *remediation.Generator,
	latency *metrics.InferenceHistogram,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		analyzer:    analyzer,
		remediation: remediationGenerator,
		latency:     latency,
		logger:      logger,
	}
}

func (server *Server) AnalyzeIncident(
	ctx context.Context,
	request *copilotpb.IncidentAnalysisRequest,
) (*copilotpb.IncidentAnalysisResponse, error) {
	started := time.Now()
	defer func() {
		server.latency.Observe(time.Since(started))
	}()
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "incident analysis request is required")
	}

	server.logger.InfoContext(ctx, "received incident triage request",
		"incident_id", request.GetIncidentId(),
		"service", request.GetServiceName(),
		"rule", request.GetFiringRule(),
		"repository", request.GetRepository(),
		"commit_sha", request.GetCommitSha(),
		"source_snippet_count", len(request.GetSourceSnippets()),
	)
	sourceSnippets := make([]triage.SourceSnippet, 0, len(request.GetSourceSnippets()))
	for _, snippet := range request.GetSourceSnippets() {
		if snippet == nil {
			continue
		}
		sourceSnippets = append(sourceSnippets, triage.SourceSnippet{
			Path: snippet.GetPath(), Content: snippet.GetContent(),
			StartLine: snippet.GetStartLine(), EndLine: snippet.GetEndLine(),
		})
	}
	result := server.analyzer.Analyze(ctx, triage.Input{
		IncidentID: request.GetIncidentId(), ServiceName: request.GetServiceName(),
		ErrorLogs: request.GetErrorLogs(), FiringRule: request.GetFiringRule(),
		Environment: request.GetEnvironment(), Provider: request.GetProvider(),
		Repository: request.GetRepository(), Branch: request.GetBranch(),
		CommitSHA: request.GetCommitSha(), CommitMessage: request.GetCommitMessage(),
		SourceSnippets: sourceSnippets, CIProvider: request.GetCiProvider(), BuildURL: request.GetBuildUrl(),
	})

	server.logger.InfoContext(ctx, "completed incident analysis", "incident_id", request.GetIncidentId())
	return &copilotpb.IncidentAnalysisResponse{
		IncidentId:          result.IncidentID,
		RootCause:           result.RootCause,
		ConfidenceScore:     result.ConfidenceScore,
		AffectedComponents:  result.AffectedComponents,
		ImmediateMitigation: result.ImmediateMitigation,
		Severity:            result.Severity,
		UnifiedDiff:         result.UnifiedDiff,
		VerificationPlan:    result.VerificationPlan,
		RollbackPlan:        result.RollbackPlan,
		CitedSourcePaths:    result.CitedSourcePaths,
	}, nil
}

func (server *Server) GenerateRemediationScript(
	ctx context.Context,
	request *copilotpb.RemediationRequest,
) (*copilotpb.RemediationResponse, error) {
	started := time.Now()
	defer func() {
		server.latency.Observe(time.Since(started))
	}()
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "remediation request is required")
	}

	server.logger.InfoContext(ctx, "generating remediation script",
		"incident_id", request.GetIncidentId(),
		"target", request.GetTargetSystem(),
	)
	result := server.remediation.Generate(remediation.Input{
		IncidentID:   request.GetIncidentId(),
		RootCause:    request.GetRootCause(),
		TargetSystem: request.GetTargetSystem(),
	})
	return &copilotpb.RemediationResponse{
		ScriptType:             result.ScriptType,
		ExecutableScript:       result.ExecutableScript,
		RequiresManualApproval: result.RequiresManualApproval,
		UnifiedDiff:            result.UnifiedDiff,
		VerificationPlan:       result.VerificationPlan,
		RollbackPlan:           result.RollbackPlan,
	}, nil
}
