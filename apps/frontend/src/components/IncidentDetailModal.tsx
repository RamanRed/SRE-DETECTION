import { useState } from 'react';
import type { Incident, TriageResult, RemediationResult } from '../types';
import { incidentApi } from '../api/incidentApi';
import {
  X, Brain, Terminal, ShieldCheck, Loader, AlertCircle,
  Copy, CheckCheck, ChevronDown, ChevronUp, Server, Clock, Hash
} from 'lucide-react';

interface Props {
  incident: Incident;
  onClose: () => void;
  onResolved: () => void;
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 24 }}>
      <h4 style={{
        fontSize: 11, fontWeight: 700, letterSpacing: '0.1em', textTransform: 'uppercase',
        color: 'var(--color-text-muted)', marginBottom: 12,
        paddingBottom: 8, borderBottom: '1px solid var(--color-border)',
      }}>{title}</h4>
      {children}
    </div>
  );
}

export default function IncidentDetailModal({ incident, onClose, onResolved }: Props) {
  const [triage, setTriage] = useState<TriageResult | null>(null);
  const [remediation, setRemediation] = useState<RemediationResult | null>(null);
  const [triageLoading, setTriageLoading] = useState(false);
  const [remediateLoading, setRemediateLoading] = useState(false);
  const [approveLoading, setApproveLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [logsExpanded, setLogsExpanded] = useState(false);

  const triggerTriage = async () => {
    setTriageLoading(true);
    setError(null);
    try {
      const result = await incidentApi.triggerTriage(incident.id);
      setTriage(result);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setTriageLoading(false);
    }
  };

  const generateRemediation = async () => {
    setRemediateLoading(true);
    setError(null);
    try {
      const result = await incidentApi.generateRemediation(incident.id);
      setRemediation(result);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setRemediateLoading(false);
    }
  };

  const approveRemediation = async () => {
    if (!remediation) return;
    setApproveLoading(true);
    setError(null);
    try {
      await incidentApi.approveRemediation(incident.id, remediation.remediationId, 'sre-engineer');
      onResolved();
      onClose();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setApproveLoading(false);
    }
  };

  const copyScript = () => {
    if (remediation?.executableScript) {
      navigator.clipboard.writeText(remediation.executableScript);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const confidencePercent = triage
    ? `${Math.round(parseFloat(triage.confidenceScore) * 100)}%`
    : null;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 740 }}>

        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24 }}>
          <div>
            <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
              <span className={`badge badge-${incident.status.toLowerCase()}`}>{incident.status}</span>
              {incident.severity && (
                <span className={`badge badge-${incident.severity.toLowerCase()}`}>{incident.severity}</span>
              )}
            </div>
            <h2 style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-text-primary)' }}>
              {incident.title}
            </h2>
          </div>
          <button className="btn btn-ghost" onClick={onClose} style={{ padding: '6px 10px' }}>
            <X size={16} />
          </button>
        </div>

        {/* Meta info */}
        <Section title="Incident Details">
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            {[
              { icon: <Hash size={13}/>, label: 'Incident ID', value: incident.id.slice(0, 18) + '…' },
              { icon: <Server size={13}/>, label: 'Service', value: incident.serviceName },
              { icon: <AlertCircle size={13}/>, label: 'Firing Rule', value: incident.firingRule ?? '—' },
              { icon: <Clock size={13}/>, label: 'Environment', value: incident.environment },
            ].map(({ icon, label, value }) => (
              <div key={label} style={{ background: 'var(--color-bg-surface)', borderRadius: 8, padding: '10px 14px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--color-text-muted)', marginBottom: 4, fontSize: 11, textTransform: 'uppercase', letterSpacing: '0.07em' }}>
                  {icon}{label}
                </div>
                <span style={{ fontSize: 13, color: 'var(--color-text-primary)', fontFamily: 'monospace' }}>{value}</span>
              </div>
            ))}
          </div>
        </Section>

        {/* Raw Logs */}
        {incident.rawLogs && (
          <Section title="Raw Logs">
            <div>
              <div className="terminal" style={{ maxHeight: logsExpanded ? 400 : 120 }}>
                {incident.rawLogs.split('\n').map((line: string, i: number) => {
                  const cls = line.includes('ERROR') || line.includes('FATAL') ? 't-error'
                            : line.includes('WARN')  ? 't-warn'
                            : line.includes('INFO')  ? 't-info'
                            : '';
                  return <div key={i} className={cls}>{line}</div>;
                })}
              </div>
              <button
                className="btn btn-ghost"
                style={{ marginTop: 6, fontSize: 11, padding: '4px 10px' }}
                onClick={() => setLogsExpanded(x => !x)}
              >
                {logsExpanded ? <ChevronUp size={12}/> : <ChevronDown size={12}/>}
                {logsExpanded ? 'Collapse' : 'Expand'} logs
              </button>
            </div>
          </Section>
        )}

        {/* AI Triage */}
        <Section title="AI Root-Cause Analysis">
          {!triage ? (
            <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
              <button className="btn btn-primary" onClick={triggerTriage} disabled={triageLoading}>
                {triageLoading ? <Loader size={14} style={{ animation: 'spin 1s linear infinite' }} /> : <Brain size={14} />}
                {triageLoading ? 'Analyzing with AI…' : 'Run AI Triage'}
              </button>
              {triageLoading && (
                <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
                  Sending logs to LLM inference engine via gRPC…
                </span>
              )}
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, animation: 'fade-in 0.4s ease' }}>
              <div style={{ background: 'rgba(99,102,241,0.08)', border: '1px solid rgba(99,102,241,0.2)', borderRadius: 10, padding: 16 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                  <span style={{ fontSize: 11, color: '#818cf8', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.08em' }}>Root Cause</span>
                  <span style={{ fontSize: 11, color: '#10b981' }}>✓ Confidence: {confidencePercent}</span>
                </div>
                <p style={{ fontSize: 13, color: 'var(--color-text-primary)', lineHeight: 1.7 }}>{triage.rootCause}</p>
              </div>

              <div style={{ background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.2)', borderRadius: 10, padding: 16 }}>
                <span style={{ fontSize: 11, color: '#fbbf24', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.08em' }}>Immediate Mitigation</span>
                <p style={{ fontSize: 13, color: 'var(--color-text-primary)', marginTop: 8 }}>{triage.immediateMitigation}</p>
              </div>

              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                <span style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Affected:</span>
                {triage.affectedComponents.map((c) => (
                  <span key={c} className="badge badge-high" style={{ fontSize: 11 }}>{c}</span>
                ))}
              </div>

              {/* Progress bar for confidence */}
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                  <span style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Confidence Score</span>
                  <span style={{ fontSize: 11, color: '#10b981' }}>{confidencePercent}</span>
                </div>
                <div className="progress-bar">
                  <div className="progress-bar-fill" style={{ width: confidencePercent ?? undefined, background: '#10b981' }} />
                </div>
              </div>
            </div>
          )}
        </Section>

        {/* Remediation Script */}
        {triage && (
          <Section title="Remediation Script">
            {!remediation ? (
              <button className="btn btn-primary" onClick={generateRemediation} disabled={remediateLoading}>
                {remediateLoading ? <Loader size={14} style={{ animation: 'spin 1s linear infinite' }} /> : <Terminal size={14} />}
                {remediateLoading ? 'Generating script…' : 'Generate Remediation Script'}
              </button>
            ) : (
              <div style={{ animation: 'fade-in 0.4s ease' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <span className="badge badge-analyzing">{remediation.scriptType}</span>
                  <button className="btn btn-ghost" onClick={copyScript} style={{ fontSize: 11, padding: '4px 10px' }}>
                    {copied ? <CheckCheck size={12} /> : <Copy size={12} />}
                    {copied ? 'Copied!' : 'Copy'}
                  </button>
                </div>
                <div className="terminal">{remediation.executableScript}</div>
                {remediation.requiresManualApproval && (
                  <div style={{ marginTop: 16, padding: '12px 16px', background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', borderRadius: 10 }}>
                    <p style={{ fontSize: 12, color: '#fca5a5', marginBottom: 10 }}>
                      ⚠️ This script requires SRE engineer approval before execution.
                    </p>
                    <button className="btn btn-success" onClick={approveRemediation} disabled={approveLoading}>
                      {approveLoading ? <Loader size={14} /> : <ShieldCheck size={14} />}
                      {approveLoading ? 'Approving…' : 'Approve & Mark Resolved'}
                    </button>
                  </div>
                )}
              </div>
            )}
          </Section>
        )}

        {/* Error */}
        {error && (
          <div style={{ padding: '12px 16px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.25)', borderRadius: 8, color: '#fca5a5', fontSize: 13 }}>
            <AlertCircle size={14} style={{ display: 'inline', marginRight: 6 }} />
            {error}
          </div>
        )}
      </div>
    </div>
  );
}
