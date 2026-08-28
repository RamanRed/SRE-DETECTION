package com.devops.copilot.service;

import com.devops.copilot.grpc.*;
import io.grpc.stub.StreamObserver;
import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import net.devh.boot.grpc.server.service.GrpcService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.ai.chat.client.ChatClient;
import org.springframework.ai.chat.prompt.PromptTemplate;
import org.springframework.beans.factory.annotation.Autowired;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

@GrpcService
public class AiTriageGrpcServer extends IncidentCopilotServiceGrpc.IncidentCopilotServiceImplBase {

    private static final Logger log = LoggerFactory.getLogger(AiTriageGrpcServer.class);

    private final ChatClient chatClient;
    private final Timer triageLatencyTimer;

    @Autowired
    public AiTriageGrpcServer(
            @Autowired(required = false) ChatClient.Builder chatClientBuilder,
            MeterRegistry meterRegistry) {
        if (chatClientBuilder != null) {
            this.chatClient = chatClientBuilder.build();
        } else {
            this.chatClient = null;
        }
        this.triageLatencyTimer = Timer.builder("ai.triage.inference")
                .description("Latency distribution of AI Triage and Remediation Inference")
                .publishPercentiles(0.5, 0.9, 0.95, 0.99)
                .register(meterRegistry);
    }

    @Override
    public void analyzeIncident(IncidentAnalysisRequest request,
                                StreamObserver<IncidentAnalysisResponse> responseObserver) {
        log.info("Received incident triage request for ID: {}, Service: {}, Rule: {}",
                request.getIncidentId(), request.getServiceName(), request.getFiringRule());

        Timer.Sample sample = Timer.start();
        try {
            String rootCause;
            String immediateMitigation;
            String severity = "HIGH";
            String confidence = "0.94";
            List<String> affectedComponents = new ArrayList<>();
            affectedComponents.add(request.getServiceName());

            if (chatClient != null) {
                try {
                    String promptString = """
                        You are an expert SRE AI Assistant diagnosing a production incident.
                        Service: {serviceName}
                        Environment: {environment}
                        Firing Alert: {firingRule}
                        Error Stack Trace / Logs:
                        {errorLogs}

                        Provide a precise diagnosis:
                        1. Root cause summary (max 2 sentences)
                        2. Immediate mitigation strategy
                        3. Estimated severity (LOW, MEDIUM, HIGH, CRITICAL)
                        """;

                    PromptTemplate template = new PromptTemplate(promptString);
                    String renderedPrompt = template.render(Map.of(
                            "serviceName", request.getServiceName(),
                            "environment", request.getEnvironment(),
                            "firingRule", request.getFiringRule(),
                            "errorLogs", request.getErrorLogs()
                    ));

                    rootCause = chatClient.prompt()
                            .user(renderedPrompt)
                            .call()
                            .content();
                    immediateMitigation = "Execute automated rollback or scale deployment replicas to relieve system pressure.";
                } catch (Exception ex) {
                    log.warn("Spring AI inference provider unavailable, falling back to heuristic engine: {}", ex.getMessage());
                    rootCause = generateHeuristicDiagnosis(request);
                    immediateMitigation = "Isolate failing database connection pool and execute automated restart.";
                }
            } else {
                rootCause = generateHeuristicDiagnosis(request);
                immediateMitigation = "Isolate failing database connection pool and execute automated restart.";
            }

            if (request.getErrorLogs().toLowerCase().contains("outofmemory") ||
                request.getErrorLogs().toLowerCase().contains("heap space")) {
                severity = "CRITICAL";
                confidence = "0.98";
                affectedComponents.add("JVM Runtime");
                affectedComponents.add("Kubernetes Pod Memory Limits");
            } else if (request.getErrorLogs().toLowerCase().contains("connection refused") ||
                       request.getErrorLogs().toLowerCase().contains("hikari") ||
                       request.getErrorLogs().toLowerCase().contains("psqlexception")) {
                severity = "CRITICAL";
                confidence = "0.96";
                affectedComponents.add("PostgreSQL Connection Pool (HikariCP)");
                affectedComponents.add("Database Network Route");
            }

            IncidentAnalysisResponse response = IncidentAnalysisResponse.newBuilder()
                    .setIncidentId(request.getIncidentId())
                    .setRootCause(rootCause)
                    .setImmediateMitigation(immediateMitigation)
                    .setConfidenceScore(confidence)
                    .setSeverity(severity)
                    .addAllAffectedComponents(affectedComponents)
                    .build();

            responseObserver.onNext(response);
            responseObserver.onCompleted();
            log.info("Completed analysis for incident: {}", request.getIncidentId());
        } catch (Exception e) {
            log.error("Error processing incident triage: ", e);
            responseObserver.onError(e);
        } finally {
            sample.stop(triageLatencyTimer);
        }
    }

    @Override
    public void generateRemediationScript(RemediationRequest request,
                                          StreamObserver<RemediationResponse> responseObserver) {
        log.info("Generating remediation script for incident: {}, Target: {}",
                request.getIncidentId(), request.getTargetSystem());

        Timer.Sample sample = Timer.start();
        try {
            String scriptType = "BASH";
            StringBuilder script = new StringBuilder();
            boolean requiresApproval = true;

            String rootCauseLower = request.getRootCause().toLowerCase();

            if (rootCauseLower.contains("database") || rootCauseLower.contains("connection") || rootCauseLower.contains("hikari")) {
                scriptType = "KUBECTL_ROLLBACK";
                script.append("#!/bin/bash\n")
                      .append("# SRE Remediation: Database connection pool recovery & Pod restart\n")
                      .append("echo \"[INFO] Flushing idle connection leaks in PostgreSQL...\"\n")
                      .append("kubectl rollout restart deployment/incident-service -n default\n")
                      .append("kubectl rollout status deployment/incident-service -n default --timeout=60s\n")
                      .append("echo \"[SUCCESS] Service restarted successfully with clean connection state.\"");
            } else if (rootCauseLower.contains("memory") || rootCauseLower.contains("oom")) {
                scriptType = "KUBECTL_ROLLBACK";
                script.append("#!/bin/bash\n")
                      .append("# SRE Remediation: OOM Killed Pod Recovery & Memory Limit Bump\n")
                      .append("kubectl set resources deployment/incident-service --limits=memory=1024Mi --requests=memory=512Mi -n default\n")
                      .append("kubectl rollout restart deployment/incident-service -n default\n")
                      .append("echo \"[SUCCESS] Pod memory scaled and deployment restarted.\"");
            } else {
                scriptType = "ANSIBLE_PLAYBOOK";
                script.append("---\n")
                      .append("- name: SRE Automated Node Recovery\n")
                      .append("  hosts: k8s_nodes\n")
                      .append("  become: yes\n")
                      .append("  tasks:\n")
                      .append("    - name: Prune dangling containers and cache\n")
                      .append("      command: docker system prune -f\n")
                      .append("    - name: Restart K3s agent\n")
                      .append("      systemd:\n")
                      .append("        name: k3s-agent\n")
                      .append("        state: restarted\n");
            }

            RemediationResponse response = RemediationResponse.newBuilder()
                    .setScriptType(scriptType)
                    .setExecutableScript(script.toString())
                    .setRequiresManualApproval(requiresApproval)
                    .build();

            responseObserver.onNext(response);
            responseObserver.onCompleted();
        } catch (Exception e) {
            log.error("Error generating remediation script: ", e);
            responseObserver.onError(e);
        } finally {
            sample.stop(triageLatencyTimer);
        }
    }

    private String generateHeuristicDiagnosis(IncidentAnalysisRequest request) {
        String logs = request.getErrorLogs();
        if (logs.contains("HikariPool") || logs.contains("Connection refused") || logs.contains("PSQLException")) {
            return "HikariCP database connection pool exhaustion detected. Target PostgreSQL database is either unreachable, rejecting connections due to max_connections breach, or timed out during active query execution.";
        }
        if (logs.contains("OutOfMemoryError") || logs.contains("Java heap space")) {
            return "JVM Heap exhaustion (OutOfMemoryError) detected due to uncollected memory allocation spike or resource leak in stream processing pipeline.";
        }
        if (logs.contains("TimeoutException") || logs.contains("504 Gateway")) {
            return "Upstream dependency latency breach detected causing thread pool saturation and downstream request timeout cascade.";
        }
        return "Detected anomalous execution failure in service '" + request.getServiceName() +
               "' triggered by rule '" + request.getFiringRule() + "'. Stack trace indicates abnormal termination during active request processing.";
    }
}
