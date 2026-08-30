import { useState, useEffect } from 'react';
import { integrationApi, pipelineApi } from '../api/incidentApi';
import type { DoraMetrics, PipelineBuild, PlatformIntegrationConfig } from '../types';
import {
  GitCommit, CheckCircle2, XCircle,
  ShieldCheck, ShieldAlert, Activity, RefreshCw,
  Play, Check, ExternalLink, GitBranch,
  Terminal, X, Copy, CheckCheck, Settings
} from 'lucide-react';

interface Props {
  integrationConfig?: PlatformIntegrationConfig;
  onOpenIntegrationSettings?: () => void;
}

function safeHTTPURL(raw: string | undefined): string {
  if (!raw) return '';
  try {
    const parsed = new URL(raw);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
      ? parsed.toString().replace(/\/$/, '')
      : '';
  } catch {
    return '';
  }
}

function repositoryLink(
  base: string,
  provider: PlatformIntegrationConfig['repositoryProvider'],
  kind: 'commit' | 'branch',
  value: string,
): string {
  if (!base) return '#';
  const encoded = encodeURIComponent(value);
  if (provider === 'GITLAB') return `${base}/-/${kind === 'commit' ? 'commit' : 'tree'}/${encoded}`;
  if (provider === 'BITBUCKET') return `${base}/${kind === 'commit' ? 'commits' : 'src'}/${encoded}`;
  return `${base}/${kind === 'commit' ? 'commit' : 'tree'}/${encoded}`;
}

function BuildStatusBadge({ status }: { status: PipelineBuild['status'] }) {
  if (status === 'SUCCESS') {
    return (
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '2px 10px', borderRadius: 12, background: 'rgba(16,185,129,0.12)', color: '#34d399', fontSize: 11, fontWeight: 600 }}>
        <CheckCircle2 size={13} /> SUCCESS
      </span>
    );
  }
  if (status === 'RUNNING' || status === 'QUEUED') {
    return (
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '2px 10px', borderRadius: 12, background: 'rgba(59,130,246,0.12)', color: '#60a5fa', fontSize: 11, fontWeight: 600 }}>
        <RefreshCw size={13} /> {status}
      </span>
    );
  }
  if (status === 'UNSTABLE') {
    return (
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '2px 10px', borderRadius: 12, background: 'rgba(245,158,11,0.12)', color: '#fbbf24', fontSize: 11, fontWeight: 600 }}>
        <ShieldAlert size={13} /> UNSTABLE
      </span>
    );
  }
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, padding: '2px 10px', borderRadius: 12, background: 'rgba(239,68,68,0.12)', color: '#f87171', fontSize: 11, fontWeight: 600 }}>
      <XCircle size={13} /> {status}
    </span>
  );
}

function DoraStatusBadge({ live }: { live: boolean }) {
  return (
    <span style={{
      fontSize: 10,
      fontWeight: 700,
      padding: '2px 8px',
      borderRadius: 12,
      background: live ? 'rgba(16, 185, 129, 0.15)' : 'rgba(100, 116, 139, 0.15)',
      color: live ? '#34d399' : '#94a3b8',
    }}>
      {live ? 'LIVE' : 'EMPTY'}
    </span>
  );
}

export default function PipelineHub({ integrationConfig, onOpenIntegrationSettings }: Props) {
  const [metrics, setMetrics] = useState<DoraMetrics | null>(null);
  const [builds, setBuilds] = useState<PipelineBuild[]>([]);
  const [selectedBuild, setSelectedBuild] = useState<PipelineBuild | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [simulating, setSimulating] = useState(false);
  const [copied, setCopied] = useState(false);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [messageIsError, setMessageIsError] = useState(false);

  const provider = integrationConfig?.repositoryProvider || 'GITHUB';
  const engine = integrationConfig?.pipelineEngine || 'JENKINS';
  const legacyRepositoryURL = integrationConfig?.githubRepo
    ? `https://github.com/${integrationConfig.githubRepo}`
    : '';
  const repositoryURL = safeHTTPURL(integrationConfig?.repositoryUrl || legacyRepositoryURL);
  const repoName = repositoryURL
    ? repositoryURL.replace(/^https?:\/\//, '')
    : integrationConfig?.githubRepo || 'No repository connected';
  const defaultBranch = integrationConfig?.targetBranch || integrationConfig?.githubBranch || 'main';
  const ciBase = safeHTTPURL(integrationConfig?.ciBaseUrl || integrationConfig?.jenkinsUrl);
  const pipelineJob = integrationConfig?.jobName || integrationConfig?.jenkinsJobName || 'Not configured';
  const integrationConnected = integrationConfig?.status === 'CONNECTED'
    || (integrationConfig?.githubStatus === 'CONNECTED' && integrationConfig?.jenkinsStatus === 'CONNECTED');
  const ciJobURL = engine === 'JENKINS' && ciBase
    ? `${ciBase}/job/${encodeURIComponent(pipelineJob)}`
    : ciBase;
  const metricsHaveData = Boolean(metrics?.dataAvailable);
  const doraFreshnessSource = metrics?.asOf
    || metrics?.calculatedAt
    || metrics?.generatedAt
    || metrics?.recentBuilds?.[0]?.updatedAt
    || metrics?.recentBuilds?.[0]?.createdAt;
  const doraFreshnessDate = doraFreshnessSource ? new Date(doraFreshnessSource) : null;
  const doraFreshness = doraFreshnessDate && !Number.isNaN(doraFreshnessDate.getTime())
    ? doraFreshnessDate.toLocaleString()
    : null;

  const fetchPipelineData = async () => {
    try {
      const data = await pipelineApi.getDoraMetrics();
      setMetrics(data);
      if (data.recentBuilds) {
        setBuilds(data.recentBuilds);
      }
    } catch (err) {
      console.error('Failed to load CI/CD metrics', err);
    }
  };

  useEffect(() => {
    fetchPipelineData();
  }, []);

  const handleSync = async () => {
    setSyncing(true);
    try {
      if (!integrationConfig?.userId) throw new Error('Connect a repository and CI engine first.');
      const result = await integrationApi.syncPlatforms(integrationConfig.userId);
      await fetchPipelineData();
      setMessageIsError(!result.success);
      setSuccessMsg(result.message || `Polling cycle completed for ${repoName}.`);
      setTimeout(() => setSuccessMsg(null), 4000);
    } catch (err) {
      setMessageIsError(true);
      setSuccessMsg((err as Error).message || 'Polling failed.');
      setTimeout(() => setSuccessMsg(null), 4000);
    } finally {
      setSyncing(false);
    }
  };

  const handleSimulateWebhook = async () => {
    setSimulating(true);
    try {
      const newBuildNumber = (builds[0]?.buildNumber ?? 30) + 1;
      const commitSha = '2e81b09';
      await pipelineApi.sendWebhook({
        pipelineName: pipelineJob,
        buildNumber: newBuildNumber,
        ciTool: engine,
        status: 'SUCCESS',
        gitCommit: commitSha,
        gitBranch: defaultBranch,
        author: integrationConfig?.username || 'RamanRed',
        commitMessage: 'fix: add --validate=false to kubectl apply to bypass OpenAPI schema download',
        durationSeconds: Math.floor(Math.random() * 30) + 110,
        testsPassed: 45,
        testsFailed: 0,
        vulnerabilitiesDetected: 0,
        environment: 'production',
        buildUrl: ciJobURL ? `${ciJobURL}/${newBuildNumber}/` : undefined,
        logSnippet: `Jenkins Pipeline Build #${newBuildNumber} execution:
[Stage 1] Checkout from ${repositoryLink(repositoryURL, provider, 'commit', commitSha)} SUCCESS
[Stage 2] Go format, vet, race tests, and static binary build SUCCESS.
[Stage 3] Frontend lint, typecheck, and production build SUCCESS.
[Stage 4] SonarCloud analysis uploaded successfully.
[Stage 5] Container builds and Trivy CRITICAL security gate SUCCESS.
[Stage 6] Immutable image rollout and in-cluster smoke checks SUCCESS.`,
      });
      setMessageIsError(false);
      setSuccessMsg(`Sample build #${newBuildNumber} recorded for ${repoName}.`);
      setTimeout(() => setSuccessMsg(null), 4000);
      await fetchPipelineData();
    } catch (err) {
      console.error('Simulation failed', err);
    } finally {
      setSimulating(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
      {/* ── Top Header & Actions ─────────────────────────────── */}
      <div style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 16,
        padding: '20px 24px',
        background: 'var(--color-bg-card)',
        borderRadius: 16,
        border: '1px solid var(--color-border)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
          <div style={{
            padding: 10,
            borderRadius: 12,
            background: 'rgba(59, 130, 246, 0.15)',
            color: '#60a5fa',
            border: '1px solid rgba(59, 130, 246, 0.25)',
          }}>
            <Activity size={22} />
          </div>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <h2 style={{ fontSize: 18, fontWeight: 700, color: 'var(--color-text-primary)', margin: 0 }}>
                CI/CD Pipeline Telemetry & DORA Metrics
              </h2>
              <span style={{
                fontSize: 11, fontWeight: 600, padding: '2px 8px', borderRadius: 6,
                background: 'rgba(99, 102, 241, 0.15)', color: '#818cf8', border: '1px solid rgba(99, 102, 241, 0.3)',
              }}>
                {repoName} ({defaultBranch})
              </span>
            </div>
            <p style={{ fontSize: 13, color: 'var(--color-text-muted)', margin: '4px 0 0 0' }}>
              {integrationConnected
                ? `${provider} repository and ${engine.replace('_', ' ')} with autonomous polling`
                : 'Connect a repository and CI/CD engine to enable autonomous polling'}
            </p>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          {onOpenIntegrationSettings && (
            <button
              onClick={onOpenIntegrationSettings}
              className="btn btn-ghost"
              title="Configure repository and CI/CD credentials"
              style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '8px 14px', fontSize: 13 }}
            >
              <Settings size={14} />
              <span>Configure connections</span>
            </button>
          )}

          <button
            onClick={handleSync}
            disabled={syncing || !integrationConnected}
            className="btn btn-ghost"
            title="Poll the connected repository and CI/CD engine now"
            style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 16px', fontSize: 13 }}
          >
            <RefreshCw size={14} style={{ animation: syncing ? 'spin 1s linear infinite' : 'none' }} />
            {syncing ? 'Polling...' : 'Poll now'}
          </button>

          {import.meta.env.DEV && (
            <button
              onClick={handleSimulateWebhook}
              disabled={simulating}
              className="btn btn-primary"
              title="Record a sample webhook event in local development"
              style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 16px', fontSize: 13 }}
            >
              <Play size={14} fill="#fff" />
              {simulating ? 'Recording...' : 'Record sample build'}
            </button>
          )}
        </div>
      </div>

      {successMsg && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '12px 16px',
          background: messageIsError ? 'rgba(239,68,68,.12)' : 'rgba(16,185,129,.12)',
          border: messageIsError ? '1px solid rgba(239,68,68,.3)' : '1px solid rgba(16,185,129,.3)',
          color: messageIsError ? '#f87171' : '#34d399',
          borderRadius: 10,
          fontSize: 13,
          fontWeight: 500,
        }}>
          {messageIsError ? <ShieldAlert size={16} /> : <Check size={16} />}
          <span>{successMsg}</span>
        </div>
      )}

      {/* ── DORA Metrics Grid ─────────────────────────────────── */}
      <div
        role="status"
        aria-live="polite"
        style={{ margin: '-10px 2px -12px', color: 'var(--color-text-muted)', fontSize: 11 }}
      >
        {metricsHaveData
          ? `DORA data: LIVE · ${doraFreshness ? `latest build telemetry ${doraFreshness}` : 'freshness unavailable'}`
          : 'DORA data: EMPTY · no qualifying pipeline builds in the last 7 days'}
      </div>
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))',
        gap: 16,
      }}>
        {/* Metric 1: Deployment Frequency */}
        <div style={{
          padding: 20,
          borderRadius: 14,
          background: 'var(--color-bg-card)',
          border: '1px solid var(--color-border)',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Deployment Frequency
            </span>
            <DoraStatusBadge live={metricsHaveData} />
          </div>
          <div style={{ fontSize: 24, fontWeight: 800, color: '#fff', marginBottom: 4 }}>
            {metricsHaveData ? metrics?.deploymentFrequency : '—'}
          </div>
          <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: 0 }}>Automated builds deploying to production</p>
        </div>

        {/* Metric 2: Lead Time */}
        <div style={{
          padding: 20,
          borderRadius: 14,
          background: 'var(--color-bg-card)',
          border: '1px solid var(--color-border)',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Lead Time For Changes
            </span>
            <DoraStatusBadge live={metricsHaveData} />
          </div>
          <div style={{ fontSize: 24, fontWeight: 800, color: '#fff', marginBottom: 4 }}>
            {metricsHaveData ? metrics?.leadTimeForChanges : '—'}
          </div>
          <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: 0 }}>Commit to production deployment</p>
        </div>

        {/* Metric 3: Change Failure Rate */}
        <div style={{
          padding: 20,
          borderRadius: 14,
          background: 'var(--color-bg-card)',
          border: '1px solid var(--color-border)',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Change Failure Rate
            </span>
            <DoraStatusBadge live={metricsHaveData} />
          </div>
          <div style={{ fontSize: 24, fontWeight: 800, color: '#34d399', marginBottom: 4 }}>
            {metricsHaveData ? `${metrics?.changeFailureRate}%` : '—'}
          </div>
          <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: 0 }}>Deployments requiring rollback or hotfix</p>
        </div>

        {/* Metric 4: MTTR */}
        <div style={{
          padding: 20,
          borderRadius: 14,
          background: 'var(--color-bg-card)',
          border: '1px solid var(--color-border)',
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <span style={{ fontSize: 11, fontWeight: 700, color: 'var(--color-text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
              Mean Time To Recovery
            </span>
            <DoraStatusBadge live={metricsHaveData} />
          </div>
          <div style={{ fontSize: 24, fontWeight: 800, color: '#fff', marginBottom: 4 }}>
            {metricsHaveData ? metrics?.meanTimeToRecovery : '—'}
          </div>
          <p style={{ fontSize: 12, color: 'var(--color-text-muted)', margin: 0 }}>From incident detection to resolution</p>
        </div>
      </div>

      {/* ── Connected CI Pipelines Status ─────────────────────── */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
        gap: 16,
      }}>
        <a
          href={ciJobURL || '#'}
          target="_blank"
          rel="noopener noreferrer"
          style={{ textDecoration: 'none' }}
          title={ciJobURL ? `Open ${engine} pipeline (${pipelineJob})` : 'CI/CD connection is not configured'}
        >
          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: 16, borderRadius: 12, background: 'var(--color-bg-card)', border: '1px solid var(--color-border)',
            cursor: 'pointer', transition: 'all 0.2s',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ width: 10, height: 10, borderRadius: '50%', background: integrationConnected ? '#10b981' : '#64748b' }}></div>
              <div>
                <div style={{ fontSize: 14, fontWeight: 600, color: '#fff', display: 'flex', alignItems: 'center', gap: 6 }}>
                  {engine.replace('_', ' ')} <ExternalLink size={12} color="var(--color-text-muted)" />
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>Pipeline: {pipelineJob}</div>
              </div>
            </div>
            <span style={{ fontSize: 11, fontWeight: 600, padding: '3px 10px', borderRadius: 6, background: integrationConnected ? 'rgba(16, 185, 129, 0.12)' : 'rgba(100,116,139,.12)', color: integrationConnected ? '#34d399' : '#94a3b8' }}>{integrationConnected ? 'CONNECTED' : 'DISCONNECTED'}</span>
          </div>
        </a>

        <a
          href={repositoryURL || '#'}
          target="_blank"
          rel="noopener noreferrer"
          style={{ textDecoration: 'none' }}
          title={repositoryURL ? `Open ${provider} repository ${repoName}` : 'Repository is not configured'}
        >
          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: 16, borderRadius: 12, background: 'var(--color-bg-card)', border: '1px solid var(--color-border)',
            cursor: 'pointer', transition: 'all 0.2s',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ width: 10, height: 10, borderRadius: '50%', background: integrationConnected ? '#10b981' : '#64748b' }}></div>
              <div>
                <div style={{ fontSize: 14, fontWeight: 600, color: '#fff', display: 'flex', alignItems: 'center', gap: 6 }}>
                  {provider} Repository <ExternalLink size={12} color="var(--color-text-muted)" />
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>{repoName}</div>
              </div>
            </div>
            <span style={{ fontSize: 11, fontWeight: 600, padding: '3px 10px', borderRadius: 6, background: integrationConnected ? 'rgba(16, 185, 129, 0.12)' : 'rgba(100,116,139,.12)', color: integrationConnected ? '#34d399' : '#94a3b8' }}>{integrationConnected ? 'ACTIVE' : 'INACTIVE'}</span>
          </div>
        </a>

        <a
          href="/api/v1/ci/metrics/dora"
          target="_blank"
          rel="noopener noreferrer"
          style={{ textDecoration: 'none' }}
          title="Click to inspect raw DORA JSON API"
        >
          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: 16, borderRadius: 12, background: 'var(--color-bg-card)', border: '1px solid var(--color-border)',
            cursor: 'pointer', transition: 'all 0.2s',
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div style={{ width: 10, height: 10, borderRadius: '50%', background: '#3b82f6' }}></div>
              <div>
                <div style={{ fontSize: 14, fontWeight: 600, color: '#fff', display: 'flex', alignItems: 'center', gap: 6 }}>
                  Webhook Ingestion <ExternalLink size={12} color="var(--color-text-muted)" />
                </div>
                <div style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>POST /api/v1/ci/webhook</div>
              </div>
            </div>
            <span style={{ fontSize: 11, fontWeight: 600, padding: '3px 10px', borderRadius: 6, background: 'rgba(59, 130, 246, 0.12)', color: '#60a5fa' }}>READY</span>
          </div>
        </a>
      </div>

      {/* ── Recent Pipeline Builds Table ──────────────────────── */}
      <div style={{
        borderRadius: 14,
        background: 'var(--color-bg-card)',
        border: '1px solid var(--color-border)',
        overflow: 'hidden',
      }}>
        <div style={{
          padding: '16px 20px',
          borderBottom: '1px solid var(--color-border)',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <h3 style={{ fontSize: 15, fontWeight: 700, color: '#fff', margin: 0 }}>Recent CI/CD Pipeline Executions</h3>
          <span style={{ fontSize: 12, color: 'var(--color-text-muted)' }}>Select a row to inspect build logs & trace sources</span>
        </div>

        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13, textAlign: 'left' }}>
            <thead>
              <tr style={{ background: 'rgba(0,0,0,0.2)', borderBottom: '1px solid var(--color-border)' }}>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Build</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Pipeline</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Tool</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Status</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Commit / Branch</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Tests</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Security Scan</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Duration</th>
                <th style={{ padding: '12px 18px', color: 'var(--color-text-muted)', fontSize: 11, textTransform: 'uppercase' }}>Source Link</th>
              </tr>
            </thead>
            <tbody>
              {builds.map((b) => {
                const commitLink = repositoryLink(repositoryURL, provider, 'commit', b.gitCommit);
                const branchLink = repositoryLink(repositoryURL, provider, 'branch', b.gitBranch || defaultBranch);
                const ciLink = safeHTTPURL(b.buildUrl) || (ciJobURL ? `${ciJobURL}/${b.buildNumber}/` : '#');

                return (
                  <tr
                    key={b.id}
                    onClick={() => setSelectedBuild(b)}
                    onKeyDown={(event) => {
                      if (event.target !== event.currentTarget) return;
                      if (event.key === 'Enter' || event.key === ' ') {
                        event.preventDefault();
                        setSelectedBuild(b);
                      }
                    }}
                    role="button"
                    tabIndex={0}
                    aria-haspopup="dialog"
                    aria-label={`Open details for ${b.pipelineName} build ${b.buildNumber}`}
                    style={{ borderBottom: '1px solid var(--color-border)', cursor: 'pointer', transition: 'background 0.15s' }}
                    className="hover-row build-row"
                  >
                    <td style={{ padding: '14px 18px', fontWeight: 700, color: '#fff' }}>
                      <span title={`View Build #${b.buildNumber} details`}>#{b.buildNumber}</span>
                    </td>
                    <td style={{ padding: '14px 18px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
                      <div>{b.pipelineName}</div>
                      {b.commitMessage && (
                        <div style={{ fontSize: 11, color: 'var(--color-text-muted)', maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {b.commitMessage}
                        </div>
                      )}
                    </td>
                    <td style={{ padding: '14px 18px' }}>
                      <span
                        title={b.ciTool === 'JENKINS' ? 'Executed on Jenkins CI Server' : 'Executed on GitHub Actions Runner'}
                        style={{ padding: '3px 8px', borderRadius: 6, background: 'rgba(255,255,255,0.06)', fontSize: 11, color: 'var(--color-text-secondary)' }}
                      >
                        {b.ciTool}
                      </span>
                    </td>
                    <td style={{ padding: '14px 18px' }}>
                      <BuildStatusBadge status={b.status} />
                    </td>
                    <td style={{ padding: '14px 18px', fontFamily: 'monospace' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <a
                          href={commitLink}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={(e) => e.stopPropagation()}
                          title={`Open commit ${b.gitCommit} on ${provider}`}
                          style={{ color: '#60a5fa', textDecoration: 'none', display: 'flex', alignItems: 'center', gap: 4 }}
                        >
                          <GitCommit size={13} />
                          <span>{b.gitCommit}</span>
                          <ExternalLink size={10} />
                        </a>
                        <a
                          href={branchLink}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={(e) => e.stopPropagation()}
                          title={`Open branch ${b.gitBranch} on ${provider}`}
                          style={{ color: 'var(--color-text-muted)', fontFamily: 'sans-serif', textDecoration: 'none' }}
                        >
                          ({b.gitBranch})
                        </a>
                      </div>
                    </td>
                    <td style={{ padding: '14px 18px' }}>
                      <span title="Automated Unit & Integration test results" style={{ color: '#34d399', fontWeight: 600 }}>
                        {b.testsPassed} passed
                      </span>
                      {b.testsFailed > 0 && <span style={{ color: '#f87171', fontWeight: 600, marginLeft: 4 }}>/ {b.testsFailed} failed</span>}
                    </td>
                    <td style={{ padding: '14px 18px' }}>
                      {b.vulnerabilitiesDetected === 0 ? (
                        <span title="Aquasec Trivy container scan passed with 0 critical/high CVEs" style={{ display: 'inline-flex', alignItems: 'center', gap: 4, color: '#34d399', fontSize: 12 }}>
                          <ShieldCheck size={14} /> 0 CVEs (Clean)
                        </span>
                      ) : (
                        <span title="Trivy security scan identified vulnerabilities" style={{ display: 'inline-flex', alignItems: 'center', gap: 4, color: '#fbbf24', fontSize: 12 }}>
                          <ShieldAlert size={14} /> {b.vulnerabilitiesDetected} CVEs
                        </span>
                      )}
                    </td>
                    <td style={{ padding: '14px 18px', color: 'var(--color-text-muted)' }}>{b.durationSeconds}s</td>
                    <td style={{ padding: '14px 18px' }}>
                      <a
                        href={ciLink}
                        target="_blank"
                        rel="noopener noreferrer"
                        onClick={(e) => e.stopPropagation()}
                        className="btn btn-ghost"
                        title={`Open Build #${b.buildNumber} in ${b.ciTool}`}
                        style={{ padding: '4px 10px', fontSize: 11, display: 'inline-flex', alignItems: 'center', gap: 4, textDecoration: 'none' }}
                      >
                        <span>{b.ciTool.replace('_', ' ')} ↗</span>
                      </a>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* ── Build Detail Drawer / Modal ────────────────────────── */}
      {selectedBuild && (
        <div style={{
          position: 'fixed', inset: 0, zIndex: 50,
          background: 'rgba(0,0,0,0.75)', backdropFilter: 'blur(4px)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 20,
        }} onClick={() => setSelectedBuild(null)}>
          <div style={{
            background: 'var(--color-bg-card)',
            border: '1px solid var(--color-border)',
            borderRadius: 16,
            maxWidth: 680, width: '100%',
            maxHeight: '90vh', overflowY: 'auto',
            padding: 24, boxShadow: '0 25px 50px -12px rgba(0,0,0,0.7)',
          }} onClick={(e) => e.stopPropagation()}>
            {/* Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: 20 }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 6 }}>
                  <span style={{ fontSize: 18, fontWeight: 800, color: '#fff' }}>
                    Build #{selectedBuild.buildNumber}
                  </span>
                  <span style={{ padding: '2px 8px', borderRadius: 6, background: 'rgba(255,255,255,0.08)', fontSize: 12, color: 'var(--color-text-secondary)' }}>
                    {selectedBuild.pipelineName}
                  </span>
                  <BuildStatusBadge status={selectedBuild.status} />
                </div>
                <p style={{ fontSize: 13, color: 'var(--color-text-muted)', margin: 0 }}>
                  Executed by <strong>{selectedBuild.author}</strong> on {new Date(selectedBuild.createdAt).toLocaleString()}
                </p>
              </div>
              <button onClick={() => setSelectedBuild(null)} className="btn btn-ghost" style={{ padding: 6 }}>
                <X size={18} />
              </button>
            </div>

            {/* Quick Link Buttons to Original Sources */}
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 10, marginBottom: 20 }}>
              <a
                href={repositoryLink(repositoryURL, provider, 'commit', selectedBuild.gitCommit)}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-primary"
                style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12, padding: '8px 14px', textDecoration: 'none' }}
              >
                <GitCommit size={14} /> View Commit on {provider} ({selectedBuild.gitCommit}) <ExternalLink size={12} />
              </a>

              <a
                href={repositoryLink(repositoryURL, provider, 'branch', selectedBuild.gitBranch || defaultBranch)}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-ghost"
                style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12, padding: '8px 14px', textDecoration: 'none' }}
              >
                <GitBranch size={14} /> Branch: {selectedBuild.gitBranch || defaultBranch} <ExternalLink size={12} />
              </a>

              <a
                href={safeHTTPURL(selectedBuild.buildUrl) || (ciJobURL ? `${ciJobURL}/${selectedBuild.buildNumber}/` : '#')}
                target="_blank"
                rel="noopener noreferrer"
                className="btn btn-ghost"
                style={{ display: 'inline-flex', alignItems: 'center', gap: 6, fontSize: 12, padding: '8px 14px', textDecoration: 'none' }}
              >
                <Activity size={14} /> Open in {selectedBuild.ciTool} Console <ExternalLink size={12} />
              </a>
            </div>

            {/* Commit message & Metrics */}
            <div style={{
              display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
              gap: 12, marginBottom: 20,
            }}>
              <div style={{ padding: 12, borderRadius: 10, background: 'var(--color-bg-surface)', border: '1px solid var(--color-border)' }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Duration</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: '#fff', marginTop: 2 }}>{selectedBuild.durationSeconds}s</div>
              </div>
              <div style={{ padding: 12, borderRadius: 10, background: 'var(--color-bg-surface)', border: '1px solid var(--color-border)' }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Unit Tests</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: '#34d399', marginTop: 2 }}>{selectedBuild.testsPassed} Passed</div>
              </div>
              <div style={{ padding: 12, borderRadius: 10, background: 'var(--color-bg-surface)', border: '1px solid var(--color-border)' }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Security Scan</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: '#34d399', marginTop: 2 }}>{selectedBuild.vulnerabilitiesDetected} CVEs</div>
              </div>
              <div style={{ padding: 12, borderRadius: 10, background: 'var(--color-bg-surface)', border: '1px solid var(--color-border)' }}>
                <div style={{ fontSize: 11, color: 'var(--color-text-muted)' }}>Environment</div>
                <div style={{ fontSize: 15, fontWeight: 700, color: '#60a5fa', marginTop: 2 }}>{selectedBuild.environment}</div>
              </div>
            </div>

            {/* Build Log Snippet */}
            {selectedBuild.logSnippet && (
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                  <div style={{ fontSize: 12, fontWeight: 700, color: 'var(--color-text-muted)', textTransform: 'uppercase' }}>
                    <Terminal size={12} style={{ display: 'inline', marginRight: 4 }} /> Build Execution Logs
                  </div>
                  <button
                    onClick={() => copyToClipboard(selectedBuild.logSnippet!)}
                    className="btn btn-ghost"
                    style={{ padding: '3px 8px', fontSize: 11, display: 'flex', alignItems: 'center', gap: 4 }}
                  >
                    {copied ? <CheckCheck size={12} color="#10b981" /> : <Copy size={12} />}
                    {copied ? 'Copied' : 'Copy Logs'}
                  </button>
                </div>
                <pre style={{
                  padding: 14, borderRadius: 10,
                  background: '#090d16', border: '1px solid var(--color-border)',
                  color: '#93c5fd', fontSize: 12, fontFamily: 'monospace',
                  whiteSpace: 'pre-wrap', maxHeight: 200, overflowY: 'auto',
                }}>
                  {selectedBuild.logSnippet}
                </pre>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
