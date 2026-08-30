import { useState, useEffect, useCallback } from 'react';
import './index.css';
import { authApi, incidentApi, integrationApi } from './api/incidentApi';
import type {
  Incident, DashboardStats, UserProfile, PlatformIntegrationConfig, IntegrationStatus,
} from './types';
import IncidentCard from './components/IncidentCard';
import IncidentDetailModal from './components/IncidentDetailModal';
import CreateIncidentModal from './components/CreateIncidentModal';
import AuthModal from './components/AuthModal';
import IntegrationSettingsModal from './components/IntegrationSettingsModal';
import StatsBar from './components/StatsBar';
import PipelineHub from './components/PipelineHub';
import {
  Shield, RefreshCw, Plus, Activity, CheckCircle,
  LayoutDashboard, Search, Wifi, WifiOff, GitMerge,
  Settings, Key, LogIn
} from 'lucide-react';

type TabKey = 'all' | 'active' | 'resolved';
type ViewMode = 'incidents' | 'pipelines';

export default function App() {
  const [viewMode, setViewMode] = useState<ViewMode>('incidents');
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>('active');
  const [selectedIncident, setSelectedIncident] = useState<Incident | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [showIntegrationModal, setShowIntegrationModal] = useState(false);
  const [user, setUser] = useState<UserProfile | null>(() => {
    try {
      const saved = sessionStorage.getItem('sre_user');
      return saved ? JSON.parse(saved) : null;
    } catch {
      return null;
    }
  });

  const [integrationConfig, setIntegrationConfig] = useState<PlatformIntegrationConfig>({
    userId: 'local-sre',
    repositoryProvider: 'GITHUB',
    targetBranch: 'main',
    pollingCadence: '15_MINUTES',
    autoRebuild: true,
    autoAITriage: true,
    status: 'DISCONNECTED',
  });

  const [loading, setLoading] = useState(false);
  const [statsLoading, setStatsLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [online, setOnline] = useState(true);

  useEffect(() => {
    if (!user?.token) return;
    let cancelled = false;
    authApi.getMe().catch(() => {
      if (!cancelled) {
        sessionStorage.removeItem('sre_user');
        setUser(null);
      }
    });
    return () => { cancelled = true; };
  }, [user?.token]);

  const loadIntegrations = useCallback(async () => {
    if (!user) return;
    try {
      const data = await integrationApi.getConfig(user.userId);
      if (data) setIntegrationConfig(data);
    } catch (err) {
      console.warn('Could not load integrations', err);
    }
  }, [user]);

  const loadIncidents = useCallback(async () => {
    setLoading(true);
    try {
      if (activeTab === 'active') {
        const data = await incidentApi.getActive();
        setIncidents(data);
      } else {
        const page = await incidentApi.getAll(0, 50);
        if (activeTab === 'resolved') {
          setIncidents(page.content.filter(i => i.status === 'RESOLVED' || i.status === 'CLOSED'));
        } else {
          setIncidents(page.content);
        }
      }
      setOnline(true);
    } catch {
      setOnline(false);
    } finally {
      setLoading(false);
    }
  }, [activeTab]);

  const loadStats = useCallback(async () => {
    setStatsLoading(true);
    try {
      const data = await incidentApi.getDashboardStats();
      setStats(data);
    } catch {
      // Stats failure is non-critical
    } finally {
      setStatsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadIntegrations();
  }, [loadIntegrations]);

  useEffect(() => {
    if (viewMode === 'incidents') {
      loadIncidents();
      loadStats();
    }
  }, [viewMode, loadIncidents, loadStats, user?.token]);

  // Auto-refresh every 30s
  useEffect(() => {
    const timer = setInterval(() => {
      loadIntegrations();
      if (viewMode === 'incidents') {
        loadIncidents();
        loadStats();
      }
    }, 30_000);
    return () => clearInterval(timer);
  }, [viewMode, loadIncidents, loadStats, loadIntegrations]);

  const filteredIncidents = incidents.filter((inc) => {
    if (!searchQuery) return true;
    const q = searchQuery.toLowerCase();
    return (
      inc.title.toLowerCase().includes(q) ||
      inc.serviceName.toLowerCase().includes(q) ||
      (inc.firingRule ?? '').toLowerCase().includes(q)
    );
  });

  const tabs: { key: TabKey; label: string; icon: React.ReactNode }[] = [
    { key: 'active',   label: 'Active',   icon: <Activity size={14}/> },
    { key: 'all',      label: 'All',      icon: <LayoutDashboard size={14}/> },
    { key: 'resolved', label: 'Resolved', icon: <CheckCircle size={14}/> },
  ];

  const integrationStatus: IntegrationStatus = integrationConfig.status
    || (integrationConfig.githubStatus === 'ERROR' || integrationConfig.jenkinsStatus === 'ERROR'
      ? 'ERROR'
      : integrationConfig.githubStatus === 'CONNECTED' && integrationConfig.jenkinsStatus === 'CONNECTED'
        ? 'CONNECTED'
        : 'DISCONNECTED');
  const integrationColor = integrationStatus === 'CONNECTED'
    ? '#34d399'
    : integrationStatus === 'SYNCING'
      ? '#fbbf24'
      : integrationStatus === 'ERROR' ? '#f87171' : '#94a3b8';
  const repositoryLabel = integrationConfig.repositoryUrl
    ? integrationConfig.repositoryUrl.replace(/^https?:\/\//, '').replace(/\/$/, '')
    : integrationConfig.githubRepo || 'Connect integrations';

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>

      {/* ── Top Navigation ───────────────────────────────── */}
      <header style={{
        background: 'var(--color-bg-surface)',
        borderBottom: '1px solid var(--color-border)',
        padding: '0 28px',
        height: 62,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        position: 'sticky',
        top: 0,
        zIndex: 40,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 20 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              width: 34, height: 34, borderRadius: 8,
              background: 'var(--color-primary)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <Shield size={20} color="#fff" />
            </div>
            <div>
              <span style={{ fontWeight: 700, fontSize: 15, color: 'var(--color-text-primary)', letterSpacing: '-0.01em' }}>
                SRE Copilot
              </span>
              <span style={{ fontSize: 11, color: 'var(--color-text-muted)', marginLeft: 8 }}>
                Platform
              </span>
            </div>
          </div>

          {/* View Switcher: Incidents vs CI/CD Pipelines */}
          <nav style={{ display: 'flex', gap: 6, marginLeft: 16 }}>
            <button
              onClick={() => setViewMode('incidents')}
              style={{
                display: 'flex', alignItems: 'center', gap: 6,
                padding: '6px 14px', borderRadius: 8, border: 'none',
                cursor: 'pointer', fontSize: 13, fontWeight: 600,
                background: viewMode === 'incidents' ? 'rgba(59, 130, 246, 0.15)' : 'transparent',
                color: viewMode === 'incidents' ? '#60a5fa' : 'var(--color-text-secondary)',
                borderBottom: viewMode === 'incidents' ? '2px solid #3b82f6' : '2px solid transparent',
                transition: 'all 0.2s',
              }}
            >
              <Activity size={15} /> Incidents Triage
            </button>

            <button
              onClick={() => setViewMode('pipelines')}
              style={{
                display: 'flex', alignItems: 'center', gap: 6,
                padding: '6px 14px', borderRadius: 8, border: 'none',
                cursor: 'pointer', fontSize: 13, fontWeight: 600,
                background: viewMode === 'pipelines' ? 'rgba(59, 130, 246, 0.15)' : 'transparent',
                color: viewMode === 'pipelines' ? '#60a5fa' : 'var(--color-text-secondary)',
                borderBottom: viewMode === 'pipelines' ? '2px solid #3b82f6' : '2px solid transparent',
                transition: 'all 0.2s',
              }}
            >
              <GitMerge size={15} /> CI/CD Pipelines & DORA
            </button>
          </nav>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {/* Platform Integration status badge */}
          <button
            onClick={() => user ? setShowIntegrationModal(true) : setShowAuthModal(true)}
            className="btn btn-ghost"
            title="Configure repository, CI/CD, and autonomous polling"
            style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '5px 12px', fontSize: 12 }}
          >
            <Key size={13} color={integrationColor} />
            <span>{repositoryLabel}</span>
            <span style={{
              color: integrationColor,
              background: integrationColor + '18',
              borderRadius: 999,
              padding: '1px 6px',
              fontSize: 9,
              fontWeight: 750,
              letterSpacing: '.04em',
            }}>
              {integrationStatus}
            </span>
            <Settings size={13} style={{ marginLeft: 2 }} />
          </button>

          {/* User Profile / Auth */}
          {user ? (
            <div style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '4px 10px', borderRadius: 20,
              background: 'rgba(255,255,255,0.05)', border: '1px solid var(--color-border)',
            }}>
              <div style={{
                width: 24, height: 24, borderRadius: '50%', background: '#6366f1',
                display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 11, fontWeight: 700, color: '#fff',
              }}>
                {user.username.substring(0, 2).toUpperCase()}
              </div>
              <div style={{ fontSize: 12, fontWeight: 600, color: '#fff' }}>{user.username}</div>
              <span style={{ fontSize: 10, padding: '1px 6px', borderRadius: 4, background: 'rgba(99, 102, 241, 0.2)', color: '#818cf8', fontWeight: 600 }}>
                {user.role}
              </span>
            </div>
          ) : (
            <button
              onClick={() => setShowAuthModal(true)}
              className="btn btn-primary"
              style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 12px', fontSize: 12 }}
            >
              <LogIn size={13} /> Login
            </button>
          )}

          {/* Online connection indicator */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12,
            color: online ? '#10b981' : '#ef4444', fontWeight: 500 }}>
            {online ? <Wifi size={13}/> : <WifiOff size={13}/>}
          </div>

          {viewMode === 'incidents' && (
            <>
              <button className="btn btn-ghost" onClick={() => { loadIncidents(); loadStats(); }}
                style={{ padding: '6px 10px' }} disabled={loading}>
                <RefreshCw size={14} style={{ animation: loading ? 'spin 1s linear infinite' : 'none' }} />
              </button>

              <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
                <Plus size={14} /> New Incident
              </button>
            </>
          )}
        </div>
      </header>

      {/* ── Main Content ─────────────────────────────────── */}
      <main style={{ flex: 1, padding: '24px 28px', maxWidth: 1340, width: '100%', margin: '0 auto' }}>

        {viewMode === 'pipelines' ? (
          <PipelineHub
            integrationConfig={integrationConfig}
            onOpenIntegrationSettings={() => user ? setShowIntegrationModal(true) : setShowAuthModal(true)}
          />
        ) : (
          <>
            {/* Stats Bar */}
            <div style={{ marginBottom: 28 }}>
              <StatsBar stats={stats} loading={statsLoading} />
            </div>

            {/* Panel header */}
            <div style={{
              background: 'var(--color-bg-card)',
              border: '1px solid var(--color-border)',
              borderRadius: 14,
              overflow: 'hidden',
            }}>
              {/* Tab bar + search */}
              <div style={{
                padding: '16px 20px',
                borderBottom: '1px solid var(--color-border)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 16,
                flexWrap: 'wrap',
              }}>
                <div style={{ display: 'flex', gap: 4 }}>
                  {tabs.map((t) => (
                    <button
                      key={t.key}
                      onClick={() => setActiveTab(t.key)}
                      style={{
                        display: 'flex', alignItems: 'center', gap: 6,
                        padding: '6px 14px', borderRadius: 8, border: 'none',
                        cursor: 'pointer', fontSize: 13, fontWeight: 500,
                        background: activeTab === t.key ? 'rgba(59, 130, 246, 0.15)' : 'transparent',
                        color: activeTab === t.key ? '#60a5fa' : 'var(--color-text-secondary)',
                        transition: 'all 0.15s',
                      }}
                    >
                      {t.icon}
                      <span>{t.label}</span>
                      <span style={{
                        fontSize: 11, padding: '1px 6px', borderRadius: 10,
                        background: 'rgba(255,255,255,0.06)',
                        color: 'var(--color-text-muted)',
                      }}>
                        {t.key === 'active' ? (stats?.openIncidents ?? incidents.length)
                         : t.key === 'resolved' ? (stats?.resolvedToday ?? 0)
                         : incidents.length}
                      </span>
                    </button>
                  ))}
                </div>

                {/* Search */}
                <div style={{ position: 'relative', minWidth: 260 }}>
                  <Search size={14} style={{
                    position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)',
                    color: 'var(--color-text-muted)',
                  }} />
                  <input
                    type="text"
                    placeholder="Search incidents, services, rules…"
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    style={{
                      paddingLeft: 32, paddingRight: 12, paddingTop: 6, paddingBottom: 6,
                      background: 'rgba(255,255,255,0.04)',
                      border: '1px solid var(--color-border)',
                      borderRadius: 8, color: 'var(--color-text-primary)',
                      fontSize: 13, width: '100%', outline: 'none',
                    }}
                  />
                </div>
              </div>

              {/* Incidents List */}
              <div style={{ padding: 16 }}>
                {loading && incidents.length === 0 ? (
                  <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-muted)', fontSize: 13 }}>
                    <RefreshCw size={20} style={{ animation: 'spin 1s linear infinite', margin: '0 auto 8px' }} />
                    Loading incidents…
                  </div>
                ) : filteredIncidents.length === 0 ? (
                  <div style={{ textAlign: 'center', padding: '48px 0', color: 'var(--color-text-muted)', fontSize: 13 }}>
                    <CheckCircle size={24} style={{ color: '#10b981', margin: '0 auto 8px' }} />
                    No incidents found
                  </div>
                ) : (
                  <div style={{ display: 'grid', gap: 10 }}>
                    {filteredIncidents.map((inc) => (
                      <IncidentCard
                        key={inc.id}
                        incident={inc}
                        onClick={(clicked) => setSelectedIncident(clicked)}
                      />
                    ))}
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </main>

      {/* Modals */}
      {selectedIncident && (
        <IncidentDetailModal
          key={selectedIncident.id}
          incident={selectedIncident}
          appliedBy={user?.username || user?.userId || ''}
          userRole={user?.role}
          onClose={() => setSelectedIncident(null)}
          onResolved={() => {
            setSelectedIncident(null);
            loadIncidents();
            loadStats();
          }}
        />
      )}

      {showCreate && (
        <CreateIncidentModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false);
            loadIncidents();
            loadStats();
          }}
        />
      )}

      {showAuthModal && (
        <AuthModal
          onSuccess={(loggedInUser) => {
            setUser(loggedInUser);
            setShowAuthModal(false);
          }}
          onClose={() => setShowAuthModal(false)}
        />
      )}

      {showIntegrationModal && (
        <IntegrationSettingsModal
          user={user}
          onClose={() => setShowIntegrationModal(false)}
          onSaved={(savedConfig) => {
            setIntegrationConfig(savedConfig);
          }}
        />
      )}
    </div>
  );
}
