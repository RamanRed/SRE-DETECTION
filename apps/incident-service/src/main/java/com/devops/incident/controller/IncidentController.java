package com.devops.incident.controller;

import com.devops.incident.dto.IncidentDto;
import com.devops.incident.service.IncidentService;
import jakarta.validation.Valid;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

@RestController
@RequestMapping("/api/v1/incidents")
@CrossOrigin(origins = "*") // Allow frontend requests (restrict in production via Ingress)
public class IncidentController {

    private static final Logger log = LoggerFactory.getLogger(IncidentController.class);

    private final IncidentService incidentService;

    @Autowired
    public IncidentController(IncidentService incidentService) {
        this.incidentService = incidentService;
    }

    // ─────────────────────────────────────────────────
    // POST /api/v1/incidents
    // Create a new incident
    // ─────────────────────────────────────────────────
    @PostMapping
    public ResponseEntity<IncidentDto.Response> createIncident(
            @Valid @RequestBody IncidentDto.CreateRequest request) {
        log.info("REST: Create incident for service '{}'", request.getServiceName());
        IncidentDto.Response response = incidentService.createIncident(request);
        return ResponseEntity.status(HttpStatus.CREATED).body(response);
    }

    // ─────────────────────────────────────────────────
    // GET /api/v1/incidents?page=0&size=20
    // Paginated list of all incidents
    // ─────────────────────────────────────────────────
    @GetMapping
    public ResponseEntity<Page<IncidentDto.Response>> getAllIncidents(
            @RequestParam(defaultValue = "0") int page,
            @RequestParam(defaultValue = "20") int size) {
        return ResponseEntity.ok(incidentService.getAllIncidents(page, size));
    }

    // ─────────────────────────────────────────────────
    // GET /api/v1/incidents/active
    // All OPEN or ANALYZING incidents for the dashboard
    // ─────────────────────────────────────────────────
    @GetMapping("/active")
    public ResponseEntity<List<IncidentDto.Response>> getActiveIncidents() {
        return ResponseEntity.ok(incidentService.getActiveIncidents());
    }

    // ─────────────────────────────────────────────────
    // GET /api/v1/incidents/{id}
    // Single incident by ID
    // ─────────────────────────────────────────────────
    @GetMapping("/{id}")
    public ResponseEntity<IncidentDto.Response> getIncidentById(@PathVariable String id) {
        return ResponseEntity.ok(incidentService.getIncidentById(id));
    }

    // ─────────────────────────────────────────────────
    // POST /api/v1/incidents/{id}/triage
    // Trigger AI root-cause analysis via gRPC
    // ─────────────────────────────────────────────────
    @PostMapping("/{id}/triage")
    public ResponseEntity<IncidentDto.TriageResponse> triggerTriage(@PathVariable String id) {
        log.info("REST: AI triage triggered for incident '{}'", id);
        return ResponseEntity.ok(incidentService.triggerAiTriage(id));
    }

    // ─────────────────────────────────────────────────
    // POST /api/v1/incidents/{id}/remediate
    // Generate AI remediation script via gRPC
    // ─────────────────────────────────────────────────
    @PostMapping("/{id}/remediate")
    public ResponseEntity<IncidentDto.RemediationResponse> generateRemediation(@PathVariable String id) {
        log.info("REST: Remediation script requested for incident '{}'", id);
        return ResponseEntity.ok(incidentService.generateRemediation(id));
    }

    // ─────────────────────────────────────────────────
    // POST /api/v1/incidents/{id}/remediation/{rid}/approve
    // SRE Engineer approves and applies remediation
    // ─────────────────────────────────────────────────
    @PostMapping("/{id}/remediation/{rid}/approve")
    public ResponseEntity<IncidentDto.RemediationResponse> approveRemediation(
            @PathVariable String id,
            @PathVariable String rid,
            @Valid @RequestBody IncidentDto.ApproveRequest request) {
        log.info("REST: Remediation '{}' approved for incident '{}' by '{}'",
                rid, id, request.getAppliedBy());
        return ResponseEntity.ok(incidentService.approveRemediation(id, rid, request));
    }

    // ─────────────────────────────────────────────────
    // GET /api/v1/incidents/stats/dashboard
    // Real-time summary stats for the SRE dashboard
    // ─────────────────────────────────────────────────
    @GetMapping("/stats/dashboard")
    public ResponseEntity<IncidentDto.DashboardStats> getDashboardStats() {
        return ResponseEntity.ok(incidentService.getDashboardStats());
    }

    // ─────────────────────────────────────────────────
    // GET /api/v1/incidents/health
    // Simple service health echo for integration tests
    // ─────────────────────────────────────────────────
    @GetMapping("/health")
    public ResponseEntity<Map<String, String>> health() {
        return ResponseEntity.ok(Map.of(
                "service", "incident-service",
                "status", "UP",
                "version", "1.0.0-SNAPSHOT"
        ));
    }
}
