# SRE Copilot frontend

React 19 + TypeScript + Vite dashboard for incidents, remediation approval,
CI/CD telemetry, DORA metrics, and platform integration settings.

## Development

Start the Go backends first, then:

```bash
npm ci
npm run dev
```

Vite listens on port 3000 and proxies `/api` to the incident service on
`http://localhost:8081`. The production Nginx image serves the SPA on port 80
and proxies the same paths to the Compose/Kubernetes DNS name
`incident-service:8081`.

## Checks

```bash
npm run lint
npx tsc --noEmit
npm run build
```

The frontend expects the compatibility contract documented in the root
`README.md`, including camelCase JSON, Spring-style pagination fields, and a
top-level `message` in error responses. That wire contract is intentionally
unchanged by the Go backend migration.

Local Compose uses explicit demo mode. The Kubernetes deployment uses a
bootstrap password, expiring signed sessions, server-side roles, and separately
authenticated CI webhooks. Integration tokens are write-only in the UI and
encrypted by the incident service. Keep the HTTP dashboard on a trusted network
and add TLS plus federated organizational identity before public multi-user
deployment.
