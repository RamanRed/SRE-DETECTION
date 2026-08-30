package httpapi

import (
	"net/http"
	"strings"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/service"
)

func (h *Handler) pipelineWebhook(response http.ResponseWriter, request *http.Request) {
	var payload webhookRequest
	if err := h.decodeJSON(response, request, &payload); err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	fieldErrors := make(map[string]string)
	if strings.TrimSpace(payload.PipelineName) == "" {
		fieldErrors["pipelineName"] = "must not be blank"
	}
	if strings.TrimSpace(payload.Status) == "" {
		fieldErrors["status"] = "must not be blank"
	}
	if payload.CITool == nil || strings.TrimSpace(*payload.CITool) == "" {
		fieldErrors["ciTool"] = "must not be blank"
	}
	if payload.BuildNumber == nil {
		fieldErrors["buildNumber"] = "is required"
	} else if *payload.BuildNumber < 0 || (*payload.BuildNumber == 0 && (payload.CITool == nil || !strings.EqualFold(strings.TrimSpace(*payload.CITool), "KUBERNETES_JOB"))) {
		fieldErrors["buildNumber"] = "must be greater than zero (or zero for Kubernetes Jobs)"
	}
	if len(fieldErrors) > 0 {
		h.writeError(response, http.StatusBadRequest, "Validation failed", fieldErrors)
		return
	}
	build, err := h.pipelines.RecordBuild(request.Context(), service.WebhookInput{
		PipelineName: payload.PipelineName, BuildNumber: payload.BuildNumber, CITool: payload.CITool,
		Status: payload.Status, GitCommit: payload.GitCommit, GitBranch: payload.GitBranch,
		CommitMessage: payload.CommitMessage, Author: payload.Author, DurationSeconds: payload.DurationSeconds,
		TestsPassed: payload.TestsPassed, TestsFailed: payload.TestsFailed,
		VulnerabilitiesDetected: payload.VulnerabilitiesDetected, Environment: payload.Environment,
		LogSnippet: payload.LogSnippet, BuildURL: payload.BuildURL,
		ExternalBuildID: payload.ExternalBuildID,
	})
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusCreated, pipelineDTO(build))
}

func (h *Handler) pipelineBuilds(response http.ResponseWriter, request *http.Request) {
	page, size, err := parsePagination(request)
	if err != nil {
		h.writeError(response, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.pipelines.Builds(request.Context(), page, size)
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	content := make([]pipelineBuildResponse, 0, len(result.Content))
	for _, build := range result.Content {
		content = append(content, pipelineDTO(build))
	}
	h.writeJSON(response, http.StatusOK, pageDTO(content, result.TotalElements, result.Page, result.Size))
}

func (h *Handler) doraMetrics(response http.ResponseWriter, request *http.Request) {
	metrics, err := h.pipelines.Metrics(request.Context())
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, doraDTO(metrics))
}

func (h *Handler) pipelineSync(response http.ResponseWriter, request *http.Request) {
	if request.ContentLength != 0 {
		var ignored map[string]any
		if err := h.decodeJSON(response, request, &ignored); err != nil {
			h.writeError(response, http.StatusBadRequest, err.Error(), nil)
			return
		}
	}
	metrics, err := h.pipelines.Metrics(request.Context())
	if err != nil {
		h.writeServiceError(response, err)
		return
	}
	h.writeJSON(response, http.StatusOK, map[string]any{
		"message":     "Stored CI/CD pipeline telemetry refreshed",
		"syncedAt":    localTime(h.clock()),
		"status":      "AVAILABLE",
		"doraMetrics": doraDTO(metrics),
	})
}
