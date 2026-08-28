import { useState } from 'react';
import type { CreateIncidentPayload } from '../types';
import { incidentApi } from '../api/incidentApi';
import { X, Plus, Loader } from 'lucide-react';

interface Props {
  onClose: () => void;
  onCreated: () => void;
}

export default function CreateIncidentModal({ onClose, onCreated }: Props) {
  const [form, setForm] = useState<CreateIncidentPayload>({
    title: '',
    serviceName: '',
    rawLogs: '',
    firingRule: '',
    environment: 'production',
    createdBy: 'sre-engineer',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
    setForm({ ...form, [e.target.name]: e.target.value });
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      await incidentApi.create(form);
      onCreated();
      onClose();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 580 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
          <h2 style={{ fontSize: 17, fontWeight: 700 }}>Report New Incident</h2>
          <button className="btn btn-ghost" onClick={onClose} style={{ padding: '6px 10px' }}><X size={16} /></button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div>
            <label style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 6, display: 'block', textTransform: 'uppercase', letterSpacing: '0.07em' }}>
              Incident Title *
            </label>
            <input className="input" name="title" value={form.title} onChange={handleChange}
              placeholder="e.g. PostgreSQL connection pool exhausted in payment-service" required />
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div>
              <label style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 6, display: 'block', textTransform: 'uppercase', letterSpacing: '0.07em' }}>
                Service Name *
              </label>
              <input className="input" name="serviceName" value={form.serviceName} onChange={handleChange}
                placeholder="e.g. payment-service" required />
            </div>
            <div>
              <label style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 6, display: 'block', textTransform: 'uppercase', letterSpacing: '0.07em' }}>
                Environment
              </label>
              <select className="input" name="environment" value={form.environment} onChange={handleChange}>
                <option value="production">production</option>
                <option value="staging">staging</option>
                <option value="development">development</option>
              </select>
            </div>
          </div>

          <div>
            <label style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 6, display: 'block', textTransform: 'uppercase', letterSpacing: '0.07em' }}>
              Firing Alert Rule
            </label>
            <input className="input" name="firingRule" value={form.firingRule} onChange={handleChange}
              placeholder="e.g. HighErrorRate_5xx or PodCrashLoopBackOff" />
          </div>

          <div>
            <label style={{ fontSize: 12, color: 'var(--color-text-muted)', marginBottom: 6, display: 'block', textTransform: 'uppercase', letterSpacing: '0.07em' }}>
              Raw Logs / Stack Trace
            </label>
            <textarea className="input font-mono" name="rawLogs" value={form.rawLogs} onChange={handleChange}
              placeholder="Paste stack trace, error logs, or metric anomalies here for AI analysis…"
              style={{ minHeight: 160 }} />
          </div>

          {error && (
            <div style={{ padding: '10px 14px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.25)', borderRadius: 8, color: '#fca5a5', fontSize: 12 }}>
              {error}
            </div>
          )}

          <div style={{ display: 'flex', gap: 10, justifyContent: 'flex-end', paddingTop: 8, borderTop: '1px solid var(--color-border)' }}>
            <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? <Loader size={14} style={{ animation: 'spin 1s linear infinite' }} /> : <Plus size={14} />}
              {loading ? 'Creating…' : 'Create Incident'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
