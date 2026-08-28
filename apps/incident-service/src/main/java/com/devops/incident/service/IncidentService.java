package com.devops.incident.service;

import com.devops.copilot.grpc.*;
import com.devops.incident.dto.IncidentDto;
import com.devops.incident.model.Incident;
import com.devops.incident.model.RemediationRecord;
import com.devops.incident.repository.IncidentRepository;
import com.devops.incident.repository.RemediationRecordRepository;
import net.devh.boot.grpc.client.inject.GrpcClient;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.http.HttpStatus;
import org.springframework.scheduling.annotation.Async;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.server.ResponseStatusException;

import java.time.LocalDateTime;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

@Service
@Transactional
public class IncidentService {

    private static final Logger log = LoggerFactory.getLogger(IncidentService.class);

    private final IncidentRepository incidentRepository;
    private final RemediationRecordRepository remediationRecordRepository;

    @GrpcClient("ai-copilot-service")
    private IncidentCopilotServiceGrpc.IncidentCopilotServiceBlockingStub copilotStub;

    @Autowired
    public IncidentService(IncidentRepository incidentRepository,
                           RemediationRecordRepository remediationRecordRepository) {
        this.incidentRepository = incidentRepository;
        this.remediationRecordRepository = remediationRecordRepository;
    }

    // ─────────────────────────────────────────────────────────────
    // Create new incident
    // ─────────────────────────────────────────────────────────────

    public IncidentDto.Response createIncident(IncidentDto.CreateRequest req) {
        log.info("Creating incident: title='{}', service='{}'", req.getTitle(), req.getServiceName());
        Incident incident = Incident.builder()
                .title(req.getTitle())
                .serviceName(req.getServiceName())
                .rawLogs(req.getRawLogs())
                .firingRule(req.getFiringRule())
                .environment(req.getEnvironment() != null ? req.getEnvironment() : "production")
                .status(Incident.IncidentStatus.OPEN)
                .createdBy(req.getCreatedBy() != null ? req.getCreatedBy() : "system")
                .build();

        Incident saved = incidentRepository.save(incident);
        log.info("Incident created with ID: {}", saved.getId());
        return IncidentDto.toResponse(saved);
    }

    // ─────────────────────────────────────────────────────────────
    // Fetch incidents
    // ─────────────────────────────────────────────────────────────

    @Transactional(readOnly = true)
    public Page<IncidentDto.Response> getAllIncidents(int page, int size) {
        return incidentRepository.findAll(PageRequest.of(page, size))
                .map(IncidentDto::toResponse);
    }

    @Transactional(readOnly = true)
    public IncidentDto.Response getIncidentById(String id) {
        return IncidentDto.toResponse(findIncidentOrThrow(id));
    }

    @Transactional(readOnly = true)
    public List<IncidentDto.Response> getActiveIncidents() {
        return incidentRepository.findAllActiveIncidents().stream()
                .map(IncidentDto::toResponse)
                .collect(Collectors.toList());
    }

    // ─────────────────────────────────────────────────────────────
    // AI Triage: calls ai-copilot-service via gRPC
    // ─────────────────────────────────────────────────────────────

    public IncidentDto.TriageResponse triggerAiTriage(String incidentId) {
        Incident incident = findIncidentOrThrow(incidentId);
        log.info("Triggering AI triage for incident: {} (service: {})", incidentId, incident.getServiceName());

        // Transition to ANALYZING
        incident.setStatus(Incident.IncidentStatus.ANALYZING);
        incidentRepository.save(incident);

        try {
            IncidentAnalysisRequest grpcRequest = IncidentAnalysisRequest.newBuilder()
                    .setIncidentId(incident.getId())
                    .setServiceName(incident.getServiceName())
                    .setErrorLogs(incident.getRawLogs() != null ? incident.getRawLogs() : "")
                    .setFiringRule(incident.getFiringRule() != null ? incident.getFiringRule() : "ManualTriage")
                    .setEnvironment(incident.getEnvironment())
                    .build();

            IncidentAnalysisResponse grpcResponse = copilotStub.analyzeIncident(grpcRequest);
            log.info("AI triage completed for incident: {} with severity: {}",
                    incidentId, grpcResponse.getSeverity());

            // Update incident severity post-triage
            try {
                incident.setSeverity(Incident.IncidentSeverity.valueOf(grpcResponse.getSeverity()));
            } catch (IllegalArgumentException ex) {
                log.warn("Unknown severity from AI engine: {}, defaulting to HIGH", grpcResponse.getSeverity());
                incident.setSeverity(Incident.IncidentSeverity.HIGH);
            }
            incidentRepository.save(incident);

            return IncidentDto.TriageResponse.builder()
                    .incidentId(grpcResponse.getIncidentId())
                    .rootCause(grpcResponse.getRootCause())
                    .immediateMitigation(grpcResponse.getImmediateMitigation())
                    .confidenceScore(grpcResponse.getConfidenceScore())
                    .severity(grpcResponse.getSeverity())
                    .affectedComponents(grpcResponse.getAffectedComponentsList())
                    .incidentStatus(incident.getStatus().name())
                    .build();

        } catch (Exception e) {
            log.error("AI triage gRPC call failed for incident {}: {}", incidentId, e.getMessage());
            incident.setStatus(Incident.IncidentStatus.OPEN);
            incidentRepository.save(incident);
            throw new ResponseStatusException(HttpStatus.SERVICE_UNAVAILABLE,
                    "AI Copilot service is unreachable. Ensure ai-copilot-service is running on port 9090.");
        }
    }

    // ─────────────────────────────────────────────────────────────
    // Remediation script generation via gRPC
    // ─────────────────────────────────────────────────────────────

    public IncidentDto.RemediationResponse generateRemediation(String incidentId) {
        Incident incident = findIncidentOrThrow(incidentId);

        // Pull the latest triage root cause from the most recent remediation record
        RemediationRecord existingRecord = remediationRecordRepository
                .findTopByIncidentIdOrderByCreatedAtDesc(incidentId)
                .orElse(null);
        String rootCause = existingRecord != null ? existingRecord.getAiRootCause() : "Unknown anomaly detected";

        log.info("Generating remediation script for incident: {} (rootCause: {})", incidentId, rootCause);

        RemediationRequest grpcRequest = RemediationRequest.newBuilder()
                .setIncidentId(incidentId)
                .setRootCause(rootCause)
                .setTargetSystem(incident.getServiceName())
                .build();

        try {
            com.devops.copilot.grpc.RemediationResponse grpcResponse = copilotStub.generateRemediationScript(grpcRequest);

            RemediationRecord record = RemediationRecord.builder()
                    .incident(incident)
                    .aiRootCause(rootCause)
                    .suggestedAction(grpcResponse.getExecutableScript())
                    .scriptType(grpcResponse.getScriptType())
                    .executableScript(grpcResponse.getExecutableScript())
                    .requiresApproval(grpcResponse.getRequiresManualApproval())
                    .executionStatus(RemediationRecord.ExecutionStatus.PENDING)
                    .build();

            RemediationRecord saved = remediationRecordRepository.save(record);
            log.info("Remediation record saved: {} for incident: {}", saved.getId(), incidentId);

            return IncidentDto.RemediationResponse.builder()
                    .remediationId(saved.getId())
                    .incidentId(incidentId)
                    .scriptType(grpcResponse.getScriptType())
                    .executableScript(grpcResponse.getExecutableScript())
                    .requiresManualApproval(grpcResponse.getRequiresManualApproval())
                    .executionStatus(saved.getExecutionStatus().name())
                    .build();

        } catch (Exception e) {
            log.error("Remediation generation failed for incident {}: {}", incidentId, e.getMessage());
            throw new ResponseStatusException(HttpStatus.SERVICE_UNAVAILABLE,
                    "AI Copilot service is unavailable. Cannot generate remediation script.");
        }
    }

    // ─────────────────────────────────────────────────────────────
    // Approve and apply remediation
    // ─────────────────────────────────────────────────────────────

    public IncidentDto.RemediationResponse approveRemediation(String incidentId,
                                                              String remediationId,
                                                              IncidentDto.ApproveRequest req) {
        RemediationRecord record = remediationRecordRepository.findById(remediationId)
                .orElseThrow(() -> new ResponseStatusException(HttpStatus.NOT_FOUND,
                        "Remediation record not found: " + remediationId));

        if (!record.getIncident().getId().equals(incidentId)) {
            throw new ResponseStatusException(HttpStatus.BAD_REQUEST,
                    "Remediation record does not belong to incident: " + incidentId);
        }

        record.setExecutionStatus(RemediationRecord.ExecutionStatus.APPROVED);
        record.setAppliedBy(req.getAppliedBy());
        record.setAppliedAt(LocalDateTime.now());
        remediationRecordRepository.save(record);

        // Also mark the incident as RESOLVED
        Incident incident = record.getIncident();
        incident.setStatus(Incident.IncidentStatus.RESOLVED);
        incident.setResolvedAt(LocalDateTime.now());
        incidentRepository.save(incident);

        log.info("Remediation {} approved by '{}' — incident {} marked RESOLVED",
                remediationId, req.getAppliedBy(), incidentId);

        return IncidentDto.RemediationResponse.builder()
                .remediationId(record.getId())
                .incidentId(incidentId)
                .scriptType(record.getScriptType())
                .executableScript(record.getExecutableScript())
                .requiresManualApproval(record.getRequiresApproval())
                .executionStatus(record.getExecutionStatus().name())
                .build();
    }

    // ─────────────────────────────────────────────────────────────
    // Dashboard stats
    // ─────────────────────────────────────────────────────────────

    @Transactional(readOnly = true)
    public IncidentDto.DashboardStats getDashboardStats() {
        return IncidentDto.DashboardStats.builder()
                .openIncidents(incidentRepository.countByStatus(Incident.IncidentStatus.OPEN))
                .analyzingIncidents(incidentRepository.countByStatus(Incident.IncidentStatus.ANALYZING))
                .resolvedToday(incidentRepository.findByDateRange(
                        LocalDateTime.now().withHour(0).withMinute(0).withSecond(0),
                        LocalDateTime.now()).stream()
                        .filter(i -> i.getStatus() == Incident.IncidentStatus.RESOLVED)
                        .count())
                .pendingRemediations(remediationRecordRepository.countByExecutionStatus(
                        RemediationRecord.ExecutionStatus.PENDING))
                .appliedRemediations(remediationRecordRepository.countByExecutionStatus(
                        RemediationRecord.ExecutionStatus.APPLIED))
                .build();
    }

    // ─────────────────────────────────────────────────────────────
    // Helper
    // ─────────────────────────────────────────────────────────────

    private Incident findIncidentOrThrow(String id) {
        return incidentRepository.findById(id)
                .orElseThrow(() -> new ResponseStatusException(
                        HttpStatus.NOT_FOUND, "Incident not found: " + id));
    }
}
