package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

func TestCreateIncidentIncludesRawLogsAndCORS(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 1, 2, 3000, time.UTC)
	operations := &stubIncidents{now: now}
	handler := New(Dependencies{
		Incidents: operations, Clock: func() time.Time { return now },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "1.0.0",
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", strings.NewReader(`{
		"title":"Database outage","serviceName":"payments","rawLogs":"dial refused","environment":"production"
	}`))
	request.Header.Set("Origin", "http://frontend.example")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("CORS header = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["rawLogs"] != "dial refused" || body["status"] != "OPEN" || body["createdAt"] != "2026-08-30T15:01:02.000003" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestIncidentValidationAndSpringPageEnvelope(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 0, 0, 0, time.UTC)
	operations := &stubIncidents{now: now}
	handler := New(Dependencies{Incidents: operations, Clock: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/v1/incidents", strings.NewReader(`{"title":" ","serviceName":""}`)))
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"fieldErrors"`) {
		t.Fatalf("validation response = %d %s", invalid.Code, invalid.Body.String())
	}

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/api/v1/incidents?page=0&size=20", nil))
	if page.Code != http.StatusOK {
		t.Fatalf("page status = %d body=%s", page.Code, page.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(page.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["totalElements"] != float64(1) || envelope["totalPages"] != float64(1) || envelope["first"] != true || envelope["last"] != true {
		t.Fatalf("unexpected page envelope: %#v", envelope)
	}
	if _, ok := envelope["pageable"]; !ok {
		t.Fatalf("pageable compatibility field missing: %#v", envelope)
	}
}

func TestPreflightDoesNotInvokeDependencies(t *testing.T) {
	handler := New(Dependencies{Clock: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodOptions, "/api/v1/incidents", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d", response.Code)
	}
}

func TestLatestAnalysisReturnsPersistedSourceAwareFields(t *testing.T) {
	operations := &stubIncidents{analysis: &domain.IncidentAnalysis{
		IncidentID: "incident-1", RootCause: "bad change", ImmediateMitigation: "rollback",
		ConfidenceScore: "0.95", Severity: "HIGH", AffectedComponents: []string{"api"},
		UnifiedDiff: "--- a/main.go", VerificationPlan: "go test ./...", RollbackPlan: "git revert",
		CitedSourcePaths: []string{"main.go"}, RepositoryURL: "https://github.com/acme/repo", CommitSHA: "abc123",
	}}
	handler := New(Dependencies{Incidents: operations, Clock: time.Now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/incidents/incident-1/analysis", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"unifiedDiff":"--- a/main.go"`) || !strings.Contains(response.Body.String(), `"commitSha":"abc123"`) {
		t.Fatalf("analysis response=%d %s", response.Code, response.Body.String())
	}
}

type stubIncidents struct {
	now          time.Time
	approveInput service.ApproveRemediationInput
	analysis     *domain.IncidentAnalysis
}

func (s *stubIncidents) CreateIncident(_ context.Context, input service.CreateIncidentInput) (domain.Incident, error) {
	return domain.Incident{
		ID: "incident-1", Title: input.Title, ServiceName: input.ServiceName, RawLogs: input.RawLogs,
		FiringRule: input.FiringRule, Environment: "production", Status: domain.IncidentOpen,
		CreatedBy: "system", CreatedAt: s.now, UpdatedAt: s.now,
	}, nil
}
func (s *stubIncidents) ListIncidents(_ context.Context, page, size int) (domain.Page[domain.Incident], error) {
	incident, _ := s.CreateIncident(context.Background(), service.CreateIncidentInput{Title: "x", ServiceName: "y"})
	return domain.Page[domain.Incident]{Content: []domain.Incident{incident}, TotalElements: 1, Page: page, Size: size}, nil
}
func (s *stubIncidents) ActiveIncidents(context.Context) ([]domain.Incident, error) {
	return []domain.Incident{}, nil
}
func (s *stubIncidents) Incident(context.Context, string) (domain.Incident, error) {
	return domain.Incident{}, nil
}
func (s *stubIncidents) LatestAnalysis(context.Context, string) (*domain.IncidentAnalysis, error) {
	return s.analysis, nil
}
func (s *stubIncidents) Triage(context.Context, string) (service.TriageResult, error) {
	return service.TriageResult{}, nil
}
func (s *stubIncidents) GenerateRemediation(context.Context, string) (service.RemediationResult, error) {
	return service.RemediationResult{}, nil
}
func (s *stubIncidents) ApproveRemediation(_ context.Context, input service.ApproveRemediationInput) (service.RemediationResult, error) {
	s.approveInput = input
	return service.RemediationResult{ExecutionStatus: domain.ExecutionApproved}, nil
}
func (s *stubIncidents) Stats(context.Context) (domain.DashboardStats, error) {
	return domain.DashboardStats{}, nil
}
