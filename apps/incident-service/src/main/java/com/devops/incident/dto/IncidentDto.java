package com.devops.incident.dto;

import com.devops.incident.model.Incident;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.LocalDateTime;
import java.util.List;

// ─────────────────────────────────────────────────
// Request DTOs
// ─────────────────────────────────────────────────

public class IncidentDto {

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class CreateRequest {
        @NotBlank(message = "Title is required")
        @Size(max = 255, message = "Title must not exceed 255 characters")
        private String title;

        @NotBlank(message = "Service name is required")
        @Size(max = 100, message = "Service name must not exceed 100 characters")
        private String serviceName;

        private String rawLogs;

        private String firingRule;

        @Builder.Default
        private String environment = "production";

        private String createdBy;
    }

    // ─────────────────────────────────────────────────
    // Response DTOs
    // ─────────────────────────────────────────────────

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class Response {
        private String id;
        private String title;
        private String serviceName;
        private String firingRule;
        private String environment;
        private String status;
        private String severity;
        private String createdBy;
        private LocalDateTime createdAt;
        private LocalDateTime updatedAt;
        private LocalDateTime resolvedAt;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class TriageResponse {
        private String incidentId;
        private String rootCause;
        private String immediateMitigation;
        private String confidenceScore;
        private String severity;
        private List<String> affectedComponents;
        private String incidentStatus;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RemediationResponse {
        private String remediationId;
        private String incidentId;
        private String scriptType;
        private String executableScript;
        private boolean requiresManualApproval;
        private String executionStatus;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class ApproveRequest {
        @NotBlank(message = "Applied-by field is required")
        private String appliedBy;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class DashboardStats {
        private long openIncidents;
        private long analyzingIncidents;
        private long resolvedToday;
        private long pendingRemediations;
        private long appliedRemediations;
    }

    // ─────────────────────────────────────────────────
    // Mapper helpers
    // ─────────────────────────────────────────────────

    public static Response toResponse(Incident incident) {
        return Response.builder()
                .id(incident.getId())
                .title(incident.getTitle())
                .serviceName(incident.getServiceName())
                .firingRule(incident.getFiringRule())
                .environment(incident.getEnvironment())
                .status(incident.getStatus().name())
                .severity(incident.getSeverity() != null ? incident.getSeverity().name() : null)
                .createdBy(incident.getCreatedBy())
                .createdAt(incident.getCreatedAt())
                .updatedAt(incident.getUpdatedAt())
                .resolvedAt(incident.getResolvedAt())
                .build();
    }
}
