#!/usr/bin/env bash
# Non-destructive application smoke test. It creates one uniquely named
# incident and approves a remediation record without executing it.

set -Eeuo pipefail

BASE_URL="${BASE_URL:-http://localhost}"
API_URL="${BASE_URL%/}/api/v1"
AI_MANAGEMENT_URL="${AI_MANAGEMENT_URL:-http://localhost:8082}"
SMOKE_ID="smoke-$(date -u +%Y%m%dT%H%M%SZ)-$$"

request() {
  curl --fail-with-body --silent --show-error \
    --connect-timeout 5 --max-time 35 "$@"
}

json_field() {
  field="$1"
  python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$field"
}

echo "Checking frontend, incident-service, and ai-copilot health..."
request "${BASE_URL%/}/health" >/dev/null
request "${API_URL}/incidents/health" >/dev/null
request "${AI_MANAGEMENT_URL%/}/actuator/health/readiness" >/dev/null

CREATE_PAYLOAD=$(SMOKE_ID="$SMOKE_ID" python3 -c '
import json, os
smoke_id = os.environ["SMOKE_ID"]
print(json.dumps({
    "title": f"E2E smoke incident {smoke_id}",
    "serviceName": "smoke-service",
    "rawLogs": "level=ERROR msg=\"PostgreSQL connection refused\" error=\"pgxpool: failed to acquire connection: dial tcp connection refused\"",
    "firingRule": "DatabaseConnectionPoolExhausted",
    "environment": "staging",
    "createdBy": "e2e-smoke"
}))')

echo "Creating ${SMOKE_ID}..."
CREATE_RESPONSE=$(request -X POST "${API_URL}/incidents" \
  -H 'Content-Type: application/json' -d "$CREATE_PAYLOAD")
INCIDENT_ID=$(printf '%s' "$CREATE_RESPONSE" | json_field id)
[[ -n "$INCIDENT_ID" ]]

echo "Triaging incident ${INCIDENT_ID}..."
TRIAGE_RESPONSE=$(request -X POST "${API_URL}/incidents/${INCIDENT_ID}/triage" \
  -H 'Content-Type: application/json')
[[ "$(printf '%s' "$TRIAGE_RESPONSE" | json_field incidentId)" == "$INCIDENT_ID" ]]
printf '%s' "$TRIAGE_RESPONSE" | python3 -c '
import json, sys
body = json.load(sys.stdin)
assert body.get("rootCause")
assert body.get("immediateMitigation")
assert body.get("severity") in {"LOW", "MEDIUM", "HIGH", "CRITICAL"}
'

echo "Generating remediation..."
REMEDIATION_RESPONSE=$(request -X POST "${API_URL}/incidents/${INCIDENT_ID}/remediate" \
  -H 'Content-Type: application/json')
REMEDIATION_ID=$(printf '%s' "$REMEDIATION_RESPONSE" | json_field remediationId)
[[ -n "$REMEDIATION_ID" ]]
printf '%s' "$REMEDIATION_RESPONSE" | python3 -c '
import json, sys
body = json.load(sys.stdin)
assert body.get("scriptType") == "KUBECTL_ROLLBACK", body
assert body.get("executableScript"), body
'

echo "Approving remediation ${REMEDIATION_ID}..."
APPROVAL_RESPONSE=$(request -X POST \
  "${API_URL}/incidents/${INCIDENT_ID}/remediation/${REMEDIATION_ID}/approve" \
  -H 'Content-Type: application/json' \
  -d '{"appliedBy":"e2e-smoke"}')
[[ "$(printf '%s' "$APPROVAL_RESPONSE" | json_field executionStatus)" == "APPROVED" ]]

INCIDENT_RESPONSE=$(request "${API_URL}/incidents/${INCIDENT_ID}")
[[ "$(printf '%s' "$INCIDENT_RESPONSE" | json_field status)" == "ANALYZING" ]]

STATS_RESPONSE=$(request "${API_URL}/incidents/stats/dashboard")
printf '%s' "$STATS_RESPONSE" | python3 -c '
import json, sys
body = json.load(sys.stdin)
required = {
    "openIncidents", "analyzingIncidents", "resolvedToday",
    "pendingRemediations", "appliedRemediations"
}
assert required <= body.keys()
assert all(isinstance(body[key], int) and body[key] >= 0 for key in required)
'

echo "E2E smoke passed; incident ${INCIDENT_ID} was created, triaged, and its remediation was approved without claiming execution."
