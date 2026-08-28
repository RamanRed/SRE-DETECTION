import { useState, useEffect, useCallback } from 'react';
import './index.css';
import { incidentApi } from './api/incidentApi';
import type { Incident, DashboardStats } from './types';
import IncidentCard from './components/IncidentCard';
import IncidentDetailModal from './components/IncidentDetailModal';
import CreateIncidentModal from './components/CreateIncidentModal';
import StatsBar from './components/StatsBar';
import {
  Shield, RefreshCw, Plus, Activity, CheckCircle,
  AlertTriangle, LayoutDashboard, Search, Wifi, WifiOff
} from 'lucide-react';

type TabKey = 'all' | 'active' | 'resolved';

export default function App() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [activeTab, setActiveTab] = useState<TabKey>('active');
  const [selectedIncident, setSelectedIncident] = useState<Incident | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [loading, setLoading] = useState(false);
  const [statsLoading, setStatsLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [online, setOnline] = useState(true);
  const [lastRefresh, setLastRefresh] = useState<Date | null>(null);

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
      setLastRefresh(new Date());
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
    loadIncidents();
    loadStats();
  }, [loadIncidents, loadStats]);

  // Auto-refresh every 30s
  useEffect(() => {
    const timer = setInterval(() => {
      loadIncidents();
      loadStats();
    }, 30_000);
    return () => clearInterval(timer);
  }, [loadIncidents, loadStats]);

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

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>

      {/* ── Top Navigation ───────────────────────────────── */}
      <header style={{
        background: 'var(--color-bg-surface)',
        borderBottom: '1px solid var(--color-border)',
        padding: '0 28px',
        height: 56,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        position: 'sticky',
        top: 0,
        zIndex: 40,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <div style={{
            width: 32, height: 32, borderRadius: 8,
            background: 'var(--color-primary)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}>
            <Shield size={18} color="#fff" />
          </div>
          <div>
            <span style={{ fontWeight: 700, fontSize: 15, color: 'var(--color-text-primary)', letterSpacing: '-0.01em' }}>
              SRE Copilot
            </span>
            <span style={{ fontSize: 11, color: 'var(--color-text-muted)', marginLeft: 8 }}>
              Incident Intelligence Platform
            </span>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          {/* Connection status */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 5, fontSize: 11,
            color: online ? '#10b981' : '#ef4444' }}>
            {online ? <Wifi size={12}/> : <WifiOff size={12}/>}
            {online ? 'Connected' : 'Offline'}
          </div>

          {lastRefresh && (
            <span style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>
              Updated {lastRefresh.toLocaleTimeString()}
            </span>
          )}

          <button className="btn btn-ghost" onClick={() => { loadIncidents(); loadStats(); }}
            style={{ padding: '6px 10px' }} disabled={loading}>
            <RefreshCw size={14} style={{ animation: loading ? 'spin 1s linear infinite' : 'none' }} />
          </button>

          <button className="btn btn-primary" onClick={() => setShowCreate(true)}>
            <Plus size={14} /> New Incident
          </button>
        </div>
      </header>

      {/* ── Main Content ─────────────────────────────────── */}
      <main style={{ flex: 1, padding: '24px 28px', maxWidth: 1280, width: '100%', margin: '0 auto' }}>

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
            justifyContent: 'space-between',
            alignItems: 'center',
            flexWrap: 'wrap',
            gap: 12,
          }}>
            <div style={{ display: 'flex', gap: 4 }}>
              {tabs.map((tab) => (
                <button
                  key={tab.key}
                  onClick={() => setActiveTab(tab.key)}
                  style={{
                    display: 'flex', alignItems: 'center', gap: 6,
                    padding: '6px 14px', borderRadius: 8, border: 'none',
                    cursor: 'pointer', fontSize: 13, fontWeight: 500,
                    background: activeTab === tab.key
                      ? 'var(--color-primary)'
                      : 'transparent',
                    color: activeTab === tab.key
                      ? '#fff'
                      : 'var(--color-text-secondary)',
                    transition: 'all 0.2s',
                  }}
                >
                  {tab.icon} {tab.label}
                </button>
              ))}
            </div>

            {/* Search */}
            <div style={{ position: 'relative' }}>
              <Search size={13} style={{
                position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)',
                color: 'var(--color-text-muted)',
              }} />
              <input
                className="input"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search incidents…"
                style={{ paddingLeft: 30, width: 220, height: 34, fontSize: 13 }}
              />
            </div>
          </div>

          {/* Incident List */}
          <div style={{ padding: '12px 16px', display: 'flex', flexDirection: 'column', gap: 8, minHeight: 200 }}>
            {loading ? (
              [...Array(4)].map((_, i) => (
                <div key={i} className="skeleton" style={{ height: 80, borderRadius: 10 }} />
              ))
            ) : filteredIncidents.length === 0 ? (
              <div style={{
                display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
                padding: '60px 20px', gap: 12, color: 'var(--color-text-muted)',
              }}>
                {online ? (
                  <>
                    <CheckCircle size={40} style={{ color: '#10b981', opacity: 0.6 }} />
                    <p style={{ fontSize: 15, fontWeight: 500, color: 'var(--color-text-secondary)' }}>
                      No incidents found
                    </p>
                    <p style={{ fontSize: 13 }}>
                      {activeTab === 'active' ? 'All systems operational 🎉' : 'No incidents match your filter.'}
                    </p>
                  </>
                ) : (
                  <>
                    <AlertTriangle size={40} style={{ color: '#ef4444', opacity: 0.7 }} />
                    <p style={{ fontSize: 15, fontWeight: 500, color: 'var(--color-text-secondary)' }}>
                      Cannot reach incident-service
                    </p>
                    <p style={{ fontSize: 13 }}>Ensure the backend is running on port 8081</p>
                    <button className="btn btn-ghost" onClick={loadIncidents} style={{ marginTop: 8 }}>
                      <RefreshCw size={13} /> Retry
                    </button>
                  </>
                )}
              </div>
            ) : (
              filteredIncidents.map((inc) => (
                <IncidentCard key={inc.id} incident={inc} onClick={setSelectedIncident} />
              ))
            )}
          </div>

          {/* Footer count */}
          {filteredIncidents.length > 0 && (
            <div style={{
              padding: '10px 20px',
              borderTop: '1px solid var(--color-border)',
              fontSize: 12, color: 'var(--color-text-muted)',
              display: 'flex', justifyContent: 'space-between',
            }}>
              <span>Showing {filteredIncidents.length} incident{filteredIncidents.length !== 1 ? 's' : ''}</span>
              <span>Auto-refresh every 30s</span>
            </div>
          )}
        </div>
      </main>

      {/* ── Modals ────────────────────────────────────────── */}
      {selectedIncident && (
        <IncidentDetailModal
          incident={selectedIncident}
          onClose={() => setSelectedIncident(null)}
          onResolved={() => { loadIncidents(); loadStats(); }}
        />
      )}

      {showCreate && (
        <CreateIncidentModal
          onClose={() => setShowCreate(false)}
          onCreated={() => { loadIncidents(); loadStats(); }}
        />
      )}

      {/* ── Global keyframe for spin ──────────────────────── */}
      <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
    </div>
  );
}
