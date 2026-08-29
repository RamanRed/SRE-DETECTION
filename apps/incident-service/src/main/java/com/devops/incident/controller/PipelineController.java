package com.devops.incident.controller;

import com.devops.incident.dto.PipelineDto;
import com.devops.incident.service.PipelineService;
import jakarta.validation.Valid;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

@RestController
@RequestMapping("/api/v1/ci")
@CrossOrigin(origins = "*")
public class PipelineController {

    private static final Logger log = LoggerFactory.getLogger(PipelineController.class);

    private final PipelineService pipelineService;

    @Autowired
    public PipelineController(PipelineService pipelineService) {
        this.pipelineService = pipelineService;
    }

    /**
     * Webhook endpoint for Jenkins, GitHub Actions, and GitLab CI to push build status.
     * POST /api/v1/ci/webhook
     */
    @PostMapping("/webhook")
    public ResponseEntity<PipelineDto.BuildResponse> receiveWebhook(
            @Valid @RequestBody PipelineDto.WebhookPayload payload) {
        log.info("REST: Received CI/CD Webhook for pipeline '{}' build #{}",
                payload.getPipelineName(), payload.getBuildNumber());
        PipelineDto.BuildResponse response = pipelineService.recordBuild(payload);
        return ResponseEntity.status(HttpStatus.CREATED).body(response);
    }

    /**
     * Get paginated list of recent CI/CD pipeline builds.
     * GET /api/v1/ci/builds?page=0&size=20
     */
    @GetMapping("/builds")
    public ResponseEntity<Page<PipelineDto.BuildResponse>> getBuilds(
            @RequestParam(defaultValue = "0") int page,
            @RequestParam(defaultValue = "20") int size) {
        return ResponseEntity.ok(pipelineService.getRecentBuilds(page, size));
    }

    /**
     * Get real-time DORA Metrics (Deployment Frequency, Lead Time, Change Failure Rate, MTTR).
     * GET /api/v1/ci/metrics/dora
     */
    @GetMapping("/metrics/dora")
    public ResponseEntity<PipelineDto.DoraMetricsResponse> getDoraMetrics() {
        return ResponseEntity.ok(pipelineService.getDoraMetrics());
    }

    /**
     * Trigger manual / simulated pipeline sync for evaluation.
     * POST /api/v1/ci/sync
     */
    @PostMapping("/sync")
    public ResponseEntity<Map<String, Object>> syncPipeline(
            @RequestBody(required = false) PipelineDto.CiSyncRequest request) {
        log.info("REST: Triggered CI/CD Pipeline Telemetry Sync");
        PipelineDto.DoraMetricsResponse metrics = pipelineService.getDoraMetrics();
        return ResponseEntity.ok(Map.of(
                "message", "CI/CD Pipeline Telemetry Synced Successfully",
                "syncedAt", java.time.LocalDateTime.now().toString(),
                "status", "CONNECTED",
                "doraMetrics", metrics
        ));
    }
}
