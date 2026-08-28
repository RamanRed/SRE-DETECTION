#!/bin/bash
# ─────────────────────────────────────────────────────────────────────────────
# SRE Incident Simulation: Database Connection Pool Exhaustion Fault Injection
# ─────────────────────────────────────────────────────────────────────────────
# Usage: ./scripts/simulate_outage.sh [docker|kubernetes]
#
# This script:
# 1. Injects a fault (DB container pause / scale-to-0)
# 2. Creates test incidents to fire Prometheus alerts
# 3. Waits for AI Copilot to triage the incident
# 4. Automatically restores the DB
# ─────────────────────────────────────────────────────────────────────────────

set -e

MODE="${1:-docker}"
INCIDENT_SERVICE_URL="${INCIDENT_SERVICE_URL:-http://localhost:8081}"
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

log_info()  { echo -e "${BLUE}[INFO]${NC}  $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $1"; }

echo ""
echo -e "${RED}╔══════════════════════════════════════════════════════════════╗"
echo -e "║  SRE CHAOS ENGINE — Database Outage Simulation               ║"
echo -e "╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# ── Step 1: Create a sentinel incident (pre-fault) ────────────────────────────
log_info "Creating pre-fault baseline incident..."
INCIDENT_PAYLOAD='{
  "title": "SIMULATED: PostgreSQL Connection Pool Exhaustion",
  "serviceName": "incident-service",
  "rawLogs": "HikariPool-1 - Connection is not available, request timed out after 30000ms\norg.springframework.dao.DataAccessResourceFailureException: Unable to acquire JDBC Connection\nCaused by: java.sql.SQLTransientConnectionException: HikariPool-1 - Connection is not available\n\tat com.zaxxer.hikari.pool.HikariPool.getConnection(HikariPool.java:213)",
  "firingRule": "DatabaseConnectionPoolExhausted",
  "environment": "production",
  "createdBy": "chaos-engine"
}'

INCIDENT_RESPONSE=$(curl -s -X POST "${INCIDENT_SERVICE_URL}/api/v1/incidents" \
  -H 'Content-Type: application/json' \
  -d "${INCIDENT_PAYLOAD}" 2>/dev/null || echo '{"error":"service_unavailable"}')

INCIDENT_ID=$(echo "$INCIDENT_RESPONSE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4 | head -1)
log_ok "Incident created: ${INCIDENT_ID}"

# ── Step 2: Inject fault ──────────────────────────────────────────────────────
echo ""
log_warn "INJECTING FAULT: Pausing PostgreSQL container for 60s..."

if [[ "$MODE" == "kubernetes" ]]; then
  kubectl scale deployment sre-postgres --replicas=0 -n sre-copilot
  log_warn "K8s: Scaled postgres to 0 replicas"
else
  docker pause sre-postgres 2>/dev/null || log_warn "Docker: postgres container already stopped or not found"
fi

log_warn "💥 FAULT INJECTED — Prometheus should fire 'DatabaseConnectionPoolExhausted' alert within 60s"
echo ""

# ── Step 3: Trigger AI triage on the incident ──────────────────────────────────
if [[ -n "$INCIDENT_ID" ]]; then
  log_info "Triggering AI triage for incident: ${INCIDENT_ID}..."
  sleep 5
  TRIAGE_RESPONSE=$(curl -s -X POST "${INCIDENT_SERVICE_URL}/api/v1/incidents/${INCIDENT_ID}/triage" \
    -H 'Content-Type: application/json' 2>/dev/null || echo '{"error":"timeout"}')
  log_ok "AI Triage response:"
  echo "${TRIAGE_RESPONSE}" | python3 -m json.tool 2>/dev/null || echo "${TRIAGE_RESPONSE}"

  # Generate remediation script
  log_info "Generating remediation script..."
  REMEDIATION=$(curl -s -X POST "${INCIDENT_SERVICE_URL}/api/v1/incidents/${INCIDENT_ID}/remediate" \
    -H 'Content-Type: application/json' 2>/dev/null || echo '{}')
  log_ok "Remediation script generated:"
  echo "${REMEDIATION}" | python3 -m json.tool 2>/dev/null || echo "${REMEDIATION}"
fi

# ── Step 4: Wait and restore ──────────────────────────────────────────────────
log_warn "Waiting 60s before restoring database..."
sleep 60

log_info "RESTORING: Unpausing PostgreSQL..."
if [[ "$MODE" == "kubernetes" ]]; then
  kubectl scale deployment sre-postgres --replicas=1 -n sre-copilot
  log_ok "K8s: Postgres scaled back to 1 replica"
else
  docker unpause sre-postgres 2>/dev/null || log_ok "Docker: postgres resumed"
fi

log_ok "Database restored. HikariCP should reconnect within 30s."
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════"
echo -e "  SIMULATION COMPLETE"
echo -e "  Incident ID: ${INCIDENT_ID}"
echo -e "  → Now review the incident in the SRE Copilot Dashboard"
echo -e "  → Check Grafana for SLO burn rate alert resolution"
echo -e "  → Approve the remediation script in the UI"
echo -e "═══════════════════════════════════════════════════════════════${NC}"
