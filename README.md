# SRE Copilot Platform

> **Autonomous Incident Triage and Remediation Platform powered by Spring AI, gRPC, and Kubernetes**

[![CI](https://github.com/your-org/sre-copilot/actions/workflows/ci.yml/badge.svg)](https://github.com/your-org/sre-copilot/actions)
[![Java](https://img.shields.io/badge/Java-17-blue?logo=openjdk)](https://adoptium.net)
[![Spring Boot](https://img.shields.io/badge/Spring%20Boot-3.3.x-brightgreen?logo=spring)](https://spring.io/projects/spring-boot)
[![React](https://img.shields.io/badge/React-18-61dafb?logo=react)](https://react.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## Architecture Overview

```
React Dashboard → Nginx Ingress → incident-service (REST :8081)
                                      ↕ gRPC (protobuf)
                                  ai-copilot-service (:9090)
                                      ↕ Spring AI
                              Ollama LLM / OpenAI API
                incident-service ↔ PostgreSQL (Flyway migrations)
```

## Tech Stack

| Layer | Technology |
|:------|:-----------|
| **Frontend** | React 18 + Vite + Tailwind CSS + TypeScript |
| **Incident Service** | Spring Boot 3.3, Java 17, Spring Data JPA, Flyway |
| **AI Copilot** | Spring Boot 3.3, Spring AI, gRPC (net.devh) |
| **LLM Backend** | Local Ollama (`qwen2.5-coder:1.5b`) or OpenAI |
| **Database** | PostgreSQL 16 (AWS RDS db.t3.micro) |
| **Container Orchestration** | K3s (Kubernetes) on AWS EC2 t3.micro |
| **IaC** | Terraform 1.5+, Ansible 2.14+ |
| **CI/CD** | GitHub Actions + Jenkins + SonarQube + Trivy |
| **Observability** | Prometheus + Grafana + ELK Stack |

## Quick Start (Local Dev)

### Prerequisites
- Java 17 (Eclipse Temurin)
- Node.js 20 LTS
- Docker Desktop + Compose v2
- Maven 3.9+

### Run with Docker Compose

```bash
# Clone the repository
git clone https://github.com/your-org/sre-copilot.git
cd sre-copilot

# Start all services (Postgres, Ollama, both backends, frontend)
docker compose up --build

# Access the dashboard
open http://localhost
```

> ⏳ First startup takes ~3-5 minutes for Ollama to pull the model.

### Run Services Individually

```bash
# Start Postgres
docker compose up postgres -d

# Start AI Copilot Service
cd apps/ai-copilot-service
mvn spring-boot:run -Dspring.ai.ollama.base-url=http://localhost:11434

# Start Incident Service
cd apps/incident-service
DB_HOST=localhost mvn spring-boot:run

# Start Frontend Dev Server
cd apps/frontend
npm run dev
```

## API Reference

| Method | Endpoint | Description |
|:-------|:---------|:------------|
| `POST` | `/api/v1/incidents` | Create new incident |
| `GET` | `/api/v1/incidents` | Paginated incident list |
| `GET` | `/api/v1/incidents/active` | Active incidents (OPEN/ANALYZING) |
| `GET` | `/api/v1/incidents/{id}` | Get single incident |
| `POST` | `/api/v1/incidents/{id}/triage` | Trigger AI root-cause analysis |
| `POST` | `/api/v1/incidents/{id}/remediate` | Generate remediation script |
| `POST` | `/api/v1/incidents/{id}/remediation/{rid}/approve` | Approve remediation |
| `GET` | `/api/v1/incidents/stats/dashboard` | Dashboard statistics |

## SRE Chaos Simulation

```bash
# Inject a DB connection pool outage and watch AI triage in action
chmod +x scripts/simulate_outage.sh
./scripts/simulate_outage.sh docker
```

## Cloud Deployment (AWS Free Tier)

```bash
# 1. Provision AWS infrastructure
cd terraform
terraform init
terraform apply -var="ec2_public_key=$(cat ~/.ssh/id_rsa.pub)" -var="db_password=YourSecretPassword"

# 2. Bootstrap EC2 node with Docker + K3s
cd ansible
ansible-playbook -i inventory.ini playbook.yml

# 3. Deploy to K3s
kubectl apply -f k8s/
```

## Syllabus Coverage

| Unit | Topics Covered |
|:-----|:--------------|
| **Unit I** | CAMS, Three Ways, Kanban, Git 3-tree, GitHub Actions, Gitleaks |
| **Unit II** | IaC (Terraform), Ansible, AWS RDS, Docker Compose, Containerization |
| **Unit III** | CI/CD, Maven, DevSecOps (SonarQube + Trivy), K8s, Jenkins |
| **Unit IV** | 3 Pillars O11y, ELK, Prometheus, Grafana, SRE, Post-Mortem |

## License

MIT — see [LICENSE](LICENSE) for details.
