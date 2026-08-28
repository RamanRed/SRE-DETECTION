import axios from 'axios';
import type {
  Incident,
  TriageResult,
  RemediationResult,
  DashboardStats,
  PageResponse,
  CreateIncidentPayload,
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

export default api;
