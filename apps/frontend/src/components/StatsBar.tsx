import type { DashboardStats } from '../types';
import { AlertTriangle, Activity, CheckCircle, Clock, Zap } from 'lucide-react';

interface Props {
  stats: DashboardStats | null;
  loading?: boolean;
}

interface StatItemProps {
  label: string;
  value: number | string;
  icon: React.ReactNode;
  color: string;
  sublabel?: string;
}

function StatItem({ label, value, icon, color, sublabel }: StatItemProps) {
  return (
    <div className="stat-card" style={{ flex: 1, minWidth: '160px' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div>
          <p className="stat-label">{label}</p>
          <p className="stat-value" style={{ color }}>{value}</p>
          {sublabel && <p style={{ fontSize: 11, color: 'var(--color-text-muted)', marginTop: 4 }}>{sublabel}</p>}
        </div>
        <div style={{
          width: 40, height: 40, borderRadius: 10,
          background: `${color}18`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          color,
        }}>
          {icon}
        </div>
      </div>
    </div>
  );
}

export default function StatsBar({ stats, loading }: Props) {
  if (loading || !stats) {
    return (
      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
        {[...Array(5)].map((_, i) => (
          <div key={i} className="skeleton" style={{ flex: 1, minWidth: '160px', height: '90px' }} />
        ))}
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
      <StatItem
        label="Open Incidents"
        value={stats.openIncidents}
        icon={<AlertTriangle size={20} />}
        color="#ef4444"
        sublabel="Requires attention"
      />
      <StatItem
        label="AI Analyzing"
        value={stats.analyzingIncidents}
        icon={<Activity size={20} />}
        color="#f59e0b"
        sublabel="In triage pipeline"
      />
      <StatItem
        label="Resolved Today"
        value={stats.resolvedToday}
        icon={<CheckCircle size={20} />}
        color="#10b981"
        sublabel="Since midnight"
      />
      <StatItem
        label="Pending Scripts"
        value={stats.pendingRemediations}
        icon={<Clock size={20} />}
        color="#6366f1"
        sublabel="Awaiting SRE approval"
      />
      <StatItem
        label="Applied Fixes"
        value={stats.appliedRemediations}
        icon={<Zap size={20} />}
        color="#3b82f6"
        sublabel="Total applied"
      />
    </div>
  );
}
