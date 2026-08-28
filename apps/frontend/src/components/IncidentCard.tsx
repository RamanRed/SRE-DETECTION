import type { Incident } from '../types';
import { AlertTriangle, Clock, Server, Activity, ChevronRight } from 'lucide-react';

interface Props {
  incident: Incident;
  onClick: (incident: Incident) => void;
}

const SEVERITY_CLASS: Record<string, string> = {
  CRITICAL: 'badge-critical',
  HIGH: 'badge-high',
  MEDIUM: 'badge-medium',
  LOW: 'badge-low',
};
const STATUS_CLASS: Record<string, string> = {
  OPEN:      'badge-open',
  ANALYZING: 'badge-analyzing',
  RESOLVED:  'badge-resolved',
  CLOSED:    'badge-closed',
};

function timeAgo(iso: string) {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (diff < 60)  return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

export default function IncidentCard({ incident, onClick }: Props) {
  const isActive = incident.status === 'OPEN' || incident.status === 'ANALYZING';
  return (
    <div
      className="card animate-fade-in"
      style={{ padding: '16px 20px', cursor: 'pointer', position: 'relative' }}
      onClick={() => onClick(incident)}
    >
      {/* Severity accent bar */}
      <div style={{
        position: 'absolute',
        left: 0, top: 0, bottom: 0,
        width: '3px',
        borderRadius: '12px 0 0 12px',
        background: incident.severity === 'CRITICAL' ? '#dc2626'
                  : incident.severity === 'HIGH'     ? '#ef4444'
                  : incident.severity === 'MEDIUM'   ? '#f59e0b'
                  :                                    '#10b981',
      }} />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12 }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6, flexWrap: 'wrap' }}>
            <span className={`badge ${STATUS_CLASS[incident.status] ?? 'badge-open'}`}>
              {incident.status === 'ANALYZING' && (
                <Activity size={10} style={{ animation: 'pulse-dot 1.5s infinite' }} />
              )}
              {incident.status}
            </span>
            {incident.severity && (
              <span className={`badge ${SEVERITY_CLASS[incident.severity] ?? ''}`}>
                <AlertTriangle size={10} />
                {incident.severity}
              </span>
            )}
          </div>

          <h3 style={{
            fontSize: '14px',
            fontWeight: 600,
            color: 'var(--color-text-primary)',
            marginBottom: 6,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {incident.title}
          </h3>

          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-text-secondary)' }}>
              <Server size={11} />
              {incident.serviceName}
            </span>
            <span style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-text-muted)' }}>
              <Clock size={11} />
              {timeAgo(incident.createdAt)}
            </span>
            {incident.firingRule && (
              <span style={{ fontSize: 12, color: '#6366f1', fontFamily: 'monospace' }}>
                {incident.firingRule}
              </span>
            )}
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {isActive && (
            <div className="animate-pulse-dot" style={{
              width: 8, height: 8, borderRadius: '50%',
              background: incident.status === 'ANALYZING' ? '#f59e0b' : '#ef4444',
              flexShrink: 0,
            }} />
          )}
          <ChevronRight size={16} style={{ color: 'var(--color-text-muted)' }} />
        </div>
      </div>
    </div>
  );
}
