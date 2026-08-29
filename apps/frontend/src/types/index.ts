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
  rawLogs?: string;
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

export interface PipelineBuild {
  id: string;
  pipelineName: string;
  buildNumber: number;
  ciTool: 'JENKINS' | 'GITHUB_ACTIONS' | 'GITLAB_CI';
  status: 'SUCCESS' | 'FAILURE' | 'RUNNING' | 'UNSTABLE';
  gitCommit: string;
  gitBranch: string;
  commitMessage?: string;
  author: string;
  durationSeconds: number;
  testsPassed: number;
  testsFailed: number;
  vulnerabilitiesDetected: number;
  environment: string;
  logSnippet?: string;
  buildUrl?: string;
  createdAt: string;
  updatedAt: string;
}

export interface DoraMetrics {
  deploymentFrequency: string;
  leadTimeForChanges: string;
  changeFailureRate: number;
  meanTimeToRecovery: string;
  totalBuilds: number;
  successfulBuilds: number;
  failedBuilds: number;
  recentBuilds: PipelineBuild[];
}

export interface UserProfile {
  authenticated: boolean;
  token: string;
  userId: string;
  username: string;
  email: string;
  role: 'SRE_LEAD' | 'DEVOPS_ENGINEER' | 'EVALUATOR';
  avatarUrl?: string;
}

export interface PlatformIntegrationConfig {
  id?: number;
  userId: string;
  username?: string;
  githubToken?: string;
  githubRepo?: string; // e.g. "RamanRed/SRE-DETECTION"
  githubBranch?: string; // e.g. "master"
  githubStatus?: 'CONNECTED' | 'DISCONNECTED' | 'ERROR';
  githubTokenConfigured?: boolean;
  jenkinsUrl?: string;
  jenkinsUsername?: string;
  jenkinsApiToken?: string;
  jenkinsJobName?: string;
  jenkinsStatus?: 'CONNECTED' | 'DISCONNECTED' | 'ERROR';
  jenkinsTokenConfigured?: boolean;
  lastSyncTime?: string;
  message?: string;
}

export interface PlatformSyncResult {
  success: boolean;
  githubStatus: string;
  jenkinsStatus: string;
  commitsSynced: number;
  buildsSynced: number;
  message: string;
}
