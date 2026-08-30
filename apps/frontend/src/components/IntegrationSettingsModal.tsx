import { useEffect, useMemo, useState } from 'react';
import { integrationApi } from '../api/incidentApi';
import type {
  IntegrationConnectPayload,
  IntegrationStatus,
  PlatformIntegrationConfig,
  UserProfile,
} from '../types';
import {
  AlertTriangle, Check, CheckCircle2, Clock3, GitBranch, Key,
  RefreshCw, Server, Settings, ShieldCheck, X,
} from 'lucide-react';

interface Props {
  user: UserProfile | null;
  onClose: () => void;
  onSaved: (config: PlatformIntegrationConfig) => void;
}

type Notice = { tone: 'success' | 'error' | 'info'; text: string };

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '9px 11px',
  borderRadius: 8,
  background: 'rgba(255,255,255,0.04)',
  border: '1px solid var(--color-border)',
  color: '#fff',
  fontSize: 13,
  outline: 'none',
};

const labelStyle: React.CSSProperties = {
  display: 'block',
  fontSize: 11,
  fontWeight: 650,
  color: 'var(--color-text-secondary)',
  marginBottom: 5,
};

const emptyConfig = (user: UserProfile | null): PlatformIntegrationConfig => ({
  userId: user?.userId || 'local-sre',
  username: user?.username || 'SRE',
  repositoryProvider: 'GITHUB',
  repositoryUrl: '',
  targetBranch: 'main',
  repositoryToken: '',
  repositoryTokenConfigured: false,
  repositoryStatus: 'DISCONNECTED',
  pipelineEngine: 'JENKINS',
  ciBaseUrl: '',
  ciUsername: '',
  ciToken: '',
  ciTokenConfigured: false,
  jobName: '',
  pollingCadence: '15_MINUTES',
  autoRebuild: true,
  autoAITriage: true,
  status: 'DISCONNECTED',
});

function normalizeCadence(value: string | undefined): PlatformIntegrationConfig['pollingCadence'] {
  switch (value?.toUpperCase()) {
    case '5M':
    case '5_MINUTES':
      return '5_MINUTES';
    case '1H':
    case '1_HOUR':
      return '1_HOUR';
    case 'DAILY':
    case '24H':
    case 'DAILY_CRON':
      return 'DAILY_CRON';
    default:
      return '15_MINUTES';
  }
}

function normalizeConfig(
  value: PlatformIntegrationConfig,
  user: UserProfile | null,
): PlatformIntegrationConfig {
  const repositoryUrl = value.repositoryUrl
    || (value.githubRepo ? 'https://github.com/' + value.githubRepo : '');
  const repositoryStatus = value.repositoryStatus || value.githubStatus || 'DISCONNECTED';
  const ciStatus = value.jenkinsStatus || 'DISCONNECTED';
  let status = value.status;
  if (!status) {
    status = repositoryStatus === 'ERROR' || ciStatus === 'ERROR'
      ? 'ERROR'
      : repositoryStatus === 'SYNCING' || ciStatus === 'SYNCING'
        ? 'SYNCING'
        : repositoryStatus === 'CONNECTED' && ciStatus === 'CONNECTED'
          ? 'CONNECTED'
          : 'DISCONNECTED';
  }

  return {
    ...emptyConfig(user),
    ...value,
    repositoryProvider: value.repositoryProvider || 'GITHUB',
    repositoryUrl,
    targetBranch: value.targetBranch || value.githubBranch || 'main',
    repositoryToken: '',
    repositoryTokenConfigured:
      value.repositoryTokenConfigured ?? value.githubTokenConfigured ?? false,
    repositoryStatus,
    pipelineEngine: value.pipelineEngine || 'JENKINS',
    ciBaseUrl: value.ciBaseUrl || value.jenkinsUrl || '',
    ciUsername: value.ciUsername || value.jenkinsUsername || '',
    ciToken: '',
    ciTokenConfigured: value.ciTokenConfigured ?? value.jenkinsTokenConfigured ?? false,
    jobName: value.jobName || value.jenkinsJobName || '',
    pollingCadence: normalizeCadence(value.pollingCadence),
    autoRebuild: value.autoRebuild ?? true,
    autoAITriage: value.autoAITriage ?? true,
    status,
  };
}

function statusPresentation(status: IntegrationStatus | undefined) {
  switch (status) {
    case 'CONNECTED':
      return { label: 'CONNECTED', color: '#34d399', background: 'rgba(16,185,129,.13)' };
    case 'SYNCING':
      return { label: 'SYNCING / POLLING', color: '#fbbf24', background: 'rgba(245,158,11,.13)' };
    case 'ERROR':
      return { label: 'CONNECTION ERROR', color: '#f87171', background: 'rgba(239,68,68,.13)' };
    default:
      return { label: 'DISCONNECTED', color: '#94a3b8', background: 'rgba(148,163,184,.1)' };
  }
}

export default function IntegrationSettingsModal({ user, onClose, onSaved }: Props) {
  const [config, setConfig] = useState<PlatformIntegrationConfig>(() => emptyConfig(user));
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [notice, setNotice] = useState<Notice | null>(null);
  const pipelineEngine = config.pipelineEngine || 'JENKINS';
  const isJenkins = pipelineEngine === 'JENKINS';
  const isGitHubActions = pipelineEngine === 'GITHUB_ACTIONS';
  const isKubernetesJob = pipelineEngine === 'KUBERNETES_JOB';
  const ciBaseLabel = isJenkins
    ? 'Jenkins base URL'
    : isGitHubActions ? 'GitHub API base URL' : 'Kubernetes API URL';
  const ciBasePlaceholder = isJenkins
    ? 'https://jenkins.example.com'
    : isGitHubActions ? 'https://api.github.com' : 'https://kubernetes.default.svc';
  const ciTokenLabel = isJenkins
    ? 'Jenkins API token'
    : isGitHubActions ? 'CI token (not used)' : 'Service-account bearer token (optional)';

  useEffect(() => {
    let mounted = true;
    const userId = user?.userId || 'local-sre';
    integrationApi.getConfig(userId)
      .then((data) => {
        if (mounted && data) setConfig(normalizeConfig(data, user));
      })
      .catch(() => {
        if (mounted) {
          setNotice({
            tone: 'info',
            text: 'No saved connection was found. Add repository and CI credentials to connect.',
          });
        }
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => { mounted = false; };
  }, [user]);

  const status = useMemo(() => statusPresentation(config.status), [config.status]);
  const update = <K extends keyof PlatformIntegrationConfig>(
    key: K,
    value: PlatformIntegrationConfig[K],
  ) => setConfig((current) => ({ ...current, [key]: value }));

  const changeRepositoryProvider = (
    nextProvider: NonNullable<PlatformIntegrationConfig['repositoryProvider']>,
  ) => {
    setConfig((current) => current.repositoryProvider === nextProvider
      ? current
      : {
          ...current,
          repositoryProvider: nextProvider,
          repositoryToken: '',
          repositoryTokenConfigured: false,
        });
  };

  const changePipelineEngine = (nextEngine: NonNullable<PlatformIntegrationConfig['pipelineEngine']>) => {
    const defaultBaseURL = nextEngine === 'GITHUB_ACTIONS'
      ? 'https://api.github.com'
      : nextEngine === 'KUBERNETES_JOB' ? 'https://kubernetes.default.svc' : '';
    setConfig((current) => ({
      ...current,
      pipelineEngine: nextEngine,
      ciBaseUrl: current.pipelineEngine === nextEngine ? current.ciBaseUrl : defaultBaseURL,
      ciUsername: nextEngine === 'JENKINS' ? current.ciUsername : '',
      ciToken: '',
      ciTokenConfigured: current.pipelineEngine === nextEngine
        ? current.ciTokenConfigured
        : false,
    }));
  };

  const connectPayload = (): IntegrationConnectPayload => ({
    userId: user?.userId || config.userId || 'local-sre',
    username: user?.username || config.username,
    repositoryProvider: config.repositoryProvider || 'GITHUB',
    repositoryUrl: config.repositoryUrl?.trim() || '',
    targetBranch: config.targetBranch?.trim() || 'main',
    repositoryToken: config.repositoryToken?.trim() || undefined,
    pipelineEngine,
    ciBaseUrl: config.ciBaseUrl?.trim() || undefined,
    ciUsername: isJenkins ? config.ciUsername?.trim() || undefined : undefined,
    ciToken: isGitHubActions ? undefined : config.ciToken?.trim() || undefined,
    jobName: config.jobName?.trim() || '',
    pollingCadence: config.pollingCadence || '15_MINUTES',
    autoRebuild: config.autoRebuild ?? true,
    autoAITriage: config.autoAITriage ?? true,
  });

  const handleConnect = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setNotice(null);
    try {
      const saved = await integrationApi.connect(connectPayload());
      const normalized = normalizeConfig(saved, user);
      setConfig(normalized);
      onSaved(normalized);
      setNotice({
        tone: normalized.status === 'CONNECTED' ? 'success' : 'info',
        text: saved.message || 'Credentials validated and autonomous polling configured.',
      });
    } catch (error) {
      setNotice({ tone: 'error', text: (error as Error).message });
    } finally {
      setSaving(false);
    }
  };

  const handleSyncNow = async () => {
    setSyncing(true);
    setNotice(null);
    setConfig((current) => ({ ...current, status: 'SYNCING' }));
    try {
      const result = await integrationApi.syncPlatforms(user?.userId || config.userId || 'local-sre');
      const nextStatus: IntegrationStatus = result.success ? 'CONNECTED' : 'ERROR';
      const next = { ...config, status: nextStatus, lastSyncTime: new Date().toISOString() };
      setConfig(next);
      onSaved(next);
      setNotice({
        tone: result.success ? 'success' : 'error',
        text: result.message || (result.success ? 'Polling cycle completed.' : 'Polling cycle failed.'),
      });
    } catch (error) {
      setConfig((current) => ({ ...current, status: 'ERROR' }));
      setNotice({ tone: 'error', text: (error as Error).message });
    } finally {
      setSyncing(false);
    }
  };

  return (
    <div
      className="modal-backdrop"
      style={{ zIndex: 60, padding: 20 }}
      onClick={onClose}
      role="presentation"
    >
      <div
        className="modal-panel"
        style={{ maxWidth: 760, padding: 0, overflow: 'hidden' }}
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-labelledby="integration-title"
      >
        <div style={{
          padding: '22px 24px',
          borderBottom: '1px solid var(--color-border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          background: 'linear-gradient(120deg, rgba(59,130,246,.1), rgba(99,102,241,.04))',
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <div style={{
              padding: 9,
              borderRadius: 10,
              background: 'rgba(59,130,246,.14)',
              color: '#60a5fa',
              border: '1px solid rgba(59,130,246,.28)',
            }}>
              <Settings size={20} />
            </div>
            <div>
              <h2 id="integration-title" style={{ fontSize: 17, margin: 0 }}>
                Repository & CI connections
              </h2>
              <p style={{ color: 'var(--color-text-muted)', fontSize: 12, marginTop: 2 }}>
                Validate read access, then schedule build and failure checks.
              </p>
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
            <span style={{
              color: status.color,
              background: status.background,
              border: '1px solid ' + status.color + '44',
              borderRadius: 999,
              fontSize: 10,
              fontWeight: 750,
              letterSpacing: '.05em',
              padding: '4px 9px',
            }}>
              {status.label}
            </span>
            <button onClick={onClose} className="btn btn-ghost" style={{ padding: 6 }} aria-label="Close">
              <X size={18} />
            </button>
          </div>
        </div>

        <div style={{ maxHeight: 'calc(90vh - 82px)', overflowY: 'auto', padding: 24 }}>
          {notice && (
            <div style={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 8,
              padding: '10px 12px',
              marginBottom: 16,
              borderRadius: 8,
              fontSize: 12,
              color: notice.tone === 'error' ? '#fca5a5' : notice.tone === 'success' ? '#6ee7b7' : '#93c5fd',
              background: notice.tone === 'error'
                ? 'rgba(239,68,68,.1)'
                : notice.tone === 'success'
                  ? 'rgba(16,185,129,.1)'
                  : 'rgba(59,130,246,.1)',
              border: '1px solid ' + (notice.tone === 'error' ? 'rgba(239,68,68,.25)' : notice.tone === 'success' ? 'rgba(16,185,129,.25)' : 'rgba(59,130,246,.25)'),
            }}>
              {notice.tone === 'error' ? <AlertTriangle size={15} /> : <CheckCircle2 size={15} />}
              <span>{notice.text}</span>
            </div>
          )}

          <form onSubmit={handleConnect} style={{ display: 'grid', gap: 16 }}>
            <section style={{ padding: 16, borderRadius: 12, background: '#090d16', border: '1px solid var(--color-border)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                <GitBranch size={16} color="#60a5fa" />
                <strong style={{ fontSize: 14 }}>Source repository</strong>
                <span style={{ marginLeft: 'auto', fontSize: 10, color: 'var(--color-text-muted)' }}>
                  contents:read · pull_requests:read{isGitHubActions ? ' · actions:write' : ''}
                </span>
              </div>
              <div className="integration-grid integration-grid-three">
                <label>
                  <span style={labelStyle}>Provider</span>
                  <select
                    style={inputStyle}
                    value={config.repositoryProvider}
                    onChange={(event) => changeRepositoryProvider(event.target.value as NonNullable<PlatformIntegrationConfig['repositoryProvider']>)}
                  >
                    <option value="GITHUB">GitHub</option>
                    <option value="GITLAB">GitLab</option>
                    <option value="BITBUCKET">Bitbucket</option>
                  </select>
                </label>
                <label style={{ gridColumn: 'span 2' }}>
                  <span style={labelStyle}>Repository URL</span>
                  <input
                    style={inputStyle}
                    type="url"
                    required
                    autoComplete="url"
                    value={config.repositoryUrl || ''}
                    onChange={(event) => update('repositoryUrl', event.target.value)}
                    placeholder="https://github.com/owner/repository"
                  />
                </label>
                <label>
                  <span style={labelStyle}>Target branch</span>
                  <input
                    style={inputStyle}
                    required
                    value={config.targetBranch || ''}
                    onChange={(event) => update('targetBranch', event.target.value)}
                    placeholder="main"
                  />
                </label>
                <label style={{ gridColumn: 'span 2' }}>
                  <span style={labelStyle}>Repository access token</span>
                  <div style={{ position: 'relative' }}>
                    <Key size={14} style={{ position: 'absolute', left: 10, top: 11, color: '#64748b' }} />
                    <input
                      style={{ ...inputStyle, paddingLeft: 32 }}
                      type="password"
                      autoComplete="new-password"
                      required={!config.repositoryTokenConfigured}
                      value={config.repositoryToken || ''}
                      onChange={(event) => update('repositoryToken', event.target.value)}
                      placeholder={config.repositoryTokenConfigured ? 'Configured — leave blank to keep it' : 'Required repository token'}
                    />
                  </div>
                  {isGitHubActions && (
                    <span style={{ display: 'block', marginTop: 5, color: 'var(--color-text-muted)', fontSize: 10 }}>
                      This token also triggers the selected workflow, so grant Actions write access.
                    </span>
                  )}
                </label>
              </div>
            </section>

            <section style={{ padding: 16, borderRadius: 12, background: '#090d16', border: '1px solid var(--color-border)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                <Server size={16} color="#a78bfa" />
                <strong style={{ fontSize: 14 }}>CI/CD engine</strong>
                <span style={{ marginLeft: 'auto', fontSize: 10, color: 'var(--color-text-muted)' }}>
                  {isJenkins
                    ? 'Job/Build · Job/Read · Job/Workspace'
                    : isGitHubActions ? 'Uses repository token' : 'jobs:get · jobs:create · jobs:watch'}
                </span>
              </div>
              <div className="integration-grid integration-grid-two">
                <label>
                  <span style={labelStyle}>Pipeline engine</span>
                  <select
                    style={inputStyle}
                    value={pipelineEngine}
                    onChange={(event) => changePipelineEngine(event.target.value as NonNullable<PlatformIntegrationConfig['pipelineEngine']>)}
                  >
                    <option value="JENKINS">Jenkins</option>
                    <option value="GITHUB_ACTIONS">GitHub Actions</option>
                    <option value="KUBERNETES_JOB">Kubernetes Job</option>
                  </select>
                </label>
                <label>
                  <span style={labelStyle}>
                    {isJenkins ? 'Jenkins job name' : isGitHubActions ? 'Workflow file or ID' : 'Kubernetes Job template'}
                  </span>
                  <input
                    style={inputStyle}
                    required
                    value={config.jobName || ''}
                    onChange={(event) => update('jobName', event.target.value)}
                    placeholder={isGitHubActions ? 'ci.yml' : 'sre-copilot-pipeline'}
                  />
                </label>
                <label style={{ gridColumn: '1 / -1' }}>
                  <span style={labelStyle}>{ciBaseLabel}</span>
                  <input
                    style={inputStyle}
                    type="url"
                    required
                    autoComplete="url"
                    value={config.ciBaseUrl || ''}
                    onChange={(event) => update('ciBaseUrl', event.target.value)}
                    placeholder={ciBasePlaceholder}
                  />
                </label>
                <label>
                  <span style={labelStyle}>Jenkins API user</span>
                  <input
                    style={inputStyle}
                    autoComplete="username"
                    disabled={!isJenkins}
                    value={isJenkins ? config.ciUsername || '' : ''}
                    onChange={(event) => update('ciUsername', event.target.value)}
                    placeholder={isJenkins ? 'service-account' : 'Not used for this engine'}
                  />
                </label>
                <label>
                  <span style={labelStyle}>{ciTokenLabel}</span>
                  <input
                    style={inputStyle}
                    type="password"
                    autoComplete="new-password"
                    required={isJenkins && !config.ciTokenConfigured}
                    disabled={isGitHubActions}
                    aria-describedby={isGitHubActions || isKubernetesJob ? 'ci-token-help' : undefined}
                    value={config.ciToken || ''}
                    onChange={(event) => update('ciToken', event.target.value)}
                    placeholder={isGitHubActions
                      ? 'Uses the repository token above'
                      : config.ciTokenConfigured
                        ? 'Configured — leave blank to keep it'
                        : isJenkins ? 'Required Jenkins API token' : 'Optional for external clusters'}
                  />
                  {(isGitHubActions || isKubernetesJob) && (
                    <span id="ci-token-help" style={{ display: 'block', marginTop: 5, color: 'var(--color-text-muted)', fontSize: 10 }}>
                      {isGitHubActions
                        ? 'GitHub Actions authenticates with the repository token above.'
                        : 'In-cluster access uses the mounted service-account token; provide a bearer token only for an external cluster.'}
                    </span>
                  )}
                </label>
              </div>
            </section>

            <section style={{ padding: 16, borderRadius: 12, background: '#090d16', border: '1px solid var(--color-border)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 14 }}>
                <Clock3 size={16} color="#fbbf24" />
                <strong style={{ fontSize: 14 }}>Autonomous polling</strong>
              </div>
              <div className="integration-grid integration-grid-two" style={{ alignItems: 'end' }}>
                <label>
                  <span style={labelStyle}>Polling cadence</span>
                  <select
                    style={inputStyle}
                    value={config.pollingCadence}
                    onChange={(event) => update('pollingCadence', event.target.value as PlatformIntegrationConfig['pollingCadence'])}
                  >
                    <option value="5_MINUTES">Every 5 minutes</option>
                    <option value="15_MINUTES">Every 15 minutes</option>
                    <option value="1_HOUR">Every hour</option>
                    <option value="DAILY_CRON">Daily</option>
                  </select>
                </label>
                <div style={{ display: 'grid', gap: 9, paddingBottom: 2 }}>
                  <label className="integration-toggle">
                    <input
                      type="checkbox"
                      checked={config.autoRebuild ?? true}
                      onChange={(event) => update('autoRebuild', event.target.checked)}
                    />
                    Trigger CI when a new commit is detected
                  </label>
                  <label className="integration-toggle">
                    <input
                      type="checkbox"
                      checked={config.autoAITriage ?? true}
                      onChange={(event) => update('autoAITriage', event.target.checked)}
                    />
                    Run code-aware AI triage after a failed build
                  </label>
                </div>
              </div>
            </section>

            <div style={{
              display: 'flex', gap: 9, alignItems: 'center', padding: '9px 11px',
              borderRadius: 8, color: '#94a3b8', background: 'rgba(15,23,42,.65)',
              border: '1px solid var(--color-border)', fontSize: 11,
            }}>
              <ShieldCheck size={15} color="#34d399" />
              Tokens are write-only: the API stores protected values and never returns them to the browser.
            </div>

            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, marginTop: 2 }}>
              <button
                type="button"
                className="btn btn-ghost"
                onClick={handleSyncNow}
                disabled={loading || saving || syncing || config.status === 'DISCONNECTED'}
              >
                <RefreshCw size={14} style={{ animation: syncing ? 'spin 1s linear infinite' : 'none' }} />
                {syncing ? 'Polling…' : 'Test & poll now'}
              </button>
              <button type="submit" className="btn btn-primary" disabled={loading || saving || syncing}>
                <Check size={15} />
                {saving ? 'Validating connections…' : 'Validate, save & connect'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}
