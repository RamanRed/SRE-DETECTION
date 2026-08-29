# SRE Copilot Platform — Complete Bug Fix & Deployment Guide

> **Scanned by Claude on 2026-08-30**  
> Every file analysed: Jenkinsfile · Terraform · Ansible · K8s manifests · Docker Compose · GitHub Actions · Prometheus · Logstash · Scripts

---

## Table of Contents

1. [Bug Summary (15 Issues Found)](#1-bug-summary)
2. [Bug 1 — Traefik Disabled but Ingress Uses Traefik (CRITICAL)](#bug-1--traefik-disabled-but-ingress-uses-traefik-critical)
3. [Bug 2 — EC2 Memory Exhaustion / OOM (CRITICAL)](#bug-2--ec2-memory-exhaustion--oom-critical)
4. [Bug 3 — Missing sre-groq-secret for Manual Deploy (CRITICAL)](#bug-3--missing-sre-groq-secret-for-manual-deploy-critical)
5. [Bug 4 — Ansible SSH Key is Relative Path (HIGH)](#bug-4--ansible-ssh-key-is-relative-path-high)
6. [Bug 5 — Postgres Pod Missing resource requests (HIGH)](#bug-5--postgres-pod-missing-resource-requests-high)
7. [Bug 6 — Frontend Docker Build Copies node_modules (HIGH)](#bug-6--frontend-docker-build-copies-node_modules-high)
8. [Bug 7 — Jenkinsfile Kubeconfig Patch Has Wrong YAML Indentation (HIGH)](#bug-7--jenkinsfile-kubeconfig-patch-has-wrong-yaml-indentation-high)
9. [Bug 8 — Incident Service Readiness Probe Uses Non-Standard Endpoint (HIGH)](#bug-8--incident-service-readiness-probe-uses-non-standard-endpoint-high)
10. [Bug 9 — Jenkinsfile --volumes-from jenkins Assumption (MEDIUM)](#bug-9--jenkinsfile---volumes-from-jenkins-assumption-medium)
11. [Bug 10 — bootstrap_node.sh vs Ansible Traefik Inconsistency (MEDIUM)](#bug-10--bootstrap_nodesh-vs-ansible-traefik-inconsistency-medium)
12. [Bug 11 — Prometheus Kubernetes SD Needs RBAC (MEDIUM)](#bug-11--prometheus-kubernetes-sd-needs-rbac-medium)
13. [Bug 12 — Hard-coded EC2 IP in Jenkinsfile (MEDIUM)](#bug-12--hard-coded-ec2-ip-in-jenkinsfile-medium)
14. [Bug 13 — Ollama URL in ai-copilot.yml When Ollama Not Deployed (LOW)](#bug-13--ollama-url-in-ai-copilot-yml-when-ollama-not-deployed-low)
15. [Bug 14 — Dynamic RDS Engine Version Can Break Terraform (LOW)](#bug-14--dynamic-rds-engine-version-can-break-terraform-low)
16. [Bug 15 — k3s-kubeconfig.yaml Committed to Git (LOW / SECURITY)](#bug-15--k3s-kubeconfigyaml-committed-to-git-low--security)
17. [Full Deployment Runbook (Step-by-Step After Fixes)](#full-deployment-runbook)
18. [Jenkins Credentials Reference](#jenkins-credentials-reference)
19. [Quick Local Run (Docker Compose)](#quick-local-run-docker-compose)

---

## 1. Bug Summary

| # | Severity | File | Issue |
|---|----------|------|-------|
| 1 | 🔴 CRITICAL | `ansible/playbook.yml` + `k8s/frontend.yml` | Traefik disabled in K3s but Ingress still uses `traefik` class — no external HTTP access |
| 2 | 🔴 CRITICAL | `terraform/variables.tf` + all `k8s/*.yml` | EC2 runs out of RAM — K3s + 3 Spring Boot JVMs + Postgres exceeds t3.small/micro capacity |
| 3 | 🔴 CRITICAL | `k8s/ai-copilot.yml` | `sre-groq-secret` only created by Jenkins; manual `kubectl apply` crashes ai-copilot pod |
| 4 | 🟠 HIGH | `ansible/inventory.ini` | SSH key path is relative — Ansible cannot connect |
| 5 | 🟠 HIGH | `k8s/postgres.yml` | Postgres pod has no `resources.requests` — scheduler cannot properly place the pod |
| 6 | 🟠 HIGH | `apps/frontend/Dockerfile` | No `.dockerignore` — copies entire `node_modules` dir into build context (500MB+) |
| 7 | 🟠 HIGH | `Jenkinsfile` | `sed` patch for kubeconfig inserts `insecure-skip-tls-verify` at wrong YAML level |
| 8 | 🟠 HIGH | `k8s/incident-service.yml` | Readiness probe hits `/api/v1/incidents/health` — must match actual Spring Boot endpoint |
| 9 | 🟡 MEDIUM | `Jenkinsfile` | `--volumes-from jenkins` requires Jenkins to run as a Docker container named exactly `jenkins` |
| 10 | 🟡 MEDIUM | `scripts/bootstrap_node.sh` | Traefik enabled here but disabled in `ansible/playbook.yml` — inconsistent |
| 11 | 🟡 MEDIUM | `observability/prometheus/prometheus.yml` | `kubernetes_sd_configs` needs K8s RBAC (ServiceAccount + ClusterRole) — not provided |
| 12 | 🟡 MEDIUM | `Jenkinsfile` | EC2 IP hard-coded in success message (`16.16.175.206`) |
| 13 | 🟢 LOW | `k8s/ai-copilot.yml` | Ollama URL set as env var but Ollama is not deployed in K8s |
| 14 | 🟢 LOW | `terraform/main.tf` | Dynamic `aws_rds_engine_version` data source picks "latest" — can cause plan drift |
| 15 | 🟢 LOW | `k3s-kubeconfig.yaml` | Kubeconfig with client certificate committed to Git repository root |

---

## Bug 1 — Traefik Disabled but Ingress Uses Traefik (CRITICAL)

**Root cause of: "frontend URL not opening in browser"**

### What is wrong

`ansible/playbook.yml` installs K3s with `--disable traefik`:
```yaml
INSTALL_K3S_EXEC="server --disable traefik --write-kubeconfig-mode=644"
```

But `k8s/frontend.yml` has the Ingress use traefik:
```yaml
annotations:
  kubernetes.io/ingress.class: "traefik"
```

Since Traefik is disabled, there is no Ingress controller running. The Ingress object is created but nothing processes it — port 80 on the EC2 shows nothing.

### Fix — Option A: Re-enable Traefik (Recommended)

Remove `--disable traefik` from the Ansible playbook.

**File: `ansible/playbook.yml`**

Change:
```yaml
- name: Install K3s single-node cluster
  shell: >
    curl -sfL https://get.k3s.io |
    INSTALL_K3S_VERSION={{ k3s_version }}
    INSTALL_K3S_EXEC="server --disable traefik --write-kubeconfig-mode=644"
    sh -
```

To:
```yaml
- name: Install K3s single-node cluster
  shell: >
    curl -sfL https://get.k3s.io |
    INSTALL_K3S_VERSION={{ k3s_version }}
    INSTALL_K3S_EXEC="server --write-kubeconfig-mode=644"
    sh -
```

If K3s is already installed with Traefik disabled, run this on the EC2:
```bash
sudo systemctl stop k3s
sudo /usr/local/bin/k3s-uninstall.sh
curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=v1.30.2+k3s1 \
  INSTALL_K3S_EXEC="server --write-kubeconfig-mode=644" sh -
```

### Fix — Option B: Use NodePort (No Ingress Controller Needed)

If you prefer not to use Traefik, change the frontend Service to NodePort in `k8s/frontend.yml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: sre-frontend
  namespace: sre-copilot
spec:
  selector:
    app: sre-frontend
  ports:
    - name: http
      port: 80
      targetPort: 80
      nodePort: 30080      # Access via http://EC2_IP:30080
  type: NodePort
```

Also add port `30080` to the EC2 security group in `terraform/main.tf`:
```hcl
ingress {
  description = "Frontend NodePort"
  from_port   = 30080
  to_port     = 30080
  protocol    = "tcp"
  cidr_blocks = ["0.0.0.0/0"]
}
```

---

## Bug 2 — EC2 Memory Exhaustion / OOM (CRITICAL)

**Root cause of: "K3s keeps crashing", "pods stuck in Pending/OOMKilled", "25MB RAM free"**

### Memory Math (Why it fails)

| Component | RAM Used |
|-----------|----------|
| Ubuntu OS | ~200 MB |
| K3s control plane | ~450 MB |
| postgres pod (limit) | 256 MB |
| incident-service pod (limit) | 320 MB |
| ai-copilot-service pod (limit) | 350 MB |
| frontend pod (limit) | 128 MB |
| **Total** | **~1.7 GB** |

A `t3.small` has 2 GB RAM. Even with a 2 GB swapfile, JVM GC pauses on swap cause extreme latency. On `t3.micro` (1 GB), the system OOM-kills pods instantly.

### Fix 1 — Verify EC2 is actually t3.small (check tfvars)

`terraform/variables.tf` defaults to `t3.small`. Make sure `terraform.tfvars` has NOT overridden it to `t3.micro`:

```hcl
# terraform/terraform.tfvars  — add this line explicitly
ec2_instance_type = "t3.small"
```

Verify on running EC2:
```bash
curl http://169.254.169.254/latest/meta-data/instance-type
# Must return: t3.small
```

### Fix 2 — Tighten JVM heap limits in K8s manifests

**File: `k8s/incident-service.yml`** — reduce JVM heap:
```yaml
- name: JAVA_TOOL_OPTIONS
  value: "-XX:TieredStopAtLevel=1 -Xmx160m -Xms48m -XX:+UseSerialGC"
```

**File: `k8s/ai-copilot.yml`** — reduce JVM heap:
```yaml
- name: JAVA_TOOL_OPTIONS
  value: "-XX:TieredStopAtLevel=1 -Xmx170m -Xms48m -XX:+UseSerialGC"
```

`-XX:+UseSerialGC` uses far less overhead memory than G1GC on small heaps.

### Fix 3 — Verify swap is active on EC2

```bash
ssh -i sre-copilot-key.pem ubuntu@16.16.175.206
free -h
# Should show 2GB swap
```

If swap is missing:
```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### Fix 4 — Enable memory swap accounting (stops K3s OOM killer from being aggressive)

```bash
# Add to /etc/default/grub
GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1"
sudo update-grub
sudo reboot
```

---

## Bug 3 — Missing sre-groq-secret for Manual Deploy (CRITICAL)

**Root cause of: "ai-copilot pod stuck in CreateContainerConfigError"**

### What is wrong

`k8s/ai-copilot.yml` references this secret:
```yaml
- name: SPRING_AI_OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: sre-groq-secret
      key: GROQ_API_KEY
```

This secret is ONLY created by the Jenkins pipeline. If you run `kubectl apply -f k8s/` directly (as done during debugging), the secret doesn't exist and the pod cannot start.

### Fix — Create the Secret Before Applying Manifests

```bash
# Replace YOUR_GROQ_KEY with your actual key from https://console.groq.com/keys
kubectl create secret generic sre-groq-secret \
  --from-literal=GROQ_API_KEY=YOUR_GROQ_KEY \
  -n sre-copilot \
  --dry-run=client -o yaml | kubectl apply -f -
```

**OR** add a placeholder to `k8s/namespace-config.yml` so the cluster always has the secret:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: sre-groq-secret
  namespace: sre-copilot
type: Opaque
stringData:
  GROQ_API_KEY: "REPLACE_WITH_REAL_KEY"   # <-- replace before applying
```

**Important:** Get your free Groq API key at https://console.groq.com/keys. The model `llama-3.3-70b-versatile` is free on the Groq free tier.

---

## Bug 4 — Ansible SSH Key is Relative Path (HIGH)

### What is wrong

**File: `ansible/inventory.ini`**
```ini
ansible_ssh_private_key_file="sre-copilot-key.pem"
```

This is a relative path. Ansible resolves it relative to the working directory where you run the command. If you're not in the project root, the key won't be found.

### Fix — Use Absolute Path

```ini
[k3s_nodes]
sre-node ansible_host=16.16.175.206 ansible_user=ubuntu \
  ansible_ssh_private_key_file="/absolute/path/to/sre-copilot-key.pem"
```

On Windows with WSL, the path looks like:
```ini
ansible_ssh_private_key_file="/mnt/c/Users/raman/Desktop/trainer module/SRE PROJECT/sre-copilot-key.pem"
```

Also ensure key permissions are correct:
```bash
chmod 400 "sre-copilot-key.pem"
```

---

## Bug 5 — Postgres Pod Missing resource requests (HIGH)

### What is wrong

**File: `k8s/postgres.yml`** — only `limits` are set, `requests` are missing:
```yaml
resources:
  limits:
    memory: "256Mi"
    cpu: "250m"
  # NO requests! K3s scheduler cannot properly place this pod
```

Without `requests`, K8s cannot bin-pack pods correctly and may schedule postgres onto a node that then has no room left.

### Fix

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "250m"
```

---

## Bug 6 — Frontend Docker Build Copies node_modules (HIGH)

### What is wrong

**File: `apps/frontend/Dockerfile`**
```dockerfile
COPY apps/frontend/ ./
```

The project has a `node_modules` directory present (visible in the file listing). Without a `.dockerignore`, this entire folder (often 300–500 MB) gets sent as Docker build context and copied into the builder stage, causing:
- Extremely slow builds
- Potential platform-incompatible native binaries
- Build failures (packages compiled for Windows won't work in Linux containers)

### Fix — Create `.dockerignore` in Project Root

Create the file `C:\Users\raman\Desktop\trainer module\SRE PROJECT\.dockerignore`:

```
# Node
apps/frontend/node_modules
apps/frontend/dist
apps/frontend/.vite

# Java
apps/*/target
apps/*/.mvn

# Git
.git
.github

# Misc
**/*.log
**/.DS_Store
*.pem
k3s-kubeconfig.yaml
terraform/.terraform
terraform/terraform.tfstate*
```

---

## Bug 7 — Jenkinsfile Kubeconfig Patch Has Wrong YAML Indentation (HIGH)

### What is wrong

**File: `Jenkinsfile`**, Stage "Deploy to K3s":
```bash
sed -i '/certificate-authority-data:/d' ${WORKSPACE}/kubeconfig.tmp
sed -i '/server:/a\\    insecure-skip-tls-verify: true' ${WORKSPACE}/kubeconfig.tmp
```

The `sed` appends `insecure-skip-tls-verify: true` as a sibling of `server:`, but it needs to be under the `cluster:` key. The resulting YAML is malformed and kubectl rejects it.

### Fix — Use Python to Patch the Kubeconfig Correctly

Replace the two `sed` lines with:
```bash
python3 - <<'PYEOF'
import yaml, os
path = os.environ['WORKSPACE'] + '/kubeconfig.tmp'
with open(path) as f:
    cfg = yaml.safe_load(f)
for cluster in cfg.get('clusters', []):
    c = cluster.get('cluster', {})
    c.pop('certificate-authority-data', None)
    c['insecure-skip-tls-verify'] = True
with open(path, 'w') as f:
    yaml.dump(cfg, f, default_flow_style=False)
print("kubeconfig patched successfully")
PYEOF
```

This correctly removes the CA data and sets `insecure-skip-tls-verify` at the right level inside each cluster entry.

---

## Bug 8 — Incident Service Readiness Probe Uses Non-Standard Endpoint (HIGH)

### What is wrong

**File: `k8s/incident-service.yml`**
```yaml
readinessProbe:
  httpGet:
    path: /api/v1/incidents/health    # ← custom endpoint, may not exist in app
    port: 8081
```

Spring Boot Actuator provides `/actuator/health/readiness` out of the box. The custom `/api/v1/incidents/health` endpoint must be explicitly implemented in the application. If it isn't, the pod never becomes Ready and the deployment hangs.

### Fix

Change to the standard Spring Boot Actuator endpoint:
```yaml
readinessProbe:
  httpGet:
    path: /actuator/health/readiness
    port: 8081
  initialDelaySeconds: 40
  periodSeconds: 10
  failureThreshold: 5
livenessProbe:
  httpGet:
    path: /actuator/health/liveness
    port: 8081
  initialDelaySeconds: 60
  periodSeconds: 20
  failureThreshold: 3
```

Also ensure `application.yml` for the Spring Boot app has:
```yaml
management:
  endpoint:
    health:
      probes:
        enabled: true
  health:
    livenessState:
      enabled: true
    readinessState:
      enabled: true
```

---

## Bug 9 — Jenkinsfile `--volumes-from jenkins` Assumption (MEDIUM)

### What is wrong

**File: `Jenkinsfile`**, every `docker run` step:
```bash
docker run --rm \
  --volumes-from jenkins \
  ...
```

This requires Jenkins to be running as a Docker container with the exact name `jenkins`. If Jenkins was started with `--name jenkins-server` or any other name, all Docker steps fail with:

```
Error response from daemon: No such container: jenkins
```

### Fix — Replace with Explicit Workspace Volume Mount

Change every `--volumes-from jenkins` in the Jenkinsfile to:
```bash
docker run --rm \
  -v ${WORKSPACE}:${WORKSPACE} \
  -v /var/jenkins_home/.m2:/var/jenkins_home/.m2 \
  ...
```

This mounts the actual workspace directory regardless of how Jenkins was started.

---

## Bug 10 — bootstrap_node.sh vs Ansible Traefik Inconsistency (MEDIUM)

### What is wrong

`scripts/bootstrap_node.sh` installs K3s **with** Traefik:
```bash
INSTALL_K3S_EXEC="server --write-kubeconfig-mode=644"
```

`ansible/playbook.yml` installs K3s **without** Traefik:
```bash
INSTALL_K3S_EXEC="server --disable traefik --write-kubeconfig-mode=644"
```

Using these interchangeably produces different cluster states.

### Fix — Align Both Files

Since Bug 1 fix re-enables Traefik, update `ansible/playbook.yml` to match `bootstrap_node.sh` (remove `--disable traefik`). After applying the Bug 1 fix, both files will be consistent.

---

## Bug 11 — Prometheus Kubernetes SD Needs RBAC (MEDIUM)

### What is wrong

**File: `observability/prometheus/prometheus.yml`**
```yaml
- job_name: 'kubernetes-pods'
  kubernetes_sd_configs:
    - role: pod
```

Kubernetes Service Discovery requires a ServiceAccount with permission to list/watch pods. No RBAC manifests exist in the `k8s/` directory. Prometheus pods will log:

```
level=error msg="Failed to list *v1.Pod" err="pods is forbidden: User \"system:serviceaccount:default:default\" cannot list resource \"pods\""
```

### Fix — Add RBAC Manifest

Create `k8s/prometheus-rbac.yml`:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
  namespace: sre-copilot
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
  - apiGroups: [""]
    resources: [nodes, nodes/proxy, services, endpoints, pods]
    verbs: [get, list, watch]
  - apiGroups: [extensions, networking.k8s.io]
    resources: [ingresses]
    verbs: [get, list, watch]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
  - kind: ServiceAccount
    name: prometheus
    namespace: sre-copilot
```

Apply with:
```bash
kubectl apply -f k8s/prometheus-rbac.yml
```

---

## Bug 12 — Hard-coded EC2 IP in Jenkinsfile (MEDIUM)

### What is wrong

**File: `Jenkinsfile`**, post block:
```groovy
echo "✅ Pipeline completed successfully! Live on AWS: http://16.16.175.206"
```

If the EC2 instance is replaced (Terraform reprovisioned, EIP changes, etc.), this message is wrong and misleading.

### Fix — Fetch IP Dynamically from Kubeconfig

```groovy
post {
    success {
        script {
            def serverUrl = sh(
                script: "grep 'server:' ${WORKSPACE}/kubeconfig.tmp 2>/dev/null | awk '{print \$2}' | sed 's|https://||' | cut -d: -f1 || echo '16.16.175.206'",
                returnStdout: true
            ).trim()
            echo "✅ Pipeline completed! Live on AWS: http://${serverUrl}"
        }
    }
    failure {
        echo "❌ Pipeline failed — check stage logs above"
    }
}
```

---

## Bug 13 — Ollama URL in ai-copilot.yml When Ollama Not Deployed (LOW)

### What is wrong

**File: `k8s/ai-copilot.yml`**
```yaml
- name: SPRING_AI_OLLAMA_BASE_URL
  value: "http://localhost:11434"
```

No Ollama service is deployed in the K8s manifests. The app is correctly configured to use Groq via `SPRING_AI_OPENAI_BASE_URL: https://api.groq.com/openai`. The Ollama env var is harmless but misleading — it implies Ollama is available when it isn't.

### Fix — Remove the stale env var

Delete these lines from `k8s/ai-copilot.yml`:
```yaml
# DELETE these lines:
- name: SPRING_AI_OLLAMA_BASE_URL
  value: "http://localhost:11434"
```

---

## Bug 14 — Dynamic RDS Engine Version Can Break Terraform (LOW)

### What is wrong

**File: `terraform/main.tf`**
```hcl
data "aws_rds_engine_version" "postgres" {
  engine = "postgres"
}

resource "aws_db_instance" "postgres" {
  engine_version = data.aws_rds_engine_version.postgres.version
```

`data.aws_rds_engine_version` with no version filter picks the **latest** available version. If AWS releases a new major version (e.g. PostgreSQL 17), `terraform apply` may attempt to upgrade from 16 to 17, causing downtime or apply failure.

### Fix — Pin the Version Explicitly

```hcl
resource "aws_db_instance" "postgres" {
  engine         = "postgres"
  engine_version = "16.3"           # pinned — update deliberately
  ...
}
```

Remove the `data "aws_rds_engine_version"` block entirely.

---

## Bug 15 — k3s-kubeconfig.yaml Committed to Git (LOW / SECURITY)

### What is wrong

`k3s-kubeconfig.yaml` is committed in the repository root. It contains:
- The EC2 public IP and K3s API server URL
- A base64-encoded **client private key** (`client-key-data`)
- Client certificate data

Anyone with read access to the repo can authenticate directly to your K3s cluster.

### Fix

**Step 1 — Remove from repo:**
```bash
git rm k3s-kubeconfig.yaml
echo "k3s-kubeconfig.yaml" >> .gitignore
git add .gitignore
git commit -m "security: remove committed kubeconfig and add to gitignore"
git push
```

**Step 2 — Rotate K3s credentials** (the committed cert cannot be un-committed from history):
```bash
# On EC2 — regenerate K3s token and restart
sudo systemctl stop k3s
sudo rm -f /var/lib/rancher/k3s/server/node-token
sudo systemctl start k3s
# Copy fresh kubeconfig
sudo cat /etc/rancher/k3s/k3s.yaml
```

**Step 3 — Remove from Git history:**
```bash
git filter-branch --force --index-filter \
  'git rm --cached --ignore-unmatch k3s-kubeconfig.yaml' \
  --prune-empty --tag-name-filter cat -- --all
git push origin --force --all
```

---

## Full Deployment Runbook

Follow this order after applying all fixes above.

### Step 1 — Provision Infrastructure (Terraform)

```bash
cd "C:\Users\raman\Desktop\trainer module\SRE PROJECT\terraform"

terraform init
terraform plan -out=tfplan
terraform apply tfplan

# Note the outputs:
terraform output ec2_public_ip      # your EC2 IP
terraform output rds_endpoint       # your RDS host
terraform output ssh_command        # ready-to-use SSH command
```

### Step 2 — Bootstrap Node (Ansible)

```bash
cd "C:\Users\raman\Desktop\trainer module\SRE PROJECT"

# Update inventory.ini with correct absolute SSH key path first (Bug 4 fix)
ansible-playbook -i ansible/inventory.ini ansible/playbook.yml -v
```

Expected output ends with:
```
✓ Swap: 2GB configured
✓ Docker Engine: Installed
✓ K3s: v1.30.2+k3s1 running
✓ kubectl: Configured for ubuntu user
```

### Step 3 — Get Fresh Kubeconfig

```bash
# On EC2
ssh -i sre-copilot-key.pem ubuntu@<EC2_IP>
cat /etc/rancher/k3s/k3s.yaml
# Copy output — replace 127.0.0.1 with your EC2 public IP
```

### Step 4 — Create Groq Secret (Bug 3 fix — BEFORE applying manifests)

```bash
kubectl --kubeconfig=k3s-kubeconfig.yaml \
  create secret generic sre-groq-secret \
  --from-literal=GROQ_API_KEY=<your_groq_key> \
  -n sre-copilot \
  --dry-run=client -o yaml | kubectl apply -f -
```

### Step 5 — Apply K8s Manifests

```bash
# Apply in order
kubectl --kubeconfig=k3s-kubeconfig.yaml apply -f k8s/namespace-config.yml
kubectl --kubeconfig=k3s-kubeconfig.yaml apply -f k8s/postgres.yml
kubectl --kubeconfig=k3s-kubeconfig.yaml apply -f k8s/ai-copilot.yml
kubectl --kubeconfig=k3s-kubeconfig.yaml apply -f k8s/incident-service.yml
kubectl --kubeconfig=k3s-kubeconfig.yaml apply -f k8s/frontend.yml
```

### Step 6 — Verify Pods are Running

```bash
watch kubectl --kubeconfig=k3s-kubeconfig.yaml get pods -n sre-copilot
```

Expected output (after 2-3 minutes):
```
NAME                                 READY   STATUS    RESTARTS   AGE
sre-postgres-xxx                     1/1     Running   0          3m
ai-copilot-service-xxx               1/1     Running   0          2m
incident-service-xxx                 1/1     Running   0          2m
sre-frontend-xxx                     1/1     Running   0          1m
```

If any pod shows `OOMKilled`: see Bug 2 fix. If `CreateContainerConfigError`: see Bug 3 fix.

### Step 7 — Verify Dashboard

Open browser: `http://<EC2_IP>`

If using NodePort (Bug 1 Option B fix): `http://<EC2_IP>:30080`

### Step 8 — Run CI/CD Pipeline (Jenkins)

In Jenkins → New Item → Pipeline → paste the Jenkinsfile.

Required credentials (see below):
- `dockerhub-password`
- `sonarqube-token`
- `k3s-kubeconfig`
- `groq-api-key`

---

## Jenkins Credentials Reference

| Credential ID | Type | Value |
|---------------|------|-------|
| `dockerhub-password` | Username + Password | Docker Hub user: `ramanred`, password: your Docker Hub password |
| `sonarqube-token` | Username + Password | Username: `ramanred`, password: SonarCloud token from https://sonarcloud.io/account/security |
| `k3s-kubeconfig` | Secret file | Upload `k3s-kubeconfig.yaml` (after replacing `127.0.0.1` with public EC2 IP) |
| `groq-api-key` | Secret text | Your Groq API key from https://console.groq.com/keys |

### SonarCloud Setup Required

Go to https://sonarcloud.io → Create organisation `ramanred` → Create two projects:
- Project key: `ramanred_sre-copilot-incident-service`
- Project key: `ramanred_sre-copilot-ai-copilot`

---

## Quick Local Run (Docker Compose)

No AWS, no Kubernetes needed. Runs everything locally.

```bash
cd "C:\Users\raman\Desktop\trainer module\SRE PROJECT"

# Start all 5 services
docker compose up --build

# Access dashboard
open http://localhost
```

First run takes 3–5 minutes (Ollama pulls the AI model ~1 GB).

**Useful commands:**
```bash
# View logs for a specific service
docker compose logs -f incident-service

# Restart one service
docker compose restart ai-copilot-service

# Stop everything
docker compose down

# Stop and delete all data (fresh start)
docker compose down -v
```

---

## Troubleshooting Quick Reference

| Symptom | Cause | Fix |
|---------|-------|-----|
| `http://EC2_IP` shows nothing | Traefik disabled | Bug 1 fix |
| Pods stuck in `Pending` or `OOMKilled` | Not enough RAM | Bug 2 fix |
| `ai-copilot` pod in `CreateContainerConfigError` | Groq secret missing | Bug 3 fix |
| `ansible-playbook` fails with SSH error | Relative key path | Bug 4 fix |
| Jenkins `docker run` fails with "No such container: jenkins" | Wrong volumes-from | Bug 9 fix |
| Prometheus shows no Kubernetes metrics | Missing RBAC | Bug 11 fix |
| `kubectl` fails with TLS cert error | Kubeconfig not patched | Bug 7 fix |
| `incident-service` pod never becomes `Ready` | Wrong readiness probe path | Bug 8 fix |
| Docker build takes forever | node_modules in context | Bug 6 fix |

---

*This README was auto-generated by scanning all project files. All line references and fixes are based on the actual source code found in `C:\Users\raman\Desktop\trainer module\SRE PROJECT`.*
