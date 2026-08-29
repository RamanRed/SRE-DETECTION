package com.devops.incident.repository;

import com.devops.incident.model.PipelineBuild;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.time.LocalDateTime;
import java.util.List;

@Repository
public interface PipelineBuildRepository extends JpaRepository<PipelineBuild, String> {

    List<PipelineBuild> findByPipelineNameOrderByCreatedAtDesc(String pipelineName);

    Page<PipelineBuild> findAllByOrderByCreatedAtDesc(Pageable pageable);

    long countByStatus(String status);

    long countByCreatedAtAfter(LocalDateTime date);

    @Query("SELECT COUNT(b) FROM PipelineBuild b WHERE b.status = 'SUCCESS' AND b.createdAt >= :after")
    long countSuccessfulBuildsAfter(LocalDateTime after);

    @Query("SELECT COUNT(b) FROM PipelineBuild b WHERE b.status = 'FAILURE' AND b.createdAt >= :after")
    long countFailedBuildsAfter(LocalDateTime after);

    @Query("SELECT AVG(b.durationSeconds) FROM PipelineBuild b WHERE b.createdAt >= :after")
    Double getAverageDurationSecondsAfter(LocalDateTime after);
}
