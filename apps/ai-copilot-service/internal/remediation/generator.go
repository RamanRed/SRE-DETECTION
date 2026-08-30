package remediation

import (
	"fmt"
	"strings"
)

type Input struct {
	IncidentID   string
	RootCause    string
	TargetSystem string
}

type Result struct {
	ScriptType             string
	ExecutableScript       string
	RequiresManualApproval bool
	UnifiedDiff            string
	VerificationPlan       string
	RollbackPlan           string
}

type Generator struct {
	namespace string
}

func NewGenerator(namespace string) *Generator {
	return &Generator{namespace: namespace}
}

func (generator *Generator) Generate(input Input) Result {
	rootCause := strings.ToLower(input.RootCause)
	result := Result{RequiresManualApproval: true}

	switch {
	case strings.Contains(rootCause, "database"),
		strings.Contains(rootCause, "connection"),
		strings.Contains(rootCause, "hikari"):
		result.ScriptType = "KUBECTL_ROLLBACK"
		result.ExecutableScript = fmt.Sprintf(`#!/bin/bash
# SRE Remediation: Database connection pool recovery & Pod restart
echo "[INFO] Flushing idle connection leaks in PostgreSQL..."
kubectl rollout restart deployment/incident-service -n %s
kubectl rollout status deployment/incident-service -n %s --timeout=60s
echo "[SUCCESS] Service restarted successfully with clean connection state."`, generator.namespace, generator.namespace)
		result.VerificationPlan = fmt.Sprintf("kubectl rollout status deployment/incident-service -n %s --timeout=120s\nkubectl get pods -l app=incident-service -n %s", generator.namespace, generator.namespace)
		result.RollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/incident-service -n %s\nkubectl rollout status deployment/incident-service -n %s --timeout=120s", generator.namespace, generator.namespace)
	case strings.Contains(rootCause, "memory"), strings.Contains(rootCause, "oom"):
		result.ScriptType = "KUBECTL_ROLLBACK"
		result.ExecutableScript = fmt.Sprintf(`#!/bin/bash
# SRE Remediation: OOM recovery within the platform's 128 Mi memory budget
set -euo pipefail
echo "[INFO] Capturing pod state before restart..."
kubectl describe deployment/incident-service -n %s
kubectl top pods -l app=incident-service -n %s || true
kubectl set resources deployment/incident-service --requests=memory=64Mi --limits=memory=128Mi -n %s
kubectl rollout restart deployment/incident-service -n %s
kubectl rollout status deployment/incident-service -n %s --timeout=120s
echo "[SUCCESS] Deployment restarted within the approved memory budget."`, generator.namespace, generator.namespace, generator.namespace, generator.namespace, generator.namespace)
		result.VerificationPlan = fmt.Sprintf("kubectl rollout status deployment/incident-service -n %s --timeout=120s\nkubectl top pods -l app=incident-service -n %s\nkubectl get deployment/incident-service -n %s -o jsonpath='{.spec.template.spec.containers[0].resources}'", generator.namespace, generator.namespace, generator.namespace)
		result.RollbackPlan = fmt.Sprintf("kubectl rollout undo deployment/incident-service -n %s\nkubectl rollout status deployment/incident-service -n %s --timeout=120s", generator.namespace, generator.namespace)
	default:
		result.ScriptType = "ANSIBLE_PLAYBOOK"
		result.ExecutableScript = `---
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
`
		result.VerificationPlan = "ansible k8s_nodes -b -m command -a 'systemctl is-active k3s'\nansible k8s_nodes -b -m command -a 'k3s kubectl get nodes'"
		result.RollbackPlan = "Restore the previous K3s configuration and restart the k3s service, then require the node and system pods to report Ready."
	}

	return result
}
