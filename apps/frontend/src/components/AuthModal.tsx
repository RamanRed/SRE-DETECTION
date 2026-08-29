import { useState } from 'react';
import { authApi } from '../api/incidentApi';
import type { UserProfile } from '../types';
import { Shield, User, X, Check, Lock } from 'lucide-react';

interface Props {
  onSuccess: (user: UserProfile) => void;
  onClose: () => void;
}

export default function AuthModal({ onSuccess, onClose }: Props) {
  const [username, setUsername] = useState('RamanRed');
  const [role, setRole] = useState<'SRE_LEAD' | 'DEVOPS_ENGINEER' | 'EVALUATOR'>('SRE_LEAD');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim()) {
      setError('Please enter a valid username');
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const profile = await authApi.login(username, role);
      localStorage.setItem('sre_user', JSON.stringify(profile));
      onSuccess(profile);
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || 'Login failed');
    } finally {
      setLoading(false);
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
        maxWidth: 440, width: '100%',
        padding: 28, boxShadow: '0 25px 50px -12px rgba(0,0,0,0.8)',
      }} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 20 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <div style={{
              padding: 8, borderRadius: 10,
              background: 'rgba(99, 102, 241, 0.15)', color: '#818cf8',
              border: '1px solid rgba(99, 102, 241, 0.3)',
            }}>
              <Shield size={20} />
            </div>
            <div>
              <h3 style={{ fontSize: 17, fontWeight: 700, color: '#fff', margin: 0 }}>SRE Copilot Login</h3>
              <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: '2px 0 0 0' }}>Authenticate your engineer session</p>
            </div>
          </div>
          <button onClick={onClose} className="btn btn-ghost" style={{ padding: 6 }}>
            <X size={18} />
          </button>
        </div>

        {error && (
          <div style={{
            padding: '10px 14px', borderRadius: 8,
            background: 'rgba(239, 68, 68, 0.12)', border: '1px solid rgba(239, 68, 68, 0.3)',
            color: '#f87171', fontSize: 13, marginBottom: 16,
          }}>
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div>
            <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', marginBottom: 6 }}>
              GitHub Username / Engineer Handle
            </label>
            <div style={{ position: 'relative' }}>
              <User size={16} style={{ position: 'absolute', left: 12, top: 12, color: 'var(--color-text-muted)' }} />
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="e.g. RamanRed"
                style={{
                  width: '100%', padding: '10px 12px 10px 38px', borderRadius: 10,
                  background: '#090d16', border: '1px solid var(--color-border)',
                  color: '#fff', fontSize: 14, outline: 'none',
                }}
              />
            </div>
          </div>

          <div>
            <label style={{ display: 'block', fontSize: 12, fontWeight: 600, color: 'var(--color-text-secondary)', marginBottom: 6 }}>
              Access Role
            </label>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8 }}>
              {[
                { id: 'SRE_LEAD', label: 'SRE Lead' },
                { id: 'DEVOPS_ENGINEER', label: 'DevOps' },
                { id: 'EVALUATOR', label: 'Evaluator' },
              ].map((r) => (
                <button
                  key={r.id}
                  type="button"
                  onClick={() => setRole(r.id as any)}
                  style={{
                    padding: '8px 4px', borderRadius: 8, fontSize: 12, fontWeight: 600,
                    cursor: 'pointer', textAlign: 'center', transition: 'all 0.15s',
                    background: role === r.id ? 'rgba(99, 102, 241, 0.2)' : 'rgba(255,255,255,0.04)',
                    color: role === r.id ? '#818cf8' : 'var(--color-text-muted)',
                    border: role === r.id ? '1px solid #6366f1' : '1px solid var(--color-border)',
                  }}
                >
                  {r.label}
                </button>
              ))}
            </div>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--color-text-muted)', marginTop: 4 }}>
            <Lock size={13} />
            <span>Secured via JWT SRE Session Bearer Token</span>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn btn-primary"
            style={{ width: '100%', padding: '11px', fontSize: 14, fontWeight: 600, marginTop: 6, display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 8 }}
          >
            {loading ? 'Authenticating...' : 'Enter SRE Copilot Platform'}
            <Check size={16} />
          </button>
        </form>
      </div>
    </div>
  );
}
