# SRE Copilot Go Cutover and Operations Notes

This document records the operational state after replacing the two Java
backends with Go. The root `README.md` is the primary runbook.

## What changed

- `incident-service` is a Go REST service with pgx/PostgreSQL and a gRPC client.
- `ai-copilot-service` is a Go gRPC service with a small management HTTP server.
- Existing REST, gRPC, JSON, health, and Prometheus paths remain compatible.
- Backend images are statically compiled (`CGO_ENABLED=0`), run as UID 10001,
  and retain Alpine CA certificates and `wget` for HTTPS and healthchecks.
- Each backend Kubernetes pod requests 64 Mi, is limited to 128 Mi, and uses
  `GOMEMLIMIT=96MiB`.
- Ollama is opt-in. The no-key default uses deterministic heuristic triage and
  is the lowest-memory way to run end to end.

## Memory model

| Workload | Request | Limit |
|:--|--:|--:|
| incident-service | 64 Mi | 128 Mi |
| ai-copilot-service | 64 Mi | 128 Mi |
| frontend | 64 Mi | 128 Mi |
| PostgreSQL | 128 Mi | 256 Mi |
| **Application total** | **320 Mi** | **640 Mi** |

K3s, the operating system, rollout surges, and build tools are outside those
limits. Keep the `t3.small` infrastructure default until workload measurements
demonstrate that a smaller node remains stable during deployments. Swap is
configured as emergency headroom, not normal application memory.

Ollama is intentionally omitted from this table. A local model can use several
GiB and will dominate the memory footprint.

## Deployment invariants

- DNS names and ports stay `incident-service:8081`,
  `ai-copilot-service:9090`, and AI management port `8082`.
- Kubernetes readiness uses `/actuator/health/readiness`; liveness uses
  `/actuator/health/liveness`.
- Incident readiness checks PostgreSQL. Liveness remains process-only.
- Prometheus scrapes `/actuator/prometheus`.
- PostgreSQL data is mounted from the `sre-postgres-data` PVC.
- No runtime database password, integration encryption key, or AI API key
  Secret is stored in Git. Create `sre-db-secret` and
  `sre-integration-secret` before deployment. A missing `sre-ai-secret` selects
  heuristic mode; Jenkins creates all three from protected credentials.
- PostgreSQL uses a `Recreate` update strategy, preventing two database pods
  from mounting the same single-writer data directory during an update.

## Jenkins flow

1. Checkout and derive a collision-resistant full-commit image tag from Git.
2. Run `gofmt`, `go vet`, race-enabled Go tests/coverage, and binary builds.
3. Lint, typecheck, and build the frontend.
4. Run SonarCloud analysis with Go coverage.
5. Build all images and fail on critical Trivy findings that have a published fix.
6. On `main`/`master` only, authenticate with a temporary Docker configuration
   directory, push full-commit and `latest` tags, and resolve their registry
   content digests.
7. After an explicit deployment approval on `main`/`master`, render a temporary
   Kustomize overlay containing the verified image digests, apply it once,
   wait for every rollout, and run in-cluster health/API smoke checks.

Registry authentication, the kubeconfig, and credential staging files are
removed by shell traps. The deploy helper uses a digest-pinned Kubernetes 1.36
kubectl image and host networking so the loopback server address in a local
K3s kubeconfig remains reachable. Therefore that Jenkins executor must run on
the Linux K3s node; a remote executor instead needs a restricted kubeconfig
whose API server address is routable from the container. Applying `k8s/`
cannot replace runtime database or AI keys because neither Secret is part of
the manifests. Temporary rendered manifests are removed by the same trap, and
the cluster never receives an intermediate `:latest` rollout. Deployment
telemetry is also emitted only for release branches.

Jenkins expects these credential IDs: `dockerhub-password` (username/password),
`sonarqube-token` (username/password with the token in the password field),
`k3s-kubeconfig` (secret file), `sre-db-credentials` (stable database
username/password), `sre-integration-encryption-key` (stable 32-byte key),
`sre-auth-session-secret` (at least 32 bytes),
`sre-auth-bootstrap-password` (at least 12 bytes), `sre-ci-webhook-token`
(independent random bearer token), and `groq-api-key` (secret text). Rotating the database credential also requires
changing the PostgreSQL role password in the existing database; replacing only
the Kubernetes Secret is not sufficient. Rotating the integration key requires
reconnecting stored integrations.

## Observability changes

The Prometheus rules now consume:

- `ai_triage_inference_seconds_bucket`
- `http_server_requests_total`
- `db_pool_waiting_requests`
- `db_pool_empty_acquire_total`
- cAdvisor `container_memory_working_set_bytes`
- cAdvisor `container_start_time_seconds`

The Logstash filter parses Go `slog` JSON (`time`, `level`, `msg`, `source`,
and `error`) rather than Spring logger/thread fields.

The repository provides configuration and RBAC, not complete installations of
Prometheus, Alertmanager, exporters, Grafana, Elasticsearch, or Logstash.

## Verification

For a normal application smoke:

```bash
docker compose up --build --wait
bash scripts/e2e_smoke.sh
```

For an isolated database-outage exercise:

```bash
bash scripts/simulate_outage.sh docker
```

The chaos script restores PostgreSQL from an exit trap. In Kubernetes, its PVC
keeps the data across the scale-to-zero interval.

## Known infrastructure choice

Terraform can provision RDS, but the default Kubernetes deployment uses the
in-cluster PostgreSQL service. RDS connection values must be supplied to the
incident deployment explicitly; Terraform does not update the manifest.

The infrastructure is intentionally tagged and documented as an internal
deployment. Its Ingress is HTTP-only. The API provides signed bootstrap
sessions, role checks, and a separate CI webhook token, but a public multi-user
service still needs TLS and an organizational identity provider. Terraform rejects global administrator and
Ingress CIDRs, opens only SSH/K3s API to the administrator range and Traefik
80/443 to an explicit client range, and leaves ports 3000-3001 closed unless
requested. Add TLS and federated identity before public use.

The node bootstrap pins K3s `v1.36.2+k3s1`, keeps bundled Traefik enabled,
configures 2 GiB swap, verifies cgroup-v2 swap accounting, and keeps all
kubeconfig copies at mode `0600`. Database passwords must come from protected
Terraform/Jenkins/Kubernetes secret inputs; protect Terraform state because it
contains the RDS password even when output values are marked sensitive.
