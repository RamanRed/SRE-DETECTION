import { useEffect, useRef, useState } from 'react';
import type { Incident, TriageResult, RemediationResult, UserProfile } from '../types';
import { incidentApi } from '../api/incidentApi';
import {
  X, Brain, Terminal, ShieldCheck, Loader, AlertCircle,
  Copy, CheckCheck, ChevronDown, ChevronUp, Server, Clock, Hash
} from 'lucide-react';

interface Props {
  incident: Incident;
  appliedBy: string;
  userRole?: UserProfile['role'];
  onClose: () => void;
  onResolved: () => void;
}

const analysisPollIntervalMs = 2_500;
const analysisPollMaxAttempts = 12;

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

export default function IncidentDetailModal({ incident, appliedBy, userRole, onClose, onResolved }: Props) {
  const [triage, setTriage] = useState<TriageResult | null>(null);
  const [remediation, setRemediation] = useState<RemediationResult | null>(null);
  const [triageLoading, setTriageLoading] = useState(false);
  const [analysisPolling, setAnalysisPolling] = useState(incident.status === 'ANALYZING');
  const [analysisPollExhausted, setAnalysisPollExhausted] = useState(false);
  const [remediateLoading, setRemediateLoading] = useState(false);
  const [approveLoading, setApproveLoading] = useState(false);
  const [approvalNotice, setApprovalNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [logsExpanded, setLogsExpanded] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const onCloseRef = useRef(onClose);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  useEffect(() => {
    let cancelled = false;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    let attempts = 0;
    const shouldPoll = incident.status === 'ANALYZING';

    const loadAnalysis = async () => {
      attempts += 1;
      try {
        const result = await incidentApi.getLatestAnalysis(incident.id);
        if (cancelled) return;
        setTriage(result);
        setAnalysisPolling(false);
        setAnalysisPollExhausted(false);
      } catch {
        if (cancelled) return;
        if (shouldPoll && attempts < analysisPollMaxAttempts) {
          retryTimer = setTimeout(loadAnalysis, analysisPollIntervalMs);
          return;
        }
        setAnalysisPolling(false);
        setAnalysisPollExhausted(shouldPoll);
      }
    };

    void loadAnalysis();
    return () => {
      cancelled = true;
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, [incident.id, incident.status]);

  useEffect(() => {
    const previouslyFocused = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const focusFrame = requestAnimationFrame(() => dialogRef.current?.focus());

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onCloseRef.current();
        return;
      }
      if (event.key !== 'Tab' || !dialogRef.current) return;

      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      ));
      if (focusable.length === 0) {
        event.preventDefault();
        dialogRef.current.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      cancelAnimationFrame(focusFrame);
      document.removeEventListener('keydown', handleKeyDown);
      previouslyFocused?.focus();
    };
  }, []);

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
    if (!remediation || !appliedBy || userRole !== 'SRE_LEAD') return;
    setApproveLoading(true);
    setError(null);
    setApprovalNotice(null);
    try {
      const result = await incidentApi.approveRemediation(incident.id, remediation.remediationId, appliedBy);
      setRemediation(result);
      if (result.executionStatus === 'APPLIED' || result.incidentStatus === 'RESOLVED') {
        onResolved();
        onClose();
        return;
      }
      const status = result.executionStatus.replaceAll('_', ' ').toLowerCase();
      setApprovalNotice(
        result.executionStatus === 'APPROVED'
          ? 'Approval recorded. Execute through the controlled runbook, then verify the service and mark the remediation applied.'
          : `Remediation status: ${status}. The incident remains open until execution is applied.`,
      );
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
  const automatedAnalysisPending = incident.status === 'ANALYZING' && !triage;
  const canApprove = userRole === 'SRE_LEAD' && Boolean(appliedBy);

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div
        ref={dialogRef}
        className="modal-panel"
        onClick={(e) => e.stopPropagation()}
        style={{ maxWidth: 740, outline: 'none' }}
        role="dialog"
        aria-modal="true"
        aria-labelledby="incident-dialog-title"
        tabIndex={-1}
      >

        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 24 }}>
          <div>
            <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
              <span className={`badge badge-${incident.status.toLowerCase()}`}>{incident.status}</span>
              {incident.severity && (
                <span className={`badge badge-${incident.severity.toLowerCase()}`}>{incident.severity}</span>
              )}
            </div>
            <h2 id="incident-dialog-title" style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-text-primary)' }}>
              {incident.title}
            </h2>
          </div>
          <button className="btn btn-ghost" onClick={onClose} style={{ padding: '6px 10px' }} aria-label="Close incident details">
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
              <button className="btn btn-primary" onClick={triggerTriage} disabled={triageLoading || automatedAnalysisPending}>
                {triageLoading || analysisPolling ? <Loader size={14} style={{ animation: 'spin 1s linear infinite' }} /> : <Brain size={14} />}
                {triageLoading
                  ? 'Analyzing with AI…'
                  : automatedAnalysisPending
                    ? analysisPolling ? 'Waiting for automated triage…' : 'Automated triage in progress'
                    : 'Run AI Triage'}
              </button>
              {(triageLoading || automatedAnalysisPending) && (
                <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>
                  {analysisPollExhausted
                    ? 'Analysis is still processing. Close this dialog and check again shortly.'
                    : triageLoading
                      ? 'Sending logs to LLM inference engine via gRPC…'
                      : 'Checking for the persisted code-aware analysis…'}
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

              {triage.unifiedDiff && (
                <div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 7 }}>
                    <span style={{ fontSize: 11, color: '#60a5fa', fontWeight: 650, textTransform: 'uppercase', letterSpacing: '.07em' }}>
                      Proposed code patch
                    </span>
                    <span style={{ fontSize: 10, color: 'var(--color-text-muted)' }}>Review before applying</span>
                  </div>
                  <pre className="terminal" style={{ color: '#cbd5e1', whiteSpace: 'pre-wrap' }}>{triage.unifiedDiff}</pre>
                </div>
              )}

              {(triage.verificationPlan || triage.rollbackPlan) && (
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: 10 }}>
                  {triage.verificationPlan && (
                    <div style={{ background: 'rgba(16,185,129,.07)', border: '1px solid rgba(16,185,129,.2)', borderRadius: 9, padding: 13 }}>
                      <span style={{ fontSize: 10, color: '#34d399', fontWeight: 700, textTransform: 'uppercase' }}>Verification plan</span>
                      <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 6, whiteSpace: 'pre-wrap' }}>{triage.verificationPlan}</p>
                    </div>
                  )}
                  {triage.rollbackPlan && (
                    <div style={{ background: 'rgba(245,158,11,.07)', border: '1px solid rgba(245,158,11,.2)', borderRadius: 9, padding: 13 }}>
                      <span style={{ fontSize: 10, color: '#fbbf24', fontWeight: 700, textTransform: 'uppercase' }}>Rollback plan</span>
                      <p style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginTop: 6, whiteSpace: 'pre-wrap' }}>{triage.rollbackPlan}</p>
                    </div>
                  )}
                </div>
              )}

              {triage.citedSourcePaths && triage.citedSourcePaths.length > 0 && (
                <div style={{ display: 'flex', gap: 7, alignItems: 'center', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Source evidence:</span>
                  {triage.citedSourcePaths.map((path) => (
                    <code key={path} style={{ fontSize: 10, color: '#93c5fd', background: 'rgba(59,130,246,.1)', padding: '2px 6px', borderRadius: 5 }}>
                      {path}
                    </code>
                  ))}
                </div>
              )}

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
                {(remediation.verificationPlan || remediation.rollbackPlan) && (
                  <div style={{ display: 'grid', gap: 8, marginTop: 10 }}>
                    {remediation.verificationPlan && (
                      <p style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        <strong style={{ color: '#34d399' }}>Verify: </strong>{remediation.verificationPlan}
                      </p>
                    )}
                    {remediation.rollbackPlan && (
                      <p style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        <strong style={{ color: '#fbbf24' }}>Rollback: </strong>{remediation.rollbackPlan}
                      </p>
                    )}
                  </div>
                )}
                {remediation.requiresManualApproval && (
                  <div style={{ marginTop: 16, padding: '12px 16px', background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)', borderRadius: 10 }}>
                    <p style={{ fontSize: 12, color: '#fca5a5', marginBottom: 10 }}>
                      ⚠️ This script requires SRE engineer approval before execution.
                    </p>
                    {canApprove ? (
                      <button className="btn btn-success" onClick={approveRemediation} disabled={approveLoading}>
                        {approveLoading ? <Loader size={14} /> : <ShieldCheck size={14} />}
                        {approveLoading ? 'Approving…' : 'Approve remediation'}
                      </button>
                    ) : (
                      <p style={{ margin: 0, fontSize: 12, color: 'var(--color-text-secondary)' }}>
                        Only an authenticated SRE lead can approve this remediation.
                      </p>
                    )}
                    {approvalNotice && (
                      <p role="status" aria-live="polite" style={{ margin: '10px 0 0', fontSize: 12, color: '#93c5fd' }}>
                        {approvalNotice}
                      </p>
                    )}
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
