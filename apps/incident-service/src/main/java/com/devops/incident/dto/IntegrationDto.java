package com.devops.incident.dto;

import com.devops.incident.model.PlatformIntegration;
import lombok.*;

import java.time.LocalDateTime;

public class IntegrationDto {

    @Getter
    @Setter
    @NoArgsConstructor
    @AllArgsConstructor
    @Builder
    public static class ConfigRequest {
        private String userId;
        private String username;
        private String githubToken;
        private String githubRepo; // e.g. "RamanRed/SRE-DETECTION"
        private String githubBranch; // e.g. "master"
        private String jenkinsUrl;
        private String jenkinsUsername;
        private String jenkinsApiToken;
        private String jenkinsJobName;
    }

    @Getter
    @Setter
    @NoArgsConstructor
    @AllArgsConstructor
    @Builder
    public static class ConfigResponse {
        private Long id;
        private String userId;
        private String username;
        private String githubRepo;
        private String githubBranch;
        private String githubStatus; // "CONNECTED", "DISCONNECTED", "ERROR"
        private boolean githubTokenConfigured;
        private String jenkinsUrl;
        private String jenkinsUsername;
        private String jenkinsJobName;
        private String jenkinsStatus; // "CONNECTED", "DISCONNECTED", "ERROR"
        private boolean jenkinsTokenConfigured;
        private LocalDateTime lastSyncTime;
        private String message;

        public static ConfigResponse fromEntity(PlatformIntegration entity) {
            if (entity == null) return null;
            return ConfigResponse.builder()
                    .id(entity.getId())
                    .userId(entity.getUserId())
                    .username(entity.getUsername())
                    .githubRepo(entity.getGithubRepo())
                    .githubBranch(entity.getGithubBranch() != null ? entity.getGithubBranch() : "master")
                    .githubStatus(entity.getGithubStatus() != null ? entity.getGithubStatus() : "DISCONNECTED")
                    .githubTokenConfigured(entity.getGithubToken() != null && !entity.getGithubToken().isBlank())
                    .jenkinsUrl(entity.getJenkinsUrl())
                    .jenkinsUsername(entity.getJenkinsUsername())
                    .jenkinsJobName(entity.getJenkinsJobName() != null ? entity.getJenkinsJobName() : "re-copilot-pipeline")
                    .jenkinsStatus(entity.getJenkinsStatus() != null ? entity.getJenkinsStatus() : "DISCONNECTED")
                    .jenkinsTokenConfigured(entity.getJenkinsApiToken() != null && !entity.getJenkinsApiToken().isBlank())
                    .lastSyncTime(entity.getLastSyncTime())
                    .build();
        }
    }

    @Getter
    @Setter
    @NoArgsConstructor
    @AllArgsConstructor
    @Builder
    public static class SyncResult {
        private boolean success;
        private String githubStatus;
        private String jenkinsStatus;
        private int commitsSynced;
        private int buildsSynced;
        private String message;
    }
}
