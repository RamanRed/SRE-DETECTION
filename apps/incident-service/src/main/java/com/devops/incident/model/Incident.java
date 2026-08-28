package com.devops.incident.model;

import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import org.hibernate.annotations.UpdateTimestamp;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

/**
 * JPA Entity representing a production incident lifecycle record.
 * Maps to the 'incidents' PostgreSQL table provisioned by Flyway V1 migration.
 */
@Entity
@Table(name = "incidents")
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class Incident {

    @Id
    @Column(name = "id", length = 36, updatable = false, nullable = false)
    private String id;

    @Column(name = "title", nullable = false, length = 255)
    private String title;

    @Column(name = "service_name", nullable = false, length = 100)
    private String serviceName;

    @Column(name = "raw_logs", columnDefinition = "TEXT")
    private String rawLogs;

    @Column(name = "firing_rule", length = 255)
    private String firingRule;

    @Column(name = "environment", nullable = false, length = 50)
    @Builder.Default
    private String environment = "production";

    @Enumerated(EnumType.STRING)
    @Column(name = "status", nullable = false, length = 50)
    @Builder.Default
    private IncidentStatus status = IncidentStatus.OPEN;

    @Enumerated(EnumType.STRING)
    @Column(name = "severity", length = 50)
    private IncidentSeverity severity;

    @Column(name = "created_at", nullable = false, updatable = false)
    private LocalDateTime createdAt;

    @UpdateTimestamp
    @Column(name = "updated_at", nullable = false)
    private LocalDateTime updatedAt;

    @Column(name = "resolved_at")
    private LocalDateTime resolvedAt;

    @Column(name = "created_by", length = 100)
    @Builder.Default
    private String createdBy = "system";

    @OneToMany(mappedBy = "incident", cascade = CascadeType.ALL, fetch = FetchType.LAZY, orphanRemoval = true)
    @Builder.Default
    private List<RemediationRecord> remediationRecords = new ArrayList<>();

    @PrePersist
    protected void onCreate() {
        if (this.id == null || this.id.isBlank()) {
            this.id = UUID.randomUUID().toString();
        }
        if (this.createdAt == null) {
            this.createdAt = LocalDateTime.now();
        }
        this.updatedAt = LocalDateTime.now();
    }

    public enum IncidentStatus {
        OPEN, ANALYZING, RESOLVED, CLOSED
    }

    public enum IncidentSeverity {
        LOW, MEDIUM, HIGH, CRITICAL
    }
}
