package triage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RamanRed/SRE-DETECTION/apps/ai-copilot-service/internal/llm"
)

const (
	providerMitigation = "Execute automated rollback or scale deployment replicas to relieve system pressure."
	fallbackMitigation = "Isolate failing database connection pool and execute automated restart."
)

type Input struct {
	IncidentID     string
	ServiceName    string
	ErrorLogs      string
	FiringRule     string
	Environment    string
	Provider       string
	Repository     string
	Branch         string
	CommitSHA      string
	CommitMessage  string
	SourceSnippets []SourceSnippet
	CIProvider     string
	BuildURL       string
}

// SourceSnippet is repository evidence captured near a suspected failure.
// It is data for diagnosis only; the AI service never reads or modifies the
// referenced path on disk.
type SourceSnippet struct {
	Path      string
	Content   string
	StartLine int32
	EndLine   int32
}

type Result struct {
	IncidentID          string
	RootCause           string
	ConfidenceScore     string
	AffectedComponents  []string
	ImmediateMitigation string
	Severity            string
	UnifiedDiff         string
	VerificationPlan    string
	RollbackPlan        string
	CitedSourcePaths    []string
}

type Analyzer struct {
	provider llm.Provider
	logger   *slog.Logger
}

func NewAnalyzer(provider llm.Provider, logger *slog.Logger) *Analyzer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Analyzer{provider: provider, logger: logger}
}

func (analyzer *Analyzer) Analyze(ctx context.Context, input Input) Result {
	severity, confidence, affected := classify(input)
	result := Result{
		IncidentID:          input.IncidentID,
		RootCause:           heuristicDiagnosis(input),
		ConfidenceScore:     confidence,
		AffectedComponents:  affected,
		ImmediateMitigation: fallbackMitigation,
		Severity:            severity,
		VerificationPlan:    fallbackVerificationPlan(input),
		RollbackPlan:        fallbackRollbackPlan(input),
		CitedSourcePaths:    relevantSourcePaths(input),
	}

	if analyzer.provider != nil {
		providerOutput, err := analyzer.provider.Complete(ctx, analysisPrompt(input))
		if err != nil || strings.TrimSpace(providerOutput) == "" {
			if err == nil {
				err = fmt.Errorf("provider returned empty content")
			}
			analyzer.logger.WarnContext(ctx, "AI inference provider unavailable; using heuristic diagnosis", "error", err)
		} else if structured, ok := parseStructuredAnalysis(providerOutput, input); ok {
			result.RootCause = structured.RootCause
			result.ImmediateMitigation = valueOrDefault(structured.ImmediateMitigation, providerMitigation)
			result.UnifiedDiff = structured.UnifiedDiff
			result.VerificationPlan = valueOrDefault(structured.VerificationPlan, result.VerificationPlan)
			result.RollbackPlan = valueOrDefault(structured.RollbackPlan, result.RollbackPlan)
			result.CitedSourcePaths = structured.CitedSourcePaths
			result.AffectedComponents = appendUnique(result.AffectedComponents, structured.AffectedComponents...)
			if severityRank(structured.Severity) > severityRank(result.Severity) {
				result.Severity = structured.Severity
				if validConfidence(string(structured.ConfidenceScore)) {
					result.ConfidenceScore = string(structured.ConfidenceScore)
				}
			}
		} else if looksStructured(providerOutput) {
			analyzer.logger.WarnContext(ctx, "AI inference provider returned invalid structured analysis; using heuristic diagnosis")
		} else {
			// Older providers returned only a root-cause paragraph. Continue to
			// accept that format so existing OpenAI-compatible integrations do not
			// break while callers adopt the structured fields.
			result.RootCause = boundedText(providerOutput, maxRootCauseBytes)
			result.ImmediateMitigation = providerMitigation
		}
	}
	return result
}

func classify(input Input) (string, string, []string) {
	severity := "HIGH"
	confidence := "0.94"
	affected := []string{input.ServiceName}
	lowerLogs := strings.ToLower(input.ErrorLogs)
	if isGoOutOfMemory(lowerLogs) {
		severity = "CRITICAL"
		confidence = "0.98"
		affected = append(affected, "Go Runtime", "Kubernetes Pod Memory Limits")
	} else if strings.Contains(lowerLogs, "outofmemory") ||
		strings.Contains(lowerLogs, "heap space") ||
		strings.Contains(lowerLogs, "out of memory") ||
		strings.Contains(lowerLogs, "oomkilled") ||
		strings.Contains(lowerLogs, "oom killed") {
		severity = "CRITICAL"
		confidence = "0.98"
		if strings.Contains(lowerLogs, "outofmemory") || strings.Contains(lowerLogs, "heap space") {
			affected = append(affected, "JVM Runtime", "Kubernetes Pod Memory Limits")
		} else {
			affected = append(affected, "Application Runtime", "Kubernetes Pod Memory Limits")
		}
	} else if isGoDatabaseFailure(lowerLogs) {
		severity = "CRITICAL"
		confidence = "0.96"
		affected = append(affected, "PostgreSQL Connection Pool (pgxpool)", "Database Network Route")
	} else if strings.Contains(lowerLogs, "connection refused") ||
		strings.Contains(lowerLogs, "hikari") ||
		strings.Contains(lowerLogs, "psqlexception") {
		severity = "CRITICAL"
		confidence = "0.96"
		affected = append(affected, "PostgreSQL Connection Pool (HikariCP)", "Database Network Route")
	}

	return severity, confidence, affected
}

func analysisPrompt(input Input) string {
	return buildAnalysisPrompt(input)
}

func heuristicDiagnosis(input Input) string {
	logs := input.ErrorLogs
	if strings.Contains(logs, "HikariPool") || strings.Contains(logs, "Connection refused") || strings.Contains(logs, "PSQLException") {
		return "HikariCP database connection pool exhaustion detected. Target PostgreSQL database is either unreachable, rejecting connections due to max_connections breach, or timed out during active query execution."
	}
	lowerLogs := strings.ToLower(logs)
	if isGoDatabaseFailure(lowerLogs) {
		return "PostgreSQL connectivity failure detected in the pgx connection pool. The database is unreachable, resetting connections, or rejecting new sessions under resource pressure."
	}
	if isGoOutOfMemory(lowerLogs) {
		return "Go runtime memory exhaustion detected. The process could not allocate additional memory due to an allocation spike, memory leak, or constrained Kubernetes pod memory limit."
	}
	if strings.Contains(logs, "OutOfMemoryError") || strings.Contains(logs, "Java heap space") {
		return "JVM Heap exhaustion (OutOfMemoryError) detected due to uncollected memory allocation spike or resource leak in stream processing pipeline."
	}
	if strings.Contains(logs, "TimeoutException") || strings.Contains(logs, "504 Gateway") {
		return "Upstream dependency latency breach detected causing thread pool saturation and downstream request timeout cascade."
	}
	return "Detected anomalous execution failure in service '" + input.ServiceName +
		"' triggered by rule '" + input.FiringRule + "'. Stack trace indicates abnormal termination during active request processing."
}

func isGoOutOfMemory(lowerLogs string) bool {
	return strings.Contains(lowerLogs, "runtime: out of memory") ||
		strings.Contains(lowerLogs, "fatal error: out of memory")
}

func isGoDatabaseFailure(lowerLogs string) bool {
	return strings.Contains(lowerLogs, "pgx") ||
		strings.Contains(lowerLogs, "pgxpool") ||
		strings.Contains(lowerLogs, "postgresql") ||
		strings.Contains(lowerLogs, "failed to connect") ||
		strings.Contains(lowerLogs, "connection reset")
}
