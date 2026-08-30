# SRE Copilot Platform architecture and syllabus

**Document version:** 2.0.0

**Author and lead:** RamanRed

**Repository:** RamanRed/SRE-DETECTION

This document describes the implemented Go architecture. It deliberately does
not contain credentials. See [credential rotation](CREDENTIAL_ROTATION.md)
before connecting an external account.

## 1. End-to-end architecture

~~~mermaid
flowchart LR
    User[SRE / developer] --> UI[React dashboard<br/>Nginx :80]
    UI -->|REST /api/v1| Core[Incident service<br/>Go :8081]
    Core --> DB[(PostgreSQL)]
    Core -->|gRPC :9090| AI[AI copilot service<br/>Go]
    AI -->|optional HTTPS| LLM[Groq / OpenAI-compatible API]
    Core --> Scheduler[Polling scheduler]
    Scheduler --> Repo[GitHub / GitLab / Bitbucket]
    Scheduler --> CI[Jenkins / GitHub Actions / Kubernetes Job]
    Core --> Metrics[Prometheus endpoints]
~~~

The two former Spring Boot services are now Go binaries. The browser frontend
remains React/TypeScript and is served by Nginx, so it does not carry a JVM
memory cost. The REST, management, and gRPC paths remain compatible with the
original deployment.

### Runtime ports

| Component | Port | Protocol |
|:--|:--|:--|
| Frontend | 80 | HTTP through Nginx/Ingress |
| Incident service | 8081 | REST, health, Prometheus |
| AI copilot | 9090 | gRPC |
| AI management | 8082 | Health and Prometheus |
| PostgreSQL | 5432 | PostgreSQL wire protocol |

## 2. Connection flow

The Integrations control in the persistent header opens a write-only
credential form. It accepts:

| Area | Fields |
|:--|:--|
| Repository | Provider, HTTPS URL, target branch, read token |
| CI/CD | Engine, base URL, user, token, job/workflow/template |
| Scheduler | 5 minutes, 15 minutes, 1 hour, or daily |
| Automation | Rebuild-on-change and AI-triage-on-failure switches |

The browser sends the form to POST /api/v1/integrations/connect. The incident
service validates both remote connections with bounded HTTP clients before it
marks the integration connected. Stored tokens are encrypted with AES-256-GCM
using INTEGRATION_ENCRYPTION_KEY. Responses contain only token-configured
booleans, never secret values.

Only absolute HTTP(S) integration URLs are accepted, embedded credentials are
rejected, redirects are not followed implicitly, response sizes are bounded,
and connect/request deadlines are enforced. Private network endpoints are
disabled by default and require an explicit deployment setting because they
expand the trusted network surface.

Status shown in the header and modal is derived from server state:

- CONNECTED: both endpoints validated and polling is scheduled.
- SYNCING: a worker holds the current polling lease.
- ERROR: the last validation or polling cycle failed.
- DISCONNECTED: no valid connection is active.

## 3. Autonomous scheduler and failure ingestion

~~~mermaid
sequenceDiagram
    autonumber
    participant W as Go scheduler
    participant R as Repository API
    participant C as CI/CD API
    participant D as PostgreSQL
    participant A as AI copilot

    W->>D: Claim due integrations with a lease
    W->>R: Read latest branch commit
    W->>D: Insert commit fingerprint
    alt fingerprint is new and auto rebuild is enabled
        W->>C: Trigger configured pipeline
        loop bounded until completion
            W->>C: Poll build status
        end
        W->>D: Record build telemetry
        alt build failed
            W->>C: Fetch bounded failure logs
            W->>W: Extract repository-relative source paths
            W->>R: Fetch bounded source snippets at failed SHA
            W->>D: Create incident
            W->>A: Analyze logs plus source evidence over gRPC
            A-->>W: RCA, diff, verification, rollback, recovery action
            W->>D: Persist analysis and incident status
        end
    end
    W->>D: Complete lease and schedule next poll
~~~

Database leases and unique commit fingerprints prevent two replicas from
processing the same commit concurrently. External responses, logs, file counts,
and source sizes are bounded so an integration cannot consume unbounded memory.
The scheduler is active only for saved, connected records.

The AI request contract is additive and preserves the original protobuf field
numbers. Repository provider, URL, branch, commit metadata, CI provider, build
URL, and source snippets occupy new fields. Generated Go bindings are committed
and CI verifies that regeneration is clean and that pull requests do not break
the wire contract.

## 4. Code-aware AI triage and remediation

Source code, logs, commit messages, and CI metadata are treated as untrusted
evidence in the prompt. Provider output must be structured JSON. The Go service
validates and bounds every returned field. A unified diff is accepted only when
it targets a source path that was actually supplied to the model; heuristic
mode returns no fabricated patch when source evidence is insufficient.

The triage result contains:

- root cause and immediate mitigation;
- normalized severity and confidence;
- affected components and cited source paths;
- an optional unified diff;
- explicit verification and rollback plans.

Recovery scripts always require human approval. Approval is deliberately a
record-only state transition: the service stores `APPROVED`, keeps the incident
open, and never executes arbitrary model-generated shell text or claims that a
service recovered. An operator must run the reviewed action through a
controlled runbook/executor and verify it before a future typed completion
workflow may mark it `APPLIED`/`RESOLVED`. Generated Kubernetes actions use the
sre-copilot namespace and the platform's 64 Mi request / 128 Mi limit. Generic
node recovery targets the single-node k3s server service and avoids broad
Docker cleanup.

## 5. EC2 and deployment stability

The previous memory failure combined K3s, PostgreSQL, two JVMs, and the
frontend on a small host. The current design applies several independent
controls:

- both backend JVMs were replaced by statically linked Go services;
- each Go pod requests 64 Mi and is limited to 128 Mi;
- GOMEMLIMIT is set to 96MiB;
- Ollama is opt-in and is not suitable for the default 2 GiB node;
- Terraform, Ansible, and the bootstrap script configure 2 GiB swap;
- bootstrap verifies cgroup-v2 swap accounting;
- PostgreSQL uses persistent storage, probes, and a Recreate strategy;
- K3s keeps its bundled Traefik controller enabled;
- Jenkins publishes full-commit tags and renders registry content digests once
  before rollout;
- the deployment apply uses bounded timeouts and skips client-side OpenAPI
  validation in the constrained Jenkins environment.

Terraform does not expose Jenkins, application internals, or observability
administration publicly by default. SSH/K3s administration and HTTP ingress
use separate non-global CIDR allowlists.

## 6. Delivery and observability

GitHub Actions runs secret scanning, protobuf lint/compatibility/generation,
Go format/vet/race tests, frontend lint/build, container builds, Compose
validation, and the end-to-end smoke test on every branch.

Jenkins runs the same quality gates plus SonarQube analysis and Trivy image
scanning. Feature branches cannot publish latest images or deploy. Publishing
and the production telemetry webhook are restricted to main/master, and
deployment requires an approval gate. Registry and deployment credentials use
temporary directories that are removed after the stage.

Both Go services emit structured JSON logs and Prometheus-format metrics.
Prometheus alert rules and a Logstash parser are included. Prometheus,
Grafana, Elasticsearch, Kibana, and Alertmanager themselves are deployment
extensions rather than hidden dependencies of the default Compose stack.
Distributed tracing is also an extension; the current AI inference latency
signal is a Prometheus histogram, not a trace.

## 7. Course syllabus mapping

| Unit | Topic | Implementation evidence |
|:--|:--|:--|
| I | DevOps foundations, CAMS, Three Ways | Shared incident workflow, automation, DORA/Prometheus measurement, post-mortem template |
| I | Git lifecycle and GitHub Actions | Git metadata in pipeline builds and .github/workflows/ci.yml |
| II | Infrastructure as Code | terraform/ network, EC2 and optional RDS resources |
| II | Configuration management | ansible/playbook.yml and scripts/bootstrap_node.sh |
| II | Containers and microservices | Three multi-stage Dockerfiles, Compose, REST plus gRPC |
| III | Pipeline as Code | Jenkinsfile quality, security, publish, approval, rollout, telemetry stages |
| III | DevSecOps | Gitleaks, SonarQube, Trivy, write-only/encrypted integration credentials |
| III | Kubernetes | Kustomize, Deployments, Services, Ingress, probes, resource/security contexts |
| IV | Metrics and logs | Prometheus handlers/rules and Logstash JSON parsing |
| IV | DORA | Persisted pipeline events and live dashboard calculations |
| IV | SRE incident management | AI triage, human approval, verification/rollback plans, dashboard lifecycle |
| IV | Resilience learning | Non-destructive E2E smoke, isolated outage simulation, blameless post-mortem template |

## 8. Production boundary

Local Compose uses explicit demo mode. The Kubernetes path disables demo mode,
requires a bootstrap password, signs expiring HMAC sessions with an independent
secret, enforces SRE/DevOps/Evaluator roles, protects data routes, and gives the
CI webhook a separate bearer token. This is a controlled single-team bootstrap
boundary, not federated identity: add TLS, an organizational OIDC provider,
central audit retention, and a managed secret store before public multi-user
exposure. Never expose the integration form or Kubernetes control endpoints
directly to the public Internet.
