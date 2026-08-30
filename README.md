# SRE Copilot Platform

An incident-triage and remediation platform with two low-memory Go services, a
React dashboard, PostgreSQL, gRPC, and Kubernetes deployment assets.

## Architecture

```text
React 19 dashboard -> Nginx/Ingress -> incident-service (Go REST :8081)
                                           |
                                           | gRPC / protobuf :9090
                                           v
                                    ai-copilot-service (Go)
                                           |
                              heuristic or OpenAI-compatible provider

incident-service <-> PostgreSQL 16
management/health/Prometheus: incident :8081, AI :8082
```

The REST and management paths remain compatible with the former Spring-based
deployment, so the frontend, reverse proxy, probes, and existing integrations
do not need a flag-day migration.

Detailed design and course material:

- [System architecture, connection flow, and syllabus](docs/ARCHITECTURE_AND_SYLLABUS.md)
- [Credential rotation and secure token setup](docs/CREDENTIAL_ROTATION.md)

## Stack

| Layer | Technology |
|:--|:--|
| Frontend | React 19, TypeScript, Vite, Nginx |
| Incident API | Go 1.25, PostgreSQL/pgx, REST, gRPC client |
| AI copilot | Go 1.25, gRPC server, deterministic heuristic, OpenAI-compatible HTTP client |
| Contract | Protocol Buffers with committed Go bindings |
| Database | PostgreSQL 16 |
| Runtime | Docker Compose or K3s/Kubernetes |
| Delivery | GitHub Actions, Jenkins, SonarCloud, Trivy |
| Operations | Prometheus rules/configuration and Logstash parsing |

## Local quick start

Prerequisites: Docker Engine 24+ with Compose v2.20+. The smoke scripts use
`curl` and Python 3. Node.js and Go are only needed when running services
directly.

```bash
docker compose up --build --wait
```

Open <http://localhost>. The default AI mode is deterministic and local: it
does not need an API key, a model download, or network egress.
Compose publishes its ports on loopback only, so the demo API and dashboard are
not exposed to other machines on the local network.

Run the non-destructive end-to-end smoke after the services are healthy:

```bash
bash scripts/e2e_smoke.sh
```

The smoke creates one uniquely named staging incident, triages it, generates a
remediation, records the human approval decision, verifies the truthful
execution state and dashboard statistics, and leaves all infrastructure
running. Approval does not execute arbitrary model-generated shell code.

### Optional cloud inference

Copy `.env.example` to `.env`, then set an OpenAI-compatible provider. Groq,
for example:

```dotenv
OPENAI_API_KEY=gsk_your_key
OPENAI_BASE_URL=https://api.groq.com/openai
OPENAI_MODEL=llama-3.3-70b-versatile
```

If the provider is unavailable during a request, the AI service falls back to
the deterministic heuristic instead of taking the incident workflow down.

### Optional local Ollama

Ollama is an opt-in Compose profile because a local model consumes much more
memory than both Go services combined.

```bash
OPENAI_API_KEY=ollama \
OPENAI_BASE_URL=http://ollama:11434/v1 \
OPENAI_MODEL=qwen2.5-coder:1.5b \
docker compose --profile ollama up --build --wait
```

The first start downloads the model. Budget roughly the `OLLAMA_MEMORY_LIMIT`
from `.env.example` in addition to the application containers. The default
3 GiB Ollama allowance does not fit on the 2 GiB `t3.small` deployment node;
use heuristic/cloud mode there or provide a larger node.

## Run without containers

Start PostgreSQL first, then run the three application processes:

```bash
docker compose up -d postgres

OPENAI_API_KEY=demo-key go run ./apps/ai-copilot-service/cmd/ai-copilot

DB_HOST=localhost \
AI_COPILOT_HOST=localhost \
go run ./apps/incident-service/cmd/incident-service

cd apps/frontend
npm ci
npm run dev
```

`demo-key`, an empty key, or a missing key selects heuristic AI mode.

## API compatibility

All frontend calls use the `/api/v1` prefix.

| Method | Path | Purpose |
|:--|:--|:--|
| POST | `/api/v1/incidents` | Create an incident |
| GET | `/api/v1/incidents?page=0&size=20` | Paginated incident list |
| GET | `/api/v1/incidents/active` | Open/analyzing incidents |
| GET | `/api/v1/incidents/{id}` | Incident detail |
| GET | `/api/v1/incidents/{id}/analysis` | Latest persisted code-aware analysis |
| POST | `/api/v1/incidents/{id}/triage` | Run gRPC triage |
| POST | `/api/v1/incidents/{id}/remediate` | Generate remediation |
| POST | `/api/v1/incidents/{id}/remediation/{rid}/approve` | Approve remediation |
| GET | `/api/v1/incidents/stats/dashboard` | Dashboard counters |
| POST | `/api/v1/ci/webhook` | Record CI telemetry |
| GET | `/api/v1/ci/builds` | Paginated CI builds |
| GET | `/api/v1/ci/metrics/dora` | DORA summary |
| POST | `/api/v1/ci/sync` | Refresh CI summary |
| POST | `/api/v1/auth/login` | Create a signed engineer session |
| GET | `/api/v1/auth/me` | Current signed session detail |
| GET/POST | `/api/v1/integrations/config` | Integration configuration |
| POST | `/api/v1/integrations/connect` | Validate and schedule a repository/CI connection |
| POST | `/api/v1/integrations/sync` | Synchronize integrations |

Errors use a stable JSON envelope with a top-level `message`, which is what the
frontend displays.

## Health and metrics

Compatibility endpoints are available on both backend management listeners:

- `/actuator/health`
- `/actuator/health/liveness`
- `/actuator/health/readiness`
- `/actuator/info`
- `/actuator/prometheus`

The incident service also keeps `/api/v1/incidents/health`. Its readiness
checks PostgreSQL; liveness is process-only so a database outage does not cause
a restart loop. The AI service exposes management HTTP on port 8082 and the
gRPC API on port 9090.

The repository contains Prometheus scrape/rule configuration and Logstash
parsing under `observability/`. It does not install Prometheus, Alertmanager,
Grafana, Elasticsearch, or their exporters; deploy those components separately
when using those configurations.

## Memory profile

The Kubernetes manifests start each Go backend with a 64 Mi request, a 128 Mi
limit, and `GOMEMLIMIT=96MiB`. Compose applies the same 128 Mi container limits.
These are initial guardrails, not a substitute for measuring production load.
PostgreSQL and K3s still require host headroom, so infrastructure defaults to a
2 GiB `t3.small`; a 1 GiB node remains tight during rolling updates. The EC2
bootstrap configures 2 GiB of swap, verifies cgroup-v2 swap accounting, and
runs K3s with swap support enabled. Swap is emergency rollout headroom, not a
replacement for adequate RAM.

## Quality checks

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test -race ./...

cd apps/frontend
npm ci
npm run lint
npm run build
```

These commands compile all packages and run the tests that are present. CI also
checks protobuf generation, builds all three container images, validates the
Compose model, starts the heuristic-mode stack, and runs the same E2E smoke.

## Kubernetes deployment

The manifests use a persistent in-cluster PostgreSQL volume. Create database
credentials once before the first PostgreSQL start; no placeholder database
password is embedded in the Kubernetes manifests:

The checked-in Ingress is deliberately HTTP-only, so keep it behind a trusted
network boundary until TLS is configured. The Kubernetes workload runs with
`DEMO_MODE=false`: it requires a bootstrap password, signs expiring sessions,
enforces server-side roles, authenticates CI webhooks separately, and encrypts
stored provider credentials. For a public multi-user deployment, replace the
single-team bootstrap login with the organization's identity provider.

```bash
kubectl apply --validate=false -f k8s/namespace-config.yml

: "${DB_USERNAME:?load DB_USERNAME from a secret manager}"
: "${DB_PASSWORD:?load DB_PASSWORD from a secret manager}"
: "${INTEGRATION_ENCRYPTION_KEY:?load a stable 32-byte key from a secret manager}"
: "${AUTH_SESSION_SECRET:?load a different random value of at least 32 bytes}"
: "${AUTH_BOOTSTRAP_PASSWORD:?load a bootstrap password of at least 12 bytes}"
: "${CI_WEBHOOK_TOKEN:?load a separate random CI webhook bearer token}"
secret_dir="$(mktemp -d)"
trap 'rm -rf "$secret_dir"' EXIT
umask 077
printf '%s' "$DB_USERNAME" >"$secret_dir/DB_USERNAME"
printf '%s' "$DB_PASSWORD" >"$secret_dir/DB_PASSWORD"
printf '%s' "$INTEGRATION_ENCRYPTION_KEY" >"$secret_dir/INTEGRATION_ENCRYPTION_KEY"
printf '%s' "$AUTH_SESSION_SECRET" >"$secret_dir/AUTH_SESSION_SECRET"
printf '%s' "$AUTH_BOOTSTRAP_PASSWORD" >"$secret_dir/AUTH_BOOTSTRAP_PASSWORD"
printf '%s' "$CI_WEBHOOK_TOKEN" >"$secret_dir/CI_WEBHOOK_TOKEN"
kubectl create secret generic sre-db-secret \
  --from-file=DB_USERNAME="$secret_dir/DB_USERNAME" \
  --from-file=DB_PASSWORD="$secret_dir/DB_PASSWORD" \
  -n sre-copilot --dry-run=client -o yaml | kubectl apply --validate=false -f -
kubectl create secret generic sre-integration-secret \
  --from-file=INTEGRATION_ENCRYPTION_KEY="$secret_dir/INTEGRATION_ENCRYPTION_KEY" \
  --from-file=AUTH_SESSION_SECRET="$secret_dir/AUTH_SESSION_SECRET" \
  --from-file=AUTH_BOOTSTRAP_PASSWORD="$secret_dir/AUTH_BOOTSTRAP_PASSWORD" \
  --from-file=CI_WEBHOOK_TOKEN="$secret_dir/CI_WEBHOOK_TOKEN" \
  -n sre-copilot --dry-run=client -o yaml | kubectl apply --validate=false -f -
rm -rf "$secret_dir"
trap - EXIT

kubectl apply --validate=false -k k8s/
kubectl rollout status deployment/sre-postgres -n sre-copilot
kubectl rollout status deployment/ai-copilot-service -n sre-copilot
kubectl rollout status deployment/incident-service -n sre-copilot
kubectl rollout status deployment/sre-frontend -n sre-copilot
```

Keep the database password and integration-encryption key stable. Updating the
Kubernetes Secret alone does not rotate the password inside an already
initialized PostgreSQL data directory. Rotating the session or webhook secret
is supported, but invalidates existing browser sessions or webhook clients.

To enable Groq/OpenAI-compatible inference, create the optional secret before
deploying the AI workload:

```bash
: "${OPENAI_API_KEY:?load OPENAI_API_KEY from a secret manager}"
printf '%s' "$OPENAI_API_KEY" | \
kubectl create secret generic sre-ai-secret \
  --from-file=OPENAI_API_KEY=/dev/stdin \
  -n sre-copilot --dry-run=client -o yaml | kubectl apply --validate=false -f -
```

No placeholder database, authentication, integration-encryption, webhook, or
AI Secret is committed, so a later `kubectl apply -k k8s/` cannot overwrite
runtime credentials. Keep the integration key stable; changing it makes
previously stored provider tokens undecryptable. Without the AI secret,
end-to-end triage uses deterministic heuristic mode.

Terraform provisions an optional RDS instance, while the checked-in Kubernetes
manifest deliberately uses its in-cluster PostgreSQL service. To use RDS,
override `DB_HOST`, credentials, and `DB_SSLMODE` in the incident deployment;
Terraform does not inject those values into Kubernetes automatically.

## AWS and K3s infrastructure

Terraform requires two non-global network allowlists: `allowed_cidr` for SSH
and the K3s API, and `public_ingress_cidrs` for Traefik ports 80/443. Direct
host access to the incident REST port and AI gRPC port is not opened. Optional
Prometheus/Grafana administration on ports 3000-3001 is disabled unless
`enable_observability_ingress=true`, and is then limited to `allowed_cidr`.

Provide `TF_VAR_db_password` through a secret manager or protected CI variable;
do not store it in a checked-in `.tfvars` file or pass it as a command-line
argument. Terraform state still contains sensitive database material, so use an
encrypted, access-controlled remote backend. The SSH public key can be loaded
from its `.pub` file into `TF_VAR_ec2_public_key`.

For example, after the two required `TF_VAR_*` values have been populated by
the deployment environment:

```bash
terraform -chdir=terraform plan \
  -var='allowed_cidr=203.0.113.10/32' \
  -var='public_ingress_cidrs=["203.0.113.0/24"]'
```

Replace the documentation-only CIDRs with the smallest real trusted ranges.
The bootstrap pins K3s `v1.36.2+k3s1`, leaves its bundled Traefik controller
enabled, protects kubeconfigs with mode `0600`, and verifies the configured
2 GiB swap is visible through cgroup v2.

## Fault simulation

The destructive chaos script pauses or scales down PostgreSQL, generates pgx
pool pressure, restores the database through an exit trap, then runs triage and
remediation:

```bash
bash scripts/simulate_outage.sh docker

INCIDENT_SERVICE_URL=http://your-ingress \
bash scripts/simulate_outage.sh kubernetes
```

Use `scripts/e2e_smoke.sh` for routine validation; reserve the outage script for
an isolated environment.
