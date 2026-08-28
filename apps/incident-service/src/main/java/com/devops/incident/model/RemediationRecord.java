package com.devops.incident.model;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.LocalDateTime;
import java.util.UUID;

/**
 * JPA Entity representing an AI-generated remediation record.
 * Maps to 'remediation_records' PostgreSQL table (Flyway V1 migration).
 */
@Entity
@Table(name = "remediation_records")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class RemediationRecord {

    @Id
    @Column(name = "id", length = 36, updatable = false, nullable = false)
    private String id;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "incident_id", nullable = false)
    private Incident incident;

    @Column(name = "ai_root_cause", columnDefinition = "TEXT")
    private String aiRootCause;

    @Column(name = "suggested_action", columnDefinition = "TEXT")
    private String suggestedAction;

    @Column(name = "script_type", length = 50)
    private String scriptType;

    @Column(name = "executable_script", columnDefinition = "TEXT")
    private String executableScript;

    @Column(name = "confidence_score", length = 10)
    private String confidenceScore;

    @Column(name = "affected_components", columnDefinition = "TEXT")
    private String affectedComponents;

    @Column(name = "requires_approval", nullable = false)
    @Builder.Default
    private Boolean requiresApproval = true;

    @Column(name = "applied_by", length = 100)
    private String appliedBy;

    @Enumerated(EnumType.STRING)
    @Column(name = "execution_status", nullable = false, length = 50)
    @Builder.Default
    private ExecutionStatus executionStatus = ExecutionStatus.PENDING;

    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @Column(name = "applied_at")
    private LocalDateTime appliedAt;

    @PrePersist
    protected void onCreate() {
        if (this.id == null || this.id.isBlank()) {
            this.id = UUID.randomUUID().toString();
        }
        if (this.createdAt == null) {
            this.createdAt = LocalDateTime.now();
        }
    }

    public enum ExecutionStatus {
        PENDING, APPROVED, EXECUTING, APPLIED, FAILED, REJECTED
    }
}
