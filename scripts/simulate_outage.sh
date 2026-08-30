#!/usr/bin/env bash
# SRE incident simulation: PostgreSQL unavailability and pgx pool pressure.
# Usage:
#   ./scripts/simulate_outage.sh docker
#   INCIDENT_SERVICE_URL=http://your-ingress ./scripts/simulate_outage.sh kubernetes

set -Eeuo pipefail

MODE="${1:-docker}"
FAULT_DURATION_SECONDS="${FAULT_DURATION_SECONDS:-30}"
if [[ -n "${INCIDENT_SERVICE_URL:-}" ]]; then
  SERVICE_URL="${INCIDENT_SERVICE_URL%/}"
elif [[ "$MODE" == "kubernetes" ]]; then
  SERVICE_URL="http://localhost"
else
  SERVICE_URL="http://localhost:8081"
fi
API_URL="${SERVICE_URL}/api/v1"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
FAULT_ACTIVE=false

log_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }

restore_database() {
  if [[ "$FAULT_ACTIVE" != true ]]; then
    return
  fi
  log_info "Restoring PostgreSQL..."
  if [[ "$MODE" == "kubernetes" ]]; then
    kubectl scale deployment sre-postgres --replicas=1 -n sre-copilot
    kubectl rollout status deployment/sre-postgres -n sre-copilot --timeout=120s
  else
    docker unpause sre-postgres >/dev/null 2>&1 || true
  fi
  FAULT_ACTIVE=false
  log_ok "PostgreSQL restored"
}

on_exit() {
  status=$?
  trap - EXIT INT TERM
  restore_database || true
  if (( status != 0 )); then
    log_error "Simulation stopped with exit code ${status}; the cleanup handler attempted database recovery."
  fi
  exit "$status"
}
trap on_exit EXIT INT TERM

case "$MODE" in
  docker|kubernetes) ;;
  *) log_error "Mode must be 'docker' or 'kubernetes'"; exit 2 ;;
esac

echo -e "${RED}SRE CHAOS ENGINE — PostgreSQL outage simulation${NC}"
log_info "Using incident API at ${API_URL}"

INCIDENT_PAYLOAD='{
  "title": "SIMULATED: PostgreSQL connection pool pressure",
  "serviceName": "incident-service",
  "rawLogs": "level=ERROR msg=\"database query failed\" error=\"pgxpool: context deadline exceeded while acquiring connection\"\nlevel=ERROR msg=\"request failed\" status=503 component=postgresql",
  "firingRule": "DatabaseConnectionPoolExhausted",
  "environment": "staging",
  "createdBy": "chaos-engine"
}'

log_info "Creating a sentinel incident before fault injection..."
INCIDENT_RESPONSE=$(curl --fail-with-body --silent --show-error \
  --connect-timeout 5 --max-time 15 \
  -X POST "${API_URL}/incidents" \
  -H 'Content-Type: application/json' \
  -d "$INCIDENT_PAYLOAD")
INCIDENT_ID=$(printf '%s' "$INCIDENT_RESPONSE" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
log_ok "Incident created: ${INCIDENT_ID}"

log_warn "Injecting PostgreSQL outage for ${FAULT_DURATION_SECONDS}s..."
if [[ "$MODE" == "kubernetes" ]]; then
  kubectl scale deployment sre-postgres --replicas=0 -n sre-copilot
else
  docker pause sre-postgres >/dev/null
fi
FAULT_ACTIVE=true

# Saturate the default 10-connection pgx pool and leave enough callers queued
# to make db_pool_waiting_requests observable after PostgreSQL recovers.
REQUEST_TIMEOUT_SECONDS=$((FAULT_DURATION_SECONDS + 30))
for _ in $(seq 1 32); do
  curl --silent --max-time "$REQUEST_TIMEOUT_SECONDS" \
    "${API_URL}/incidents/stats/dashboard" >/dev/null 2>&1 &
done
log_warn "Fault active; Prometheus should evaluate DatabaseConnectionPoolExhausted."
sleep "$FAULT_DURATION_SECONDS"

restore_database
wait || true
sleep 5

log_info "Triggering AI triage after database recovery..."
TRIAGE_RESPONSE=$(curl --fail-with-body --silent --show-error --max-time 35 \
  -X POST "${API_URL}/incidents/${INCIDENT_ID}/triage" \
  -H 'Content-Type: application/json')
printf '%s\n' "$TRIAGE_RESPONSE" | python3 -m json.tool

log_info "Generating a remediation script..."
REMEDIATION_RESPONSE=$(curl --fail-with-body --silent --show-error --max-time 35 \
  -X POST "${API_URL}/incidents/${INCIDENT_ID}/remediate" \
  -H 'Content-Type: application/json')
printf '%s\n' "$REMEDIATION_RESPONSE" | python3 -m json.tool

log_ok "Simulation complete. Review incident ${INCIDENT_ID} in the dashboard and approve the generated remediation manually."
