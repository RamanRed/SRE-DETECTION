package httpapi

import (
	"net/http"
	"strings"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/domain"
	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

func (h *Handler) createIncident(response http.ResponseWriter, request *http.Request) {
	var payload createIncidentRequest
	if err := h.decodeJSON(response, request, &payload); err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	fieldErrors := make(map[string]string)
	if strings.TrimSpace(payload.Title) == "" {
		fieldErrors["title"] = "Title is required"
	} else if maxCharacters(payload.Title) > 255 {
		fieldErrors["title"] = "Title must not exceed 255 characters"
	}
	if strings.TrimSpace(payload.ServiceName) == "" {
		fieldErrors["serviceName"] = "Service name is required"
	} else if maxCharacters(payload.ServiceName) > 100 {
		fieldErrors["serviceName"] = "Service name must not exceed 100 characters"
	}
	if len(fieldErrors) > 0 {
		h.writeError(response, http.StatusBadRequest, "Validation failed", fieldErrors)
		return
	}
	incident, err := h.incidents.CreateIncident(request.Context(), service.CreateIncidentInput{
		Title: payload.Title, ServiceName: payload.ServiceName, RawLogs: payload.RawLogs,
		FiringRule: payload.FiringRule, Environment: payload.Environment, CreatedBy: payload.CreatedBy,
	})
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusCreated, incidentDTO(incident))
}

func (h *Handler) listIncidents(response http.ResponseWriter, request *http.Request) {
	page, size, err := parsePagination(request)
	if err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.incidents.ListIncidents(request.Context(), page, size)
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	content := make([]incidentResponse, 0, len(result.Content))
	for _, incident := range result.Content {
		content = append(content, incidentDTO(incident))
	}
	h.writeJSON(response, http.StatusOK, pageDTO(content, result.TotalElements, result.Page, result.Size))
}

func (h *Handler) activeIncidents(response http.ResponseWriter, request *http.Request) {
	incidents, err := h.incidents.ActiveIncidents(request.Context())
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	content := make([]incidentResponse, 0, len(incidents))
	for _, incident := range incidents {
		content = append(content, incidentDTO(incident))
	}
	h.writeJSON(response, http.StatusOK, content)
}

func (h *Handler) getIncident(response http.ResponseWriter, request *http.Request) {
	incident, err := h.incidents.Incident(request.Context(), request.PathValue("id"))
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, incidentDTO(incident))
}

func (h *Handler) getIncidentAnalysis(response http.ResponseWriter, request *http.Request) {
	analysis, err := h.incidents.LatestAnalysis(request.Context(), request.PathValue("id"))
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	if analysis == nil {
		h.writeError(response, http.StatusNotFound, "Incident analysis not found", nil)
		return
	}
	affected := analysis.AffectedComponents
	if affected == nil {
		affected = []string{}
	}
	cited := analysis.CitedSourcePaths
	if cited == nil {
		cited = []string{}
	}
	h.writeJSON(response, http.StatusOK, triageResponse{
		IncidentID: analysis.IncidentID, RootCause: analysis.RootCause,
		ImmediateMitigation: analysis.ImmediateMitigation, ConfidenceScore: analysis.ConfidenceScore,
		Severity: analysis.Severity, AffectedComponents: affected,
		IncidentStatus: string(domain.IncidentAnalyzing), UnifiedDiff: analysis.UnifiedDiff,
		VerificationPlan: analysis.VerificationPlan, RollbackPlan: analysis.RollbackPlan,
		CitedSourcePaths: cited, RepositoryURL: analysis.RepositoryURL, CommitSHA: analysis.CommitSHA,
	})
}

func (h *Handler) triageIncident(response http.ResponseWriter, request *http.Request) {
	result, err := h.incidents.Triage(request.Context(), request.PathValue("id"))
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	affected := result.AffectedComponents
	if affected == nil {
		affected = []string{}
	}
	h.writeJSON(response, http.StatusOK, triageResponse{
		IncidentID: result.IncidentID, RootCause: result.RootCause,
		ImmediateMitigation: result.ImmediateMitigation, ConfidenceScore: result.ConfidenceScore,
		Severity: result.Severity, AffectedComponents: affected, IncidentStatus: string(result.IncidentStatus),
		UnifiedDiff: result.UnifiedDiff, VerificationPlan: result.VerificationPlan,
		RollbackPlan: result.RollbackPlan, CitedSourcePaths: result.CitedSourcePaths,
		RepositoryURL: result.RepositoryURL, CommitSHA: result.CommitSHA,
	})
}

func (h *Handler) remediateIncident(response http.ResponseWriter, request *http.Request) {
	result, err := h.incidents.GenerateRemediation(request.Context(), request.PathValue("id"))
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, remediationDTO(result))
}

func (h *Handler) approveRemediation(response http.ResponseWriter, request *http.Request) {
	var payload approveRequest
	if err := h.decodeJSON(response, request, &payload); err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if claims, ok := requestClaims(request); ok {
		payload.AppliedBy = claims.UserID
	}
	if strings.TrimSpace(payload.AppliedBy) == "" {
		h.writeError(response, http.StatusBadRequest, "Validation failed", map[string]string{"appliedBy": "Applied-by field is required"})
		return
	}
	result, err := h.incidents.ApproveRemediation(request.Context(), service.ApproveRemediationInput{
		IncidentID: request.PathValue("id"), RemediationID: request.PathValue("rid"), AppliedBy: payload.AppliedBy,
	})
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, remediationDTO(result))
}

func (h *Handler) dashboardStats(response http.ResponseWriter, request *http.Request) {
	stats, err := h.incidents.Stats(request.Context())
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, dashboardStatsResponse{
		OpenIncidents: stats.OpenIncidents, AnalyzingIncidents: stats.AnalyzingIncidents,
		ResolvedToday: stats.ResolvedToday, PendingRemediations: stats.PendingRemediations,
		AppliedRemediations: stats.AppliedRemediations,
	})
}

func (h *Handler) incidentHealth(response http.ResponseWriter, _ *http.Request) {
	h.writeJSON(response, http.StatusOK, map[string]string{
		"service": "incident-service", "status": "UP", "version": h.version,
	})
}

func (h *Handler) notFound(response http.ResponseWriter, _ *http.Request) {
	h.writeError(response, http.StatusNotFound, "Resource not found", nil)
}

func remediationDTO(result service.RemediationResult) remediationResponse {
	return remediationResponse{
		RemediationID: result.RemediationID, IncidentID: result.IncidentID,
		ScriptType: result.ScriptType, ExecutableScript: result.ExecutableScript,
		RequiresManualApproval: result.RequiresManualApproval, ExecutionStatus: string(result.ExecutionStatus),
		UnifiedDiff: result.UnifiedDiff, VerificationPlan: result.VerificationPlan,
		RollbackPlan: result.RollbackPlan,
	}
}
