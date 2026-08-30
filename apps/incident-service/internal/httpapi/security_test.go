package httpapi

import (
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

func TestSecureBoundaryRequiresSessionAndMakesEvaluatorReadOnly(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	auth, err := service.NewConfiguredAuthService(strings.Repeat("s", 32), "correct horse battery", service.RoleEvaluator, false, func() time.Time { return now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	username, password := "reviewer", "correct horse battery"
	profile, err := auth.Login(&username, &password, nil)
	if err != nil {
		t.Fatal(err)
	}
	incidents := &stubIncidents{now: now}
	handler := New(Dependencies{
		Incidents: incidents, Auth: auth, RequireAuth: true,
		AllowedOrigins: []string{"https://console.example"}, Clock: func() time.Time { return now },
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET status=%d body=%s", unauthorized.Code, unauthorized.Body.String())
	}

	readRequest := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	readRequest.Header.Set("Authorization", "Bearer "+profile.Token)
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("evaluator GET status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}

	writeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/incidents", strings.NewReader(`{"title":"x","serviceName":"y"}`))
	writeRequest.Header.Set("Authorization", "Bearer "+profile.Token)
	writeResponse := httptest.NewRecorder()
	handler.ServeHTTP(writeResponse, writeRequest)
	if writeResponse.Code != http.StatusForbidden {
		t.Fatalf("evaluator POST status=%d body=%s", writeResponse.Code, writeResponse.Body.String())
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/api/v1/incidents/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("public health status=%d", health.Code)
	}
}

func TestIntegrationResponseNeverSerializesCredentials(t *testing.T) {
	repositoryToken, ciToken := "repository-secret", "ci-secret"
	payload, err := json.Marshal(integrationDTO(domain.PlatformIntegration{
		UserID: "operator", RepositoryToken: &repositoryToken, CIToken: &ciToken,
	}))
	if err != nil {
		t.Fatal(err)
	}
	body := string(payload)
	if strings.Contains(body, repositoryToken) || strings.Contains(body, ciToken) || strings.Contains(body, `"repositoryToken"`) || strings.Contains(body, `"ciToken"`) {
		t.Fatalf("integration response leaked a credential: %s", body)
	}
	if !strings.Contains(body, `"repositoryTokenConfigured":true`) || !strings.Contains(body, `"ciTokenConfigured":true`) {
		t.Fatalf("write-only credential flags missing: %s", body)
	}
}

func TestApprovalRequiresLeadAndUsesSignedSubject(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	auth, err := service.NewConfiguredAuthService(strings.Repeat("k", 32), "correct horse battery", service.RoleSRELead, false, func() time.Time { return now }, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	username, password := "Alice Smith", "correct horse battery"
	profile, err := auth.Login(&username, &password, nil)
	if err != nil {
		t.Fatal(err)
	}
	incidents := &stubIncidents{now: now}
	handler := New(Dependencies{Incidents: incidents, Auth: auth, RequireAuth: true, Clock: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/i1/remediation/r1/approve", strings.NewReader(`{"appliedBy":"spoofed"}`))
	request.Header.Set("Authorization", "Bearer "+profile.Token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("approval status=%d body=%s", response.Code, response.Body.String())
	}
	if incidents.approveInput.AppliedBy != "alice-smith" {
		t.Fatalf("appliedBy = %q, want signed subject", incidents.approveInput.AppliedBy)
	}
}

func TestWebhookRequiresExactBearerSchemeAndCORSAllowlist(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	handler := New(Dependencies{
		RequireAuth: true, CIWebhookToken: "webhook-secret", AllowedOrigins: []string{"https://console.example"},
		Clock: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	rawRequest := httptest.NewRequest(http.MethodPost, "/api/v1/ci/webhook", strings.NewReader(`{}`))
	rawRequest.Header.Set("Authorization", "webhook-secret")
	rawResponse := httptest.NewRecorder()
	handler.ServeHTTP(rawResponse, rawRequest)
	if rawResponse.Code != http.StatusUnauthorized {
		t.Fatalf("raw webhook token status=%d", rawResponse.Code)
	}

	bearerRequest := httptest.NewRequest(http.MethodPost, "/api/v1/ci/webhook", strings.NewReader(`{}`))
	bearerRequest.Header.Set("Authorization", "Bearer webhook-secret")
	bearerResponse := httptest.NewRecorder()
	handler.ServeHTTP(bearerResponse, bearerRequest)
	if bearerResponse.Code != http.StatusBadRequest {
		t.Fatalf("valid webhook token did not reach validation: status=%d body=%s", bearerResponse.Code, bearerResponse.Body.String())
	}

	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/incidents", nil)
	preflight.Header.Set("Origin", "https://evil.example")
	preflightResponse := httptest.NewRecorder()
	handler.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusForbidden || preflightResponse.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("disallowed preflight status=%d origin=%q", preflightResponse.Code, preflightResponse.Header().Get("Access-Control-Allow-Origin"))
	}
	allowed := httptest.NewRequest(http.MethodOptions, "/api/v1/incidents", nil)
	allowed.Header.Set("Origin", "https://console.example")
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent || allowedResponse.Header().Get("Access-Control-Allow-Origin") != "https://console.example" {
		t.Fatalf("allowed preflight status=%d origin=%q", allowedResponse.Code, allowedResponse.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestSecureCORSNeverFallsBackToWildcardForEmptyAllowlist(t *testing.T) {
	handler := cors(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}), []string{"", "   "}, true)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if origin := response.Header().Get("Access-Control-Allow-Origin"); origin != "" {
		t.Fatalf("secure empty CORS allowlist returned origin %q", origin)
	}
}
