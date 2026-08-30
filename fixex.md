# Current platform status

The platform backends now build and run in Go 1.25 while retaining the frontend
REST contract and the protobuf gRPC contract.

- Default local mode uses deterministic heuristic AI and requires no secret.
- Cloud inference uses any OpenAI-compatible provider through `OPENAI_*`.
- Local Ollama is available through the opt-in `ollama` Compose profile.
- Kubernetes backend limits are 128 Mi with `GOMEMLIMIT=96MiB`.
- PostgreSQL uses persistent storage and health probes in Kubernetes.
- GitHub Actions and Jenkins run Go format, vet, race-test, coverage, frontend,
  and container build gates.
- Jenkins publishes full-commit tags, deploys resolved registry digests, waits
  for rollouts, and runs smoke checks.
- Image publication, deployment approval, and telemetry are restricted to
  `main`/`master`; temporary registry and cluster credentials are cleaned up.
- K3s is pinned to a supported 1.36 patch, uses Traefik consistently, and the
  2 GiB node swap is verified through cgroup v2.
- Terraform rejects global management/Ingress CIDRs. The HTTP bootstrap-auth
  Ingress must remain internal until TLS and federated identity are added.
- CI/CD and DORA endpoints remain available under `/api/v1/ci`.

See `README.md` for commands and `SRE-PROJECT-FIXES-README.md` for operational
details. Configuration files do not by themselves install the full
Prometheus/Grafana/ELK stack.
