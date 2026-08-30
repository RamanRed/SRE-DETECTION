package copilot

import (
	"context"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
	"github.com/RamanRed/SRE-DETECTION/gen/copilotpb"
	"google.golang.org/grpc"
)

type Client struct {
	client copilotpb.IncidentCopilotServiceClient
}

func New(connection grpc.ClientConnInterface) *Client {
	return &Client{client: copilotpb.NewIncidentCopilotServiceClient(connection)}
}

func (c *Client) AnalyzeIncident(ctx context.Context, request service.CopilotAnalysisRequest) (service.CopilotAnalysisResult, error) {
	snippets := make([]*copilotpb.SourceSnippet, 0, len(request.SourceSnippets))
	for _, snippet := range request.SourceSnippets {
		snippets = append(snippets, &copilotpb.SourceSnippet{
			Path: snippet.Path, Content: snippet.Content,
			StartLine: snippet.StartLine, EndLine: snippet.EndLine,
		})
	}
	response, err := c.client.AnalyzeIncident(ctx, &copilotpb.IncidentAnalysisRequest{
		IncidentId:  request.IncidentID,
		ServiceName: request.ServiceName,
		ErrorLogs:   request.ErrorLogs,
		FiringRule:  request.FiringRule,
		Environment: request.Environment,
		Provider:    request.Commit.Provider, Repository: request.Commit.Repository,
		Branch: request.Commit.Branch, CommitSha: request.Commit.SHA,
		CommitMessage: request.Commit.Message, SourceSnippets: snippets,
		CiProvider: request.CIProvider, BuildUrl: request.BuildURL,
	})
	if err != nil {
		return service.CopilotAnalysisResult{}, err
	}
	return service.CopilotAnalysisResult{
		IncidentID:          response.GetIncidentId(),
		RootCause:           response.GetRootCause(),
		ImmediateMitigation: response.GetImmediateMitigation(),
		ConfidenceScore:     response.GetConfidenceScore(),
		Severity:            response.GetSeverity(),
		AffectedComponents:  append([]string(nil), response.GetAffectedComponents()...),
		UnifiedDiff:         response.GetUnifiedDiff(),
		VerificationPlan:    response.GetVerificationPlan(),
		RollbackPlan:        response.GetRollbackPlan(),
		CitedSourcePaths:    append([]string(nil), response.GetCitedSourcePaths()...),
	}, nil
}

func (c *Client) GenerateRemediation(ctx context.Context, request service.CopilotRemediationRequest) (service.CopilotRemediationResult, error) {
	response, err := c.client.GenerateRemediationScript(ctx, &copilotpb.RemediationRequest{
		IncidentId:   request.IncidentID,
		RootCause:    request.RootCause,
		TargetSystem: request.TargetSystem,
	})
	if err != nil {
		return service.CopilotRemediationResult{}, err
	}
	return service.CopilotRemediationResult{
		ScriptType:             response.GetScriptType(),
		ExecutableScript:       response.GetExecutableScript(),
		RequiresManualApproval: response.GetRequiresManualApproval(),
		UnifiedDiff:            response.GetUnifiedDiff(),
		VerificationPlan:       response.GetVerificationPlan(),
		RollbackPlan:           response.GetRollbackPlan(),
	}, nil
}
