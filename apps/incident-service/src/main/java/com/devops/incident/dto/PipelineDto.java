package com.devops.incident.dto;

import com.devops.incident.model.PipelineBuild;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.LocalDateTime;
import java.util.List;

public class PipelineDto {

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class WebhookPayload {
        @NotBlank
        private String pipelineName;
        private Integer buildNumber;
        private String ciTool; // JENKINS, GITHUB_ACTIONS, GITLAB_CI
        @NotBlank
        private String status; // SUCCESS, FAILURE, RUNNING, UNSTABLE
        private String gitCommit;
        private String gitBranch;
        private String commitMessage;
        private String author;
        private Integer durationSeconds;
        private Integer testsPassed;
        private Integer testsFailed;
        private Integer vulnerabilitiesDetected;
        private String environment;
        private String logSnippet;
        private String buildUrl;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class BuildResponse {
        private String id;
        private String pipelineName;
        private Integer buildNumber;
        private String ciTool;
        private String status;
        private String gitCommit;
        private String gitBranch;
        private String commitMessage;
        private String author;
        private Integer durationSeconds;
        private Integer testsPassed;
        private Integer testsFailed;
        private Integer vulnerabilitiesDetected;
        private String environment;
        private String logSnippet;
        private String buildUrl;
        private LocalDateTime createdAt;
        private LocalDateTime updatedAt;

        public static BuildResponse fromEntity(PipelineBuild entity) {
            return BuildResponse.builder()
                    .id(entity.getId())
                    .pipelineName(entity.getPipelineName())
                    .buildNumber(entity.getBuildNumber())
                    .ciTool(entity.getCiTool())
                    .status(entity.getStatus())
                    .gitCommit(entity.getGitCommit())
                    .gitBranch(entity.getGitBranch())
                    .commitMessage(entity.getCommitMessage())
                    .author(entity.getAuthor())
                    .durationSeconds(entity.getDurationSeconds())
                    .testsPassed(entity.getTestsPassed())
                    .testsFailed(entity.getTestsFailed())
                    .vulnerabilitiesDetected(entity.getVulnerabilitiesDetected())
                    .environment(entity.getEnvironment())
                    .logSnippet(entity.getLogSnippet())
                    .buildUrl(entity.getBuildUrl())
                    .createdAt(entity.getCreatedAt())
                    .updatedAt(entity.getUpdatedAt())
                    .build();
        }
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class DoraMetricsResponse {
        private String deploymentFrequency; // e.g. "3.5 deploys/day"
        private String leadTimeForChanges;   // e.g. "4m 12s"
        private Double changeFailureRate;    // e.g. 5.2%
        private String meanTimeToRecovery;   // e.g. "12m 45s"
        private Long totalBuilds;
        private Long successfulBuilds;
        private Long failedBuilds;
        private List<BuildResponse> recentBuilds;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class CiSyncRequest {
        private String jenkinsUrl;
        private String jobName;
        private String username;
        private String apiToken;
    }
}
