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
  unifiedDiff?: string;
  verificationPlan?: string;
  rollbackPlan?: string;
  citedSourcePaths?: string[];
  repositoryUrl?: string;
  commitSha?: string;
  sourceReferences?: string[];
}

export interface RemediationResult {
  remediationId: string;
  incidentId: string;
  scriptType: ScriptType;
  executableScript: string;
  requiresManualApproval: boolean;
  executionStatus: ExecutionStatus;
  incidentStatus?: IncidentStatus;
  message?: string;
  unifiedDiff?: string;
  verificationPlan?: string;
  rollbackPlan?: string;
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
  ciTool: 'JENKINS' | 'GITHUB_ACTIONS' | 'GITLAB_CI' | 'KUBERNETES_JOB';
  status: 'SUCCESS' | 'FAILURE' | 'RUNNING' | 'QUEUED' | 'UNSTABLE' | 'CANCELLED' | 'TIMED_OUT';
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
  dataAvailable: boolean;
  deploymentFrequency: string;
  leadTimeForChanges: string;
  changeFailureRate: number;
  meanTimeToRecovery: string;
  totalBuilds: number;
  successfulBuilds: number;
  failedBuilds: number;
  recentBuilds: PipelineBuild[];
  asOf?: string;
  calculatedAt?: string;
  generatedAt?: string;
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

export type RepositoryProvider = 'GITHUB' | 'GITLAB' | 'BITBUCKET';
export type PipelineEngine = 'JENKINS' | 'GITHUB_ACTIONS' | 'KUBERNETES_JOB';
export type PollingCadence = '5_MINUTES' | '15_MINUTES' | '1_HOUR' | 'DAILY_CRON';
export type IntegrationStatus = 'CONNECTED' | 'SYNCING' | 'DISCONNECTED' | 'ERROR';

export interface PlatformIntegrationConfig {
  id?: number;
  userId: string;
  username?: string;
  repositoryProvider?: RepositoryProvider;
  repositoryUrl?: string;
  targetBranch?: string;
  repositoryToken?: string;
  repositoryTokenConfigured?: boolean;
  repositoryStatus?: IntegrationStatus;
  pipelineEngine?: PipelineEngine;
  ciBaseUrl?: string;
  ciUsername?: string;
  ciToken?: string;
  ciTokenConfigured?: boolean;
  jobName?: string;
  pollingCadence?: PollingCadence;
  autoRebuild?: boolean;
  autoAITriage?: boolean;
  status?: IntegrationStatus;
  lastPolledCommit?: string;
  lastError?: string;

  // Legacy aliases remain during the API migration so existing saved records
  // can be edited without exposing or re-entering their credentials.
  githubToken?: string;
  githubRepo?: string; // e.g. "RamanRed/SRE-DETECTION"
  githubBranch?: string; // e.g. "master"
  githubStatus?: IntegrationStatus;
  githubTokenConfigured?: boolean;
  jenkinsUrl?: string;
  jenkinsUsername?: string;
  jenkinsApiToken?: string;
  jenkinsJobName?: string;
  jenkinsStatus?: IntegrationStatus;
  jenkinsTokenConfigured?: boolean;
  lastSyncTime?: string;
  message?: string;
}

export interface IntegrationConnectPayload {
  userId: string;
  username?: string;
  repositoryProvider: RepositoryProvider;
  repositoryUrl: string;
  targetBranch: string;
  repositoryToken?: string;
  pipelineEngine: PipelineEngine;
  ciBaseUrl?: string;
  ciUsername?: string;
  ciToken?: string;
  jobName: string;
  pollingCadence: PollingCadence;
  autoRebuild: boolean;
  autoAITriage: boolean;
}

export interface PlatformSyncResult {
  success: boolean;
  status?: IntegrationStatus;
  githubStatus?: string;
  jenkinsStatus?: string;
  repositoryStatus?: string;
  ciStatus?: string;
  commitsSynced: number;
  buildsSynced: number;
  message: string;
}
