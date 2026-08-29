package com.devops.incident.model;

import jakarta.persistence.*;
import lombok.*;
import org.hibernate.annotations.CreationTimestamp;
import org.hibernate.annotations.UpdateTimestamp;

import java.time.LocalDateTime;

@Entity
@Table(name = "platform_integrations")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class PlatformIntegration {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "user_id", nullable = false, unique = true)
    private String userId;

    @Column(name = "username")
    private String username;

    @Column(name = "github_token", length = 512)
    private String githubToken;

    @Column(name = "github_repo")
    private String githubRepo; // e.g. "RamanRed/SRE-DETECTION"

    @Column(name = "github_branch")
    private String githubBranch; // e.g. "master"

    @Column(name = "github_status")
    private String githubStatus; // "CONNECTED", "DISCONNECTED", "ERROR"

    @Column(name = "jenkins_url")
    private String jenkinsUrl; // e.g. "http://16.16.175.206:8080"

    @Column(name = "jenkins_username")
    private String jenkinsUsername;

    @Column(name = "jenkins_api_token", length = 512)
    private String jenkinsApiToken;

    @Column(name = "jenkins_job_name")
    private String jenkinsJobName; // e.g. "re-copilot-pipeline"

    @Column(name = "jenkins_status")
    private String jenkinsStatus; // "CONNECTED", "DISCONNECTED", "ERROR"

    @Column(name = "last_sync_time")
    private LocalDateTime lastSyncTime;

    @CreationTimestamp
    @Column(name = "created_at", updatable = false)
    private LocalDateTime createdAt;

    @UpdateTimestamp
    @Column(name = "updated_at")
    private LocalDateTime updatedAt;
}
