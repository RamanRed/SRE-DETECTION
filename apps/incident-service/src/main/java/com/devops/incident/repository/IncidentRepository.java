package com.devops.incident.repository;

import com.devops.incident.model.Incident;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.LocalDateTime;
import java.util.List;

@Repository
public interface IncidentRepository extends JpaRepository<Incident, String> {

    Page<Incident> findByStatusOrderByCreatedAtDesc(Incident.IncidentStatus status, Pageable pageable);

    Page<Incident> findByServiceNameContainingIgnoreCaseOrderByCreatedAtDesc(String serviceName, Pageable pageable);

    List<Incident> findBySeverityAndStatusOrderByCreatedAtDesc(Incident.IncidentSeverity severity, Incident.IncidentStatus status);

    @Query("SELECT i FROM Incident i WHERE i.createdAt BETWEEN :from AND :to ORDER BY i.createdAt DESC")
    List<Incident> findByDateRange(@Param("from") LocalDateTime from, @Param("to") LocalDateTime to);

    @Query("SELECT COUNT(i) FROM Incident i WHERE i.status = :status")
    long countByStatus(@Param("status") Incident.IncidentStatus status);

    @Query("SELECT i FROM Incident i WHERE i.status IN ('OPEN', 'ANALYZING') ORDER BY i.createdAt DESC")
    List<Incident> findAllActiveIncidents();
}
