import { useState, useEffect } from 'react';
import { integrationApi } from '../api/incidentApi';
import type { PlatformIntegrationConfig, UserProfile } from '../types';
import {
  Settings, Key, X, Check,
  RefreshCw, CheckCircle2
} from 'lucide-react';

interface Props {
  user: UserProfile | null;
  onClose: () => void;
  onSaved: (config: PlatformIntegrationConfig) => void;
}

export default function IntegrationSettingsModal({ user, onClose, onSaved }: Props) {
  const [config, setConfig] = useState<PlatformIntegrationConfig>({
    userId: user?.userId || 'ramanred',
    username: user?.username || 'RamanRed',
    githubToken: '',
    githubRepo: 'RamanRed/SRE-DETECTION',
    githubBranch: 'master',
    jenkinsUrl: 'http://16.16.175.206:8080',
    jenkinsUsername: 'admin',
    jenkinsApiToken: '',
    jenkinsJobName: 're-copilot-pipeline',
    githubStatus: 'CONNECTED',
    jenkinsStatus: 'CONNECTED',
  });

  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);

  useEffect(() => {
    const userId = user?.userId || 'ramanred';
    integrationApi.getConfig(userId).then((data) => {
      if (data) {
        setConfig((prev) => ({ ...prev, ...data }));
      }
    }).catch((err) => console.error('Failed to load integration config', err));
  }, [user]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setStatusMsg(null);
    try {
      const saved = await integrationApi.saveConfig({
        ...config,
        userId: user?.userId || 'ramanred',
        username: user?.username || 'RamanRed',
      });
      setStatusMsg('Configuration and tokens saved successfully!');
      onSaved(saved);
      setTimeout(() => setStatusMsg(null), 3000);
    } catch (err: unknown) {
      const e = err as Error;
      setStatusMsg('Error: ' + e.message);
    } finally {
      setSaving(false);
    }
  };

  const handleSyncNow = async () => {
    setSyncing(true);
    try {
      const res = await integrationApi.syncPlatforms(user?.userId || 'ramanred');
      setStatusMsg(res.message || 'Platforms successfully synchronized!');
      setTimeout(() => setStatusMsg(null), 4000);
    } catch (err: unknown) {
      const e = err as Error;
      setStatusMsg('Sync error: ' + e.message);
    } finally {
      setSyncing(false);
    }
  };

  return (
    <div style={{
      position: 'fixed', inset: 0, zIndex: 60,
      background: 'rgba(0,0,0,0.8)', backdropFilter: 'blur(6px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20,
    }} onClick={onClose}>
      <div style={{
        background: 'var(--color-bg-card)',
        border: '1px solid var(--color-border)',
        borderRadius: 18,
        maxWidth: 620, width: '100%',
        maxHeight: '90vh', overflowY: 'auto',
        padding: 28, boxShadow: '0 25px 50px -12px rgba(0,0,0,0.8)',
      }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              padding: 8, borderRadius: 10,
              background: 'rgba(59, 130, 246, 0.15)', color: '#60a5fa',
              border: '1px solid rgba(59, 130, 246, 0.3)',
            }}>
              <Settings size={20} />
            </div>
            <div>
              <h3 style={{ fontSize: 17, fontWeight: 700, color: '#fff', margin: 0 }}>
                Platform Tokens & Integrations
              </h3>
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: '2px 0 0 0' }}>
                Connect your GitHub PAT, target repository, and Jenkins credentials
              </p>
            </div>
          </div>
          <button onClick={onClose} className="btn btn-ghost" style={{ padding: 6 }}>
            <X size={18} />
          </button>
        </div>

        {statusMsg && (
          <div style={{
            display: 'flex', alignItems: 'center', gap: 8,
            padding: '10px 14px', borderRadius: 8,
            background: 'rgba(16, 185, 129, 0.12)', border: '1px solid rgba(16, 185, 129, 0.3)',
            color: '#34d399', fontSize: 13, marginBottom: 16,
          }}>
            <CheckCircle2 size={16} />
            <span>{statusMsg}</span>
          </div>
        )}

        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>
          {/* ── GitHub Section ───────────────────────────── */}
          <div style={{
            padding: 16, borderRadius: 12,
            background: '#090d16', border: '1px solid var(--color-border)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 14, fontWeight: 700, color: '#fff' }}>GitHub Integration</span>
              </div>
              <span style={{
                display: 'inline-flex', alignItems: 'center', gap: 4,
                fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 6,
                background: 'rgba(16, 185, 129, 0.15)', color: '#34d399',
              }}>
                <CheckCircle2 size={12} /> {config.githubStatus ?? 'CONNECTED'}
              </span>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 12, marginBottom: 12 }}>
              <div>
                <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                  GitHub Repository (`owner/repo`)
                </label>
                <input
                  type="text"
                  value={config.githubRepo || ''}
                  onChange={(e) => setConfig({ ...config, githubRepo: e.target.value })}
                  placeholder="e.g. RamanRed/SRE-DETECTION"
                  style={{
                    width: '100%', padding: '8px 12px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                  Default Branch
                </label>
                <input
                  type="text"
                  value={config.githubBranch || 'master'}
                  onChange={(e) => setConfig({ ...config, githubBranch: e.target.value })}
                  placeholder="master"
                  style={{
                    width: '100%', padding: '8px 12px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>
            </div>

            <div>
              <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                GitHub Personal Access Token (PAT)
              </label>
              <div style={{ position: 'relative' }}>
                <Key size={14} style={{ position: 'absolute', left: 10, top: 10, color: 'var(--color-text-muted)' }} />
                <input
                  type="password"
                  value={config.githubToken || ''}
                  onChange={(e) => setConfig({ ...config, githubToken: e.target.value })}
                  placeholder={config.githubTokenConfigured ? '•••••••••••••••• (Configured)' : 'ghp_xxxxxxxxxxxxxxxxxxxx'}
                  style={{
                    width: '100%', padding: '8px 12px 8px 32px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>
              <p style={{ fontSize: 11, color: 'var(--color-text-muted)', margin: '4px 0 0 0' }}>
                Used for fetching commit diffs, workflow runs, and repository telemetry.
              </p>
            </div>
          </div>

          {/* ── Jenkins CI Section ───────────────────────────── */}
          <div style={{
            padding: 16, borderRadius: 12,
            background: '#090d16', border: '1px solid var(--color-border)',
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 14, fontWeight: 700, color: '#fff' }}>Jenkins CI Integration</span>
              </div>
              <span style={{
                display: 'inline-flex', alignItems: 'center', gap: 4,
                fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 6,
                background: 'rgba(16, 185, 129, 0.15)', color: '#34d399',
              }}>
                <CheckCircle2 size={12} /> {config.jenkinsStatus ?? 'CONNECTED'}
              </span>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '2fr 1fr', gap: 12, marginBottom: 12 }}>
              <div>
                <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                  Jenkins Server URL
                </label>
                <input
                  type="text"
                  value={config.jenkinsUrl || ''}
                  onChange={(e) => setConfig({ ...config, jenkinsUrl: e.target.value })}
                  placeholder="http://16.16.175.206:8080"
                  style={{
                    width: '100%', padding: '8px 12px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                  Job / Pipeline Name
                </label>
                <input
                  type="text"
                  value={config.jenkinsJobName || 're-copilot-pipeline'}
                  onChange={(e) => setConfig({ ...config, jenkinsJobName: e.target.value })}
                  placeholder="re-copilot-pipeline"
                  style={{
                    width: '100%', padding: '8px 12px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
              <div>
                <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                  Jenkins Username
                </label>
                <input
                  type="text"
                  value={config.jenkinsUsername || ''}
                  onChange={(e) => setConfig({ ...config, jenkinsUsername: e.target.value })}
                  placeholder="admin"
                  style={{
                    width: '100%', padding: '8px 12px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>

              <div>
                <label style={{ display: 'block', fontSize: 11, fontWeight: 600, color: 'var(--color-text-muted)', marginBottom: 4 }}>
                  Jenkins API Token
                </label>
                <input
                  type="password"
                  value={config.jenkinsApiToken || ''}
                  onChange={(e) => setConfig({ ...config, jenkinsApiToken: e.target.value })}
                  placeholder={config.jenkinsTokenConfigured ? '••••••••••••••••' : '11abcdef123456789...'}
                  style={{
                    width: '100%', padding: '8px 12px', borderRadius: 8,
                    background: 'rgba(255,255,255,0.04)', border: '1px solid var(--color-border)',
                    color: '#fff', fontSize: 13, outline: 'none',
                  }}
                />
              </div>
            </div>
          </div>

          {/* Action buttons */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: 10 }}>
            <button
              type="button"
              onClick={handleSyncNow}
              disabled={syncing}
              className="btn btn-ghost"
              style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 16px', fontSize: 13 }}
            >
              <RefreshCw size={14} style={{ animation: syncing ? 'spin 1s linear infinite' : 'none' }} />
              {syncing ? 'Syncing...' : 'Test & Sync Now'}
            </button>

            <button
              type="submit"
              disabled={saving}
              className="btn btn-primary"
              style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '10px 20px', fontSize: 13 }}
            >
              <Check size={16} />
              {saving ? 'Saving...' : 'Save & Connect'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
