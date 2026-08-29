package com.devops.incident.repository;

import com.devops.incident.model.PlatformIntegration;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface PlatformIntegrationRepository extends JpaRepository<PlatformIntegration, Long> {
    Optional<PlatformIntegration> findByUserId(String userId);
}
