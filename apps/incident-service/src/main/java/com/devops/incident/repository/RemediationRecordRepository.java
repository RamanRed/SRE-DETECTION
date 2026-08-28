package com.devops.incident.repository;

import com.devops.incident.model.RemediationRecord;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface RemediationRecordRepository extends JpaRepository<RemediationRecord, String> {

    List<RemediationRecord> findByIncidentIdOrderByCreatedAtDesc(String incidentId);

    Optional<RemediationRecord> findTopByIncidentIdOrderByCreatedAtDesc(String incidentId);

    @Query("SELECT r FROM RemediationRecord r WHERE r.incident.id = :incidentId AND r.executionStatus = :status")
    List<RemediationRecord> findByIncidentIdAndStatus(
            @Param("incidentId") String incidentId,
            @Param("status") RemediationRecord.ExecutionStatus status);

    long countByExecutionStatus(RemediationRecord.ExecutionStatus status);
}
