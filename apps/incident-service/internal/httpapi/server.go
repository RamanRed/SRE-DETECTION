package httpapi

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

type IncidentOperations interface {
	CreateIncident(context.Context, service.CreateIncidentInput) (domain.Incident, error)
	ListIncidents(context.Context, int, int) (domain.Page[domain.Incident], error)
	ActiveIncidents(context.Context) ([]domain.Incident, error)
	Incident(context.Context, string) (domain.Incident, error)
	Triage(context.Context, string) (service.TriageResult, error)
	GenerateRemediation(context.Context, string) (service.RemediationResult, error)
	ApproveRemediation(context.Context, service.ApproveRemediationInput) (service.RemediationResult, error)
	Stats(context.Context) (domain.DashboardStats, error)
	LatestAnalysis(context.Context, string) (*domain.IncidentAnalysis, error)
}

type PipelineOperations interface {
	RecordBuild(context.Context, service.WebhookInput) (domain.PipelineBuild, error)
	Builds(context.Context, int, int) (domain.Page[domain.PipelineBuild], error)
	Metrics(context.Context) (service.DORAMetrics, error)
}

type IntegrationOperations interface {
	Config(context.Context, string) (domain.PlatformIntegration, error)
	SaveConfig(context.Context, service.IntegrationConfigInput) (domain.PlatformIntegration, error)
	Sync(context.Context, string) (service.PlatformSyncResult, error)
	Connect(context.Context, service.IntegrationConfigInput) (service.ConnectResult, error)
}

type AuthOperations interface {
	Login(*string, *string, *string) (service.AuthResponse, error)
	CurrentUser(service.AuthClaims) service.AuthResponse
	VerifyToken(string) (service.AuthClaims, error)
}

type Dependencies struct {
	Incidents      IncidentOperations
	Pipelines      PipelineOperations
	Integrations   IntegrationOperations
	Auth           AuthOperations
	Management     http.Handler
	Clock          service.Clock
	Logger         *slog.Logger
	Version        string
	RequireAuth    bool
	AllowedOrigins []string
	CIWebhookToken string
}

type Handler struct {
	incidents      IncidentOperations
	pipelines      PipelineOperations
	integrations   IntegrationOperations
	auth           AuthOperations
	clock          service.Clock
	logger         *slog.Logger
	version        string
	requireAuth    bool
	ciWebhookToken string
}

func New(dependencies Dependencies) http.Handler {
	if dependencies.Clock == nil {
		dependencies.Clock = service.SystemClock
	}
	if dependencies.Logger == nil {
		dependencies.Logger = slog.Default()
	}
	handler := &Handler{
		incidents: dependencies.Incidents, pipelines: dependencies.Pipelines,
		integrations: dependencies.Integrations, auth: dependencies.Auth,
		clock: dependencies.Clock, logger: dependencies.Logger, version: dependencies.Version,
		requireAuth:    dependencies.RequireAuth,
		ciWebhookToken: dependencies.CIWebhookToken,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/incidents", handler.createIncident)
	mux.HandleFunc("GET /api/v1/incidents", handler.listIncidents)
	mux.HandleFunc("GET /api/v1/incidents/active", handler.activeIncidents)
	mux.HandleFunc("GET /api/v1/incidents/stats/dashboard", handler.dashboardStats)
	mux.HandleFunc("GET /api/v1/incidents/health", handler.incidentHealth)
	mux.HandleFunc("GET /api/v1/incidents/{id}", handler.getIncident)
	mux.HandleFunc("GET /api/v1/incidents/{id}/analysis", handler.getIncidentAnalysis)
	mux.HandleFunc("POST /api/v1/incidents/{id}/triage", handler.triageIncident)
	mux.HandleFunc("POST /api/v1/incidents/{id}/remediate", handler.remediateIncident)
	mux.HandleFunc("POST /api/v1/incidents/{id}/remediation/{rid}/approve", handler.approveRemediation)

	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.HandleFunc("GET /api/v1/auth/me", handler.currentUser)

	mux.HandleFunc("POST /api/v1/ci/webhook", handler.pipelineWebhook)
	mux.HandleFunc("GET /api/v1/ci/builds", handler.pipelineBuilds)
	mux.HandleFunc("GET /api/v1/ci/metrics/dora", handler.doraMetrics)
	mux.HandleFunc("POST /api/v1/ci/sync", handler.pipelineSync)

	mux.HandleFunc("GET /api/v1/integrations/config", handler.integrationConfig)
	mux.HandleFunc("POST /api/v1/integrations/config", handler.saveIntegrationConfig)
	mux.HandleFunc("POST /api/v1/integrations/sync", handler.syncIntegrations)
	mux.HandleFunc("POST /api/v1/integrations/connect", handler.connectIntegration)
	if dependencies.Management != nil {
		mux.Handle("/actuator/", dependencies.Management)
	}
	mux.HandleFunc("/", handler.notFound)
	secured := authorize(handler, mux)
	return cors(secured, dependencies.AllowedOrigins, dependencies.RequireAuth)
}

func cors(next http.Handler, allowedOrigins []string, secure bool) http.Handler {
	if len(allowedOrigins) == 0 {
		if !secure {
			allowedOrigins = []string{"*"}
		}
	}
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		_, wildcard := allowed["*"]
		_, permitted := allowed[origin]
		if wildcard {
			response.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && permitted {
			response.Header().Set("Access-Control-Allow-Origin", origin)
			response.Header().Add("Vary", "Origin")
		}
		response.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		response.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		response.Header().Set("Access-Control-Max-Age", "1800")
		if request.Method == http.MethodOptions {
			if !wildcard && origin != "" && !permitted {
				response.WriteHeader(http.StatusForbidden)
				return
			}
			response.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(response, request)
	})
}

type authContextKey struct{}

func authorize(handler *Handler, next http.Handler) http.Handler {
	return recoverPanics(handler, http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if handler.requireAuth && request.Method == http.MethodPost && request.URL.Path == "/api/v1/ci/webhook" {
			if handler.ciWebhookToken == "" {
				handler.writeError(response, http.StatusServiceUnavailable, "CI webhook ingestion is disabled", nil)
				return
			}
			header := request.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				handler.writeError(response, http.StatusUnauthorized, "A valid CI webhook bearer token is required", nil)
				return
			}
			provided := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			if len(provided) != len(handler.ciWebhookToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(handler.ciWebhookToken)) != 1 {
				handler.writeError(response, http.StatusUnauthorized, "A valid CI webhook bearer token is required", nil)
				return
			}
			next.ServeHTTP(response, request)
			return
		}
		if !handler.requireAuth || !protectedRoute(request) {
			next.ServeHTTP(response, request)
			return
		}
		header := request.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || handler.auth == nil {
			handler.writeError(response, http.StatusUnauthorized, "A valid bearer session is required", nil)
			return
		}
		claims, err := handler.auth.VerifyToken(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
		if err != nil {
			handler.writeError(response, http.StatusUnauthorized, "Session is invalid or expired", nil)
			return
		}
		if !roleAllowed(request, claims.Role) {
			handler.writeError(response, http.StatusForbidden, "This role is not authorized for the requested operation", nil)
			return
		}
		ctx := context.WithValue(request.Context(), authContextKey{}, claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	}))
}

func protectedRoute(request *http.Request) bool {
	path := request.URL.Path
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	return path != "/api/v1/auth/login" && path != "/api/v1/incidents/health" && path != "/api/v1/ci/webhook"
}

func roleAllowed(request *http.Request, role string) bool {
	role = strings.ToUpper(strings.TrimSpace(role))
	if request.URL.Path == "/api/v1/auth/me" {
		return role == service.RoleSRELead || role == service.RoleDevOpsEngineer || role == service.RoleEvaluator
	}
	if strings.HasSuffix(request.URL.Path, "/approve") {
		return role == service.RoleSRELead
	}
	if strings.HasPrefix(request.URL.Path, "/api/v1/integrations/") || request.Method != http.MethodGet {
		return role == service.RoleSRELead || role == service.RoleDevOpsEngineer
	}
	return role == service.RoleSRELead || role == service.RoleDevOpsEngineer || role == service.RoleEvaluator
}

func requestClaims(request *http.Request) (service.AuthClaims, bool) {
	claims, ok := request.Context().Value(authContextKey{}).(service.AuthClaims)
	return claims, ok
}

func recoverPanics(handler *Handler, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				handler.logger.Error("recovered HTTP panic", "panic", recovered, "path", request.URL.Path)
				handler.writeError(response, http.StatusInternalServerError, "An unexpected error occurred", nil)
			}
		}()
		next.ServeHTTP(response, request)
	})
}
