package remediation

import (
	"strings"
	"testing"
)

func TestGeneratorScripts(t *testing.T) {
	generator := NewGenerator("sre-copilot")
	tests := []struct {
		name       string
		rootCause  string
		scriptType string
		script     string
	}{
		{
			name:       "database",
			rootCause:  "Hikari database connection exhaustion",
			scriptType: "KUBECTL_ROLLBACK",
			script: `#!/bin/bash
# SRE Remediation: Database connection pool recovery & Pod restart
echo "[INFO] Flushing idle connection leaks in PostgreSQL..."
kubectl rollout restart deployment/incident-service -n sre-copilot
kubectl rollout status deployment/incident-service -n sre-copilot --timeout=60s
echo "[SUCCESS] Service restarted successfully with clean connection state."`,
		},
		{
			name:       "memory",
			rootCause:  "OOM memory exhaustion",
			scriptType: "KUBECTL_ROLLBACK",
			script: `#!/bin/bash
# SRE Remediation: OOM recovery within the platform's 128 Mi memory budget
set -euo pipefail
echo "[INFO] Capturing pod state before restart..."
kubectl describe deployment/incident-service -n sre-copilot
kubectl top pods -l app=incident-service -n sre-copilot || true
kubectl set resources deployment/incident-service --requests=memory=64Mi --limits=memory=128Mi -n sre-copilot
kubectl rollout restart deployment/incident-service -n sre-copilot
kubectl rollout status deployment/incident-service -n sre-copilot --timeout=120s
echo "[SUCCESS] Deployment restarted within the approved memory budget."`,
		},
		{
			name:       "generic",
			rootCause:  "unknown anomaly",
			scriptType: "ANSIBLE_PLAYBOOK",
			script: `---
- name: SRE Automated Node Recovery
  hosts: k8s_nodes
  become: yes
  tasks:
    - name: Inspect node disk usage before recovery
      command: df -h
      changed_when: false
    - name: Capture K3s service status before recovery
      command: systemctl status k3s --no-pager
      changed_when: false
      failed_when: false
    - name: Restart the single-node K3s server
      systemd:
        name: k3s
        state: restarted
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := generator.Generate(Input{IncidentID: "inc", RootCause: test.rootCause, TargetSystem: "ignored"})
			if result.ScriptType != test.scriptType || result.ExecutableScript != test.script {
				t.Fatalf("unexpected result:\n%+v\nwant script:\n%s", result, test.script)
			}
			if !result.RequiresManualApproval {
				t.Fatal("manual approval must always be required")
			}
			if result.VerificationPlan == "" || result.RollbackPlan == "" {
				t.Fatalf("verification and rollback plans are required: %+v", result)
			}
			if result.UnifiedDiff != "" {
				t.Fatalf("operational remediation must not fabricate a source diff: %q", result.UnifiedDiff)
			}
		})
	}
}

func TestGeneratorOOMRemediationStaysWithinGoMemoryBudget(t *testing.T) {
	result := NewGenerator("sre-copilot").Generate(Input{RootCause: "Go runtime out of memory"})
	for _, required := range []string{
		"--requests=memory=64Mi --limits=memory=128Mi",
		"kubectl describe deployment/incident-service",
		"kubectl rollout status deployment/incident-service",
	} {
		if !strings.Contains(result.ExecutableScript, required) {
			t.Errorf("OOM remediation missing %q:\n%s", required, result.ExecutableScript)
		}
	}
	for _, forbidden := range []string{"512Mi", "1024Mi", "k3s-agent"} {
		if strings.Contains(result.ExecutableScript, forbidden) {
			t.Errorf("OOM remediation contains unsafe legacy value %q:\n%s", forbidden, result.ExecutableScript)
		}
	}
	if !result.RequiresManualApproval {
		t.Fatal("OOM remediation must require manual approval")
	}
}

func TestGeneratorRestartsK3sServerNotAgent(t *testing.T) {
	result := NewGenerator("sre-copilot").Generate(Input{RootCause: "unknown anomaly"})
	if !strings.Contains(result.ExecutableScript, "name: k3s") || strings.Contains(result.ExecutableScript, "k3s-agent") {
		t.Fatalf("generic recovery targets the wrong K3s unit:\n%s", result.ExecutableScript)
	}
	if strings.Contains(result.ExecutableScript, "docker system prune") || !strings.Contains(result.ExecutableScript, "command: df -h") {
		t.Fatalf("generic recovery must inspect disk without destructive pruning:\n%s", result.ExecutableScript)
	}
}

func TestGeneratorDatabaseKeywordsWinOverMemory(t *testing.T) {
	result := NewGenerator("platform").Generate(Input{RootCause: "database OOM memory failure"})
	if result.ScriptType != "KUBECTL_ROLLBACK" {
		t.Fatalf("script type = %q", result.ScriptType)
	}
	want := "kubectl rollout status deployment/incident-service -n platform --timeout=60s"
	if !contains(result.ExecutableScript, want) {
		t.Fatalf("database script did not win or namespace missing:\n%s", result.ExecutableScript)
	}
}

func contains(value, fragment string) bool {
	for index := 0; index+len(fragment) <= len(value); index++ {
		if value[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
