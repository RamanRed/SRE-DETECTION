import axios from 'axios';
import type {
  Incident,
  TriageResult,
  RemediationResult,
  DashboardStats,
  PageResponse,
  CreateIncidentPayload,
  PipelineBuild,
  DoraMetrics,
  UserProfile,
  PlatformIntegrationConfig,
  PlatformSyncResult,
} from '../types';

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
});

// ─── Request interceptor for logging ─────────────────────────────
api.interceptors.request.use((config) => {
  console.debug(`[API] ${config.method?.toUpperCase()} ${config.url}`);
  return config;
});

// ─── Response interceptor for error formatting ────────────────────
api.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error.response?.data?.message ?? error.message ?? 'Unknown API error';
    console.error('[API Error]', message);
    return Promise.reject(new Error(message));
  }
);

// ─── Incident API ─────────────────────────────────────────────────
export const incidentApi = {
  create: (payload: CreateIncidentPayload) =>
    api.post<Incident>('/incidents', payload).then((r) => r.data),

  getAll: (page = 0, size = 20) =>
    api.get<PageResponse<Incident>>('/incidents', { params: { page, size } }).then((r) => r.data),

  getById: (id: string) =>
    api.get<Incident>(`/incidents/${id}`).then((r) => r.data),

  getActive: () =>
    api.get<Incident[]>('/incidents/active').then((r) => r.data),

  triggerTriage: (id: string) =>
    api.post<TriageResult>(`/incidents/${id}/triage`).then((r) => r.data),

  generateRemediation: (id: string) =>
    api.post<RemediationResult>(`/incidents/${id}/remediate`).then((r) => r.data),

  approveRemediation: (incidentId: string, remediationId: string, appliedBy: string) =>
    api.post<RemediationResult>(
      `/incidents/${incidentId}/remediation/${remediationId}/approve`,
      { appliedBy }
    ).then((r) => r.data),

  getDashboardStats: () =>
    api.get<DashboardStats>('/incidents/stats/dashboard').then((r) => r.data),
};

// ─── CI/CD Pipeline & DORA API ────────────────────────────────────
export const pipelineApi = {
  getBuilds: (page = 0, size = 20) =>
    api.get<PageResponse<PipelineBuild>>('/ci/builds', { params: { page, size } }).then((r) => r.data),

  getDoraMetrics: () =>
    api.get<DoraMetrics>('/ci/metrics/dora').then((r) => r.data),

  triggerSync: () =>
    api.post<{ message: string; doraMetrics: DoraMetrics }>('/ci/sync').then((r) => r.data),

  sendWebhook: (payload: Partial<PipelineBuild>) =>
    api.post<PipelineBuild>('/ci/webhook', payload).then((r) => r.data),
};

// ─── Auth API ─────────────────────────────────────────────────────
export const authApi = {
  login: (username: string, role: string) =>
    api.post<UserProfile>('/auth/login', { username, role }).then((r) => r.data),

  getMe: (userId: string) =>
    api.get<UserProfile>('/auth/me', { params: { userId } }).then((r) => r.data),
};

// ─── Platform Integrations API ────────────────────────────────────
export const integrationApi = {
  getConfig: (userId: string) =>
    api.get<PlatformIntegrationConfig>('/integrations/config', { params: { userId } }).then((r) => r.data),

  saveConfig: (payload: Partial<PlatformIntegrationConfig>) =>
    api.post<PlatformIntegrationConfig>('/integrations/config', payload).then((r) => r.data),

  syncPlatforms: (userId: string) =>
    api.post<PlatformSyncResult>('/integrations/sync', null, { params: { userId } }).then((r) => r.data),
};

export default api;
