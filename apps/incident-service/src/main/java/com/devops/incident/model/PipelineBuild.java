package com.devops.incident.model;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import org.hibernate.annotations.UpdateTimestamp;

import java.time.LocalDateTime;
import java.util.UUID;

/**
 * Entity representing a CI/CD Pipeline execution record from Jenkins, GitHub Actions, or GitLab CI.
 */
@Entity
@Table(name = "pipeline_builds")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class PipelineBuild {

    @Id
    @Column(name = "id", length = 36, updatable = false, nullable = false)
    private String id;

    @Column(name = "pipeline_name", nullable = false, length = 100)
    private String pipelineName;

    @Column(name = "build_number", nullable = false)
    private Integer buildNumber;

    @Column(name = "ci_tool", nullable = false, length = 50)
    @Builder.Default
    private String ciTool = "JENKINS";

    @Column(name = "status", nullable = false, length = 50)
    private String status; // SUCCESS, FAILURE, RUNNING, UNSTABLE

    @Column(name = "git_commit", length = 50)
    private String gitCommit;

    @Column(name = "git_branch", length = 100)
    private String gitBranch;

    @Column(name = "commit_message", length = 255)
    private String commitMessage;

    @Column(name = "author", length = 100)
    private String author;

    @Column(name = "duration_seconds")
    private Integer durationSeconds;

    @Column(name = "tests_passed")
    @Builder.Default
    private Integer testsPassed = 0;

    @Column(name = "tests_failed")
    @Builder.Default
    private Integer testsFailed = 0;

    @Column(name = "vulnerabilities_detected")
    @Builder.Default
    private Integer vulnerabilitiesDetected = 0;

    @Column(name = "environment", length = 50)
    @Builder.Default
    private String environment = "production";

    @Column(name = "log_snippet", columnDefinition = "TEXT")
    private String logSnippet;

    @Column(name = "build_url", length = 255)
    private String buildUrl;

    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @UpdateTimestamp
    @Column(name = "updated_at", nullable = false)
    private LocalDateTime updatedAt;

    @PrePersist
    public void prePersist() {
        if (id == null || id.isBlank()) {
            id = UUID.randomUUID().toString();
        }
        if (createdAt == null) {
            createdAt = LocalDateTime.now();
        }
    }
}
