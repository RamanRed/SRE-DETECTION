package triage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
)

type providerFunc func(context.Context, string) (string, error)

func (function providerFunc) Complete(ctx context.Context, prompt string) (string, error) {
	return function(ctx, prompt)
}

func TestAnalyzerHeuristicCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		logs       string
		rootCause  string
		severity   string
		confidence string
		affected   []string
	}{
		{
			name:       "database",
			logs:       "HikariPool-1 Connection refused with PSQLException",
			rootCause:  "HikariCP database connection pool exhaustion detected. Target PostgreSQL database is either unreachable, rejecting connections due to max_connections breach, or timed out during active query execution.",
			severity:   "CRITICAL",
			confidence: "0.96",
			affected:   []string{"orders", "PostgreSQL Connection Pool (HikariCP)", "Database Network Route"},
		},
		{
			name:       "Go pgx PostgreSQL database failure",
			logs:       "PGXPOOL: failed to connect to PostgreSQL: connection reset by peer",
			rootCause:  "PostgreSQL connectivity failure detected in the pgx connection pool. The database is unreachable, resetting connections, or rejecting new sessions under resource pressure.",
			severity:   "CRITICAL",
			confidence: "0.96",
			affected:   []string{"orders", "PostgreSQL Connection Pool (pgxpool)", "Database Network Route"},
		},
		{
			name:       "out of memory",
			logs:       "java.lang.OutOfMemoryError: Java heap space",
			rootCause:  "JVM Heap exhaustion (OutOfMemoryError) detected due to uncollected memory allocation spike or resource leak in stream processing pipeline.",
			severity:   "CRITICAL",
			confidence: "0.98",
			affected:   []string{"orders", "JVM Runtime", "Kubernetes Pod Memory Limits"},
		},
		{
			name:       "Go runtime out of memory",
			logs:       "runtime: out of memory: cannot allocate 4194304-byte block",
			rootCause:  "Go runtime memory exhaustion detected. The process could not allocate additional memory due to an allocation spike, memory leak, or constrained Kubernetes pod memory limit.",
			severity:   "CRITICAL",
			confidence: "0.98",
			affected:   []string{"orders", "Go Runtime", "Kubernetes Pod Memory Limits"},
		},
		{
			name:       "Go fatal out of memory",
			logs:       "fatal error: out of memory",
			rootCause:  "Go runtime memory exhaustion detected. The process could not allocate additional memory due to an allocation spike, memory leak, or constrained Kubernetes pod memory limit.",
			severity:   "CRITICAL",
			confidence: "0.98",
			affected:   []string{"orders", "Go Runtime", "Kubernetes Pod Memory Limits"},
		},
		{
			name:       "timeout",
			logs:       "TimeoutException following 504 Gateway",
			rootCause:  "Upstream dependency latency breach detected causing thread pool saturation and downstream request timeout cascade.",
			severity:   "HIGH",
			confidence: "0.94",
			affected:   []string{"orders"},
		},
		{
			name:       "generic",
			logs:       "unexpected exit",
			rootCause:  "Detected anomalous execution failure in service 'orders' triggered by rule 'ProcessFailed'. Stack trace indicates abnormal termination during active request processing.",
			severity:   "HIGH",
			confidence: "0.94",
			affected:   []string{"orders"},
		},
		{
			name:       "case-sensitive root and case-insensitive classification",
			logs:       "hikaripool unavailable",
			rootCause:  "Detected anomalous execution failure in service 'orders' triggered by rule 'ProcessFailed'. Stack trace indicates abnormal termination during active request processing.",
			severity:   "CRITICAL",
			confidence: "0.96",
			affected:   []string{"orders", "PostgreSQL Connection Pool (HikariCP)", "Database Network Route"},
		},
		{
			name:       "mixed keywords retain independent precedence",
			logs:       "HikariPool failed with OutOfMemoryError",
			rootCause:  "HikariCP database connection pool exhaustion detected. Target PostgreSQL database is either unreachable, rejecting connections due to max_connections breach, or timed out during active query execution.",
			severity:   "CRITICAL",
			confidence: "0.98",
			affected:   []string{"orders", "JVM Runtime", "Kubernetes Pod Memory Limits"},
		},
	}

	analyzer := NewAnalyzer(nil, discardLogger())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := analyzer.Analyze(context.Background(), Input{
				IncidentID: "inc-1", ServiceName: "orders", ErrorLogs: test.logs,
				FiringRule: "ProcessFailed", Environment: "production",
			})
			if result.IncidentID != "inc-1" || result.RootCause != test.rootCause || result.Severity != test.severity || result.ConfidenceScore != test.confidence {
				t.Fatalf("unexpected result: %+v", result)
			}
			if result.ImmediateMitigation != fallbackMitigation {
				t.Errorf("mitigation = %q", result.ImmediateMitigation)
			}
			if !slices.Equal(result.AffectedComponents, test.affected) {
				t.Errorf("affected = %#v, want %#v", result.AffectedComponents, test.affected)
			}
		})
	}
}

func TestGoDatabaseFailurePhrasesAreCaseInsensitive(t *testing.T) {
	for _, logs := range []string{
		"PGX returned an error",
		"PgxPool acquire failed",
		"POSTGRESQL server unavailable",
		"Failed To Connect to database host",
		"CONNECTION RESET BY PEER",
	} {
		if !isGoDatabaseFailure(strings.ToLower(logs)) {
			t.Errorf("database failure was not recognized: %q", logs)
		}
	}
}

func TestAnalyzerUsesProviderOutputAndPreservesClassification(t *testing.T) {
	var capturedPrompt string
	analyzer := NewAnalyzer(providerFunc(func(_ context.Context, prompt string) (string, error) {
		capturedPrompt = prompt
		return "provider diagnosis", nil
	}), discardLogger())
	result := analyzer.Analyze(context.Background(), Input{
		IncidentID: "inc-2", ServiceName: "billing", Environment: "staging",
		FiringRule: "DatabaseDown", ErrorLogs: "Connection refused",
	})
	if result.RootCause != "provider diagnosis" || result.ImmediateMitigation != providerMitigation {
		t.Fatalf("provider output not used: %+v", result)
	}
	if result.Severity != "CRITICAL" || result.ConfidenceScore != "0.96" {
		t.Fatalf("heuristic classification not preserved: %+v", result)
	}
	for _, fragment := range []string{"Service: billing", "Environment: staging", "Firing Alert: DatabaseDown", "Connection refused"} {
		if !strings.Contains(capturedPrompt, fragment) {
			t.Errorf("prompt missing %q: %s", fragment, capturedPrompt)
		}
	}
}

func TestAnalyzerBoundsLegacyProviderOutput(t *testing.T) {
	analyzer := NewAnalyzer(providerFunc(func(context.Context, string) (string, error) {
		return strings.Repeat("diagnosis ", maxRootCauseBytes), nil
	}), discardLogger())
	result := analyzer.Analyze(context.Background(), Input{ServiceName: "billing", FiringRule: "BuildFailed"})
	if len(result.RootCause) > maxRootCauseBytes {
		t.Fatalf("legacy provider root cause length = %d, want <= %d", len(result.RootCause), maxRootCauseBytes)
	}
}

func TestAnalyzerFallsBackOnProviderFailureOrEmptyContent(t *testing.T) {
	providers := []providerFunc{
		func(context.Context, string) (string, error) { return "", errors.New("unavailable") },
		func(context.Context, string) (string, error) { return "  ", nil },
	}
	for index, provider := range providers {
		analyzer := NewAnalyzer(provider, discardLogger())
		result := analyzer.Analyze(context.Background(), Input{ServiceName: "api", FiringRule: "Failed"})
		if result.ImmediateMitigation != fallbackMitigation || !strings.HasPrefix(result.RootCause, "Detected anomalous") {
			t.Errorf("provider %d did not fall back: %+v", index, result)
		}
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
