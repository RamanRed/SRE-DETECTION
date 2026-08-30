package httpapi

import (
	"net/http"
	"strings"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

func (h *Handler) integrationConfig(response http.ResponseWriter, request *http.Request) {
	userID := request.URL.Query().Get("userId")
	if claims, ok := requestClaims(request); ok {
		userID = claims.UserID
	}
	if userID == "" {
		userID = "ramanred"
	}
	config, err := h.integrations.Config(request.Context(), userID)
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, integrationDTO(config))
}

func (h *Handler) saveIntegrationConfig(response http.ResponseWriter, request *http.Request) {
	var payload integrationConfigRequest
	if err := h.decodeJSON(response, request, &payload); err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(payload.UserID) == "" {
		payload.UserID = "ramanred"
	}
	if claims, ok := requestClaims(request); ok {
		payload.UserID = claims.UserID
	}
	config, err := h.integrations.SaveConfig(request.Context(), service.IntegrationConfigInput{
		UserID: payload.UserID, Username: payload.Username, GitHubToken: payload.GitHubToken,
		GitHubRepo: payload.GitHubRepo, GitHubBranch: payload.GitHubBranch,
		JenkinsURL: payload.JenkinsURL, JenkinsUsername: payload.JenkinsUsername,
		JenkinsAPIToken: payload.JenkinsAPIToken, JenkinsJobName: payload.JenkinsJobName,
		RepositoryProvider: payload.RepositoryProvider, RepositoryURL: payload.RepositoryURL,
		TargetBranch: payload.TargetBranch, RepositoryToken: payload.RepositoryToken,
		PipelineEngine: payload.PipelineEngine, CIBaseURL: payload.CIBaseURL,
		CIUsername: payload.CIUsername, CIToken: payload.CIToken, JobName: payload.JobName,
		PollingCadence: payload.PollingCadence, AutoRebuild: payload.AutoRebuild,
		AutoAITriage: payload.AutoAITriage,
	})
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, integrationDTO(config))
}

func (h *Handler) connectIntegration(response http.ResponseWriter, request *http.Request) {
	var payload integrationConfigRequest
	if err := h.decodeJSON(response, request, &payload); err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if strings.TrimSpace(payload.UserID) == "" {
		payload.UserID = "ramanred"
	}
	if claims, ok := requestClaims(request); ok {
		payload.UserID = claims.UserID
	}
	result, err := h.integrations.Connect(request.Context(), service.IntegrationConfigInput{
		UserID: payload.UserID, Username: payload.Username,
		RepositoryProvider: payload.RepositoryProvider, RepositoryURL: payload.RepositoryURL,
		TargetBranch: payload.TargetBranch, RepositoryToken: payload.RepositoryToken,
		PipelineEngine: payload.PipelineEngine, CIBaseURL: payload.CIBaseURL,
		CIUsername: payload.CIUsername, CIToken: payload.CIToken, JobName: payload.JobName,
		PollingCadence: payload.PollingCadence, AutoRebuild: payload.AutoRebuild,
		AutoAITriage: payload.AutoAITriage,
	})
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	dto := integrationDTO(result.Integration)
	dto.Message = &result.Message
	h.writeJSON(response, http.StatusOK, dto)
}

func (h *Handler) syncIntegrations(response http.ResponseWriter, request *http.Request) {
	userID := request.URL.Query().Get("userId")
	if claims, ok := requestClaims(request); ok {
		userID = claims.UserID
	}
	if userID == "" {
		userID = "ramanred"
	}
	result, err := h.integrations.Sync(request.Context(), userID)
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, platformSyncResponse{
		Success: result.Success, GitHubStatus: result.GitHubStatus, JenkinsStatus: result.JenkinsStatus,
		CommitsSynced: result.CommitsSynced, BuildsSynced: result.BuildsSynced, Message: result.Message,
	})
}
