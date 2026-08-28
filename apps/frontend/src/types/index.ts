// ─── Type definitions for the SRE Copilot API ───────────────────────────────

export type IncidentStatus = 'OPEN' | 'ANALYZING' | 'RESOLVED' | 'CLOSED';
export type IncidentSeverity = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
export type ExecutionStatus = 'PENDING' | 'APPROVED' | 'EXECUTING' | 'APPLIED' | 'FAILED' | 'REJECTED';
export type ScriptType = 'KUBECTL_ROLLBACK' | 'ANSIBLE_PLAYBOOK' | 'BASH';

export interface Incident {
  id: string;
  title: string;
  serviceName: string;
  firingRule?: string;
  environment: string;
  status: IncidentStatus;
  severity?: IncidentSeverity;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
}

export interface TriageResult {
  incidentId: string;
  rootCause: string;
  immediateMitigation: string;
  confidenceScore: string;
  severity: IncidentSeverity;
  affectedComponents: string[];
  incidentStatus: IncidentStatus;
}

export interface RemediationResult {
  remediationId: string;
  incidentId: string;
  scriptType: ScriptType;
  executableScript: string;
  requiresManualApproval: boolean;
  executionStatus: ExecutionStatus;
}

export interface DashboardStats {
  openIncidents: number;
  analyzingIncidents: number;
  resolvedToday: number;
  pendingRemediations: number;
  appliedRemediations: number;
}

export interface PageResponse<T> {
  content: T[];
  totalElements: number;
  totalPages: number;
  size: number;
  number: number;
}

export interface CreateIncidentPayload {
  title: string;
  serviceName: string;
  rawLogs?: string;
  firingRule?: string;
  environment: string;
  createdBy?: string;
}
