pipeline {
    agent any

    environment {
        DOCKER_REGISTRY = 'docker.io'
        IMAGE_PREFIX    = 'docker.io/ramanred/sre-copilot'
        SONAR_HOST_URL  = 'https://sonarcloud.io'
        SONAR_ORG       = 'ramanred'
        SRE_WEBHOOK_URL = 'http://localhost/api/v1/ci/webhook'
    }

    options {
        buildDiscarder(logRotator(numToKeepStr: '10'))
        timeout(time: 45, unit: 'MINUTES')
        timestamps()
        disableConcurrentBuilds()
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
                script {
                    env.IMAGE_TAG = sh(
                        script: 'git rev-parse HEAD',
                        returnStdout: true
                    ).trim()
                    def rawBranch = env.BRANCH_NAME ?: env.GIT_BRANCH ?: ''
                    if (rawBranch.startsWith('refs/heads/')) {
                        rawBranch = rawBranch.substring('refs/heads/'.length())
                    } else if (rawBranch.startsWith('origin/')) {
                        rawBranch = rawBranch.substring('origin/'.length())
                    }
                    // Preserve the complete path: feature/main must never be
                    // treated as the protected release branch main.
                    env.RELEASE_BRANCH = rawBranch
                    echo "Building ${env.RELEASE_BRANCH ?: 'unknown branch'} @ ${env.IMAGE_TAG}"
                }
            }
        }

        stage('Go Quality Gates') {
            steps {
                sh '''
                    set -eu
                    mkdir -p /var/jenkins_home/.cache/go-build /var/jenkins_home/go/pkg/mod
                    docker run --rm \
                      -u "$(id -u):$(id -g)" \
                      -e HOME=/tmp \
                      -e GOCACHE=/go-cache \
                      -e GOMODCACHE=/go-mod-cache \
                      -v "$WORKSPACE:$WORKSPACE" \
                      -v /var/jenkins_home/.cache/go-build:/go-cache \
                      -v /var/jenkins_home/go/pkg/mod:/go-mod-cache \
                      -w "$WORKSPACE" \
                      golang:1.25 \
                      sh -ec '
                        unformatted=$(gofmt -l .)
                        test -z "$unformatted" || { echo "$unformatted"; exit 1; }
                        go vet ./...
                        go test -race -covermode=atomic -coverprofile=apps/incident-service/coverage.out ./apps/incident-service/...
                        go test -race -covermode=atomic -coverprofile=apps/ai-copilot-service/coverage.out ./apps/ai-copilot-service/...
                        CGO_ENABLED=0 go build -trimpath -o /tmp/incident-service ./apps/incident-service/cmd/incident-service
                        CGO_ENABLED=0 go build -trimpath -o /tmp/ai-copilot ./apps/ai-copilot-service/cmd/ai-copilot
                      '
                '''
            }
            post {
                always {
                    archiveArtifacts artifacts: 'apps/*/coverage.out', allowEmptyArchive: true
                }
            }
        }

        stage('Frontend Quality Gates') {
            steps {
                sh '''
                    set -eu
                    docker run --rm \
                      -u "$(id -u):$(id -g)" \
                      -e HOME=/tmp \
                      -v "$WORKSPACE:$WORKSPACE" \
                      -w "$WORKSPACE/apps/frontend" \
                      node:20-alpine \
                      sh -ec 'npm ci && npm run lint && npx tsc --noEmit && npm run build'
                '''
            }
        }

        stage('SonarCloud SAST') {
            steps {
                withCredentials([usernamePassword(
                    credentialsId: 'sonarqube-token',
                    usernameVariable: 'SONAR_USER',
                    passwordVariable: 'SONAR_TOKEN_VAL'
                )]) {
                    sh '''
                        set -eu
                        for service in incident-service ai-copilot-service; do
                          docker run --rm \
                            -v "$WORKSPACE:$WORKSPACE" \
                            -w "$WORKSPACE/apps/$service" \
                            -e SONAR_TOKEN="$SONAR_TOKEN_VAL" \
                            sonarsource/sonar-scanner-cli:12.1.0.3233_8.0.1 \
                            sonar-scanner \
                              -Dsonar.organization="$SONAR_ORG" \
                              -Dsonar.host.url="$SONAR_HOST_URL"
                        done
                    '''
                }
            }
        }

        stage('Docker Build') {
            steps {
                sh '''
                    set -eu
                    docker build -t "$IMAGE_PREFIX-incident-service:$IMAGE_TAG" -f apps/incident-service/Dockerfile .
                    docker build -t "$IMAGE_PREFIX-ai-copilot:$IMAGE_TAG" -f apps/ai-copilot-service/Dockerfile .
                    docker build -t "$IMAGE_PREFIX-frontend:$IMAGE_TAG" -f apps/frontend/Dockerfile .
                '''
            }
        }

        stage('Trivy Security Gate') {
            steps {
                sh '''
                    set -eu
                    for image in \
                      "$IMAGE_PREFIX-incident-service:$IMAGE_TAG" \
                      "$IMAGE_PREFIX-ai-copilot:$IMAGE_TAG" \
                      "$IMAGE_PREFIX-frontend:$IMAGE_TAG"; do
                      docker run --rm \
                        -v /var/run/docker.sock:/var/run/docker.sock \
                        aquasec/trivy:0.73.0 image \
                        --exit-code 1 --ignore-unfixed --severity CRITICAL \
                        "$image"
                    done
                '''
            }
        }

        stage('Push Images') {
            when {
                expression {
                    return ['main', 'master'].contains(env.RELEASE_BRANCH)
                }
            }
            steps {
                withCredentials([usernamePassword(
                    credentialsId: 'dockerhub-password',
                    usernameVariable: 'DOCKER_USER',
                    passwordVariable: 'DOCKER_PASS'
                )]) {
                    sh '''
                        set -eu
                        umask 077
                        DOCKER_CONFIG_TMP=$(mktemp -d "$WORKSPACE/.docker-auth.XXXXXX")
                        cleanup_registry_auth() {
                          docker logout "$DOCKER_REGISTRY" >/dev/null 2>&1 || true
                          rm -rf "$DOCKER_CONFIG_TMP"
                        }
                        trap cleanup_registry_auth EXIT INT TERM
                        export DOCKER_CONFIG="$DOCKER_CONFIG_TMP"
                        printf '%s' "$DOCKER_PASS" | docker login "$DOCKER_REGISTRY" -u "$DOCKER_USER" --password-stdin
                        for service in incident-service ai-copilot frontend; do
                          docker push "$IMAGE_PREFIX-$service:$IMAGE_TAG"
                          docker tag "$IMAGE_PREFIX-$service:$IMAGE_TAG" "$IMAGE_PREFIX-$service:latest"
                          docker push "$IMAGE_PREFIX-$service:latest"
                        done
                    '''
                }
                script {
                    def digestFor = { image ->
                        def reference = sh(
                            script: "docker image inspect --format='{{index .RepoDigests 0}}' '${image}'",
                            returnStdout: true
                        ).trim()
                        if (!(reference ==~ /.+@sha256:[0-9a-f]{64}/)) {
                            error("Registry digest was unavailable for ${image}")
                        }
                        return reference.substring(reference.indexOf('@') + 1)
                    }
                    env.INCIDENT_IMAGE_DIGEST = digestFor("${env.IMAGE_PREFIX}-incident-service:${env.IMAGE_TAG}")
                    env.AI_IMAGE_DIGEST = digestFor("${env.IMAGE_PREFIX}-ai-copilot:${env.IMAGE_TAG}")
                    env.FRONTEND_IMAGE_DIGEST = digestFor("${env.IMAGE_PREFIX}-frontend:${env.IMAGE_TAG}")
                }
            }
        }

        stage('Deploy and Verify K3s') {
            when {
                beforeInput true
                expression {
                    return ['main', 'master'].contains(env.RELEASE_BRANCH)
                }
            }
            input {
                message 'Deploy the verified digest-pinned images to the K3s environment?'
                ok 'Deploy'
            }
            steps {
                withCredentials([
                    file(credentialsId: 'k3s-kubeconfig', variable: 'KUBECONFIG_FILE'),
                    usernamePassword(
                        credentialsId: 'sre-db-credentials',
                        usernameVariable: 'DB_DEPLOY_USER',
                        passwordVariable: 'DB_DEPLOY_PASSWORD'
                    ),
                    string(
                        credentialsId: 'sre-integration-encryption-key',
                        variable: 'INTEGRATION_ENCRYPTION_KEY'
                    ),
                    string(credentialsId: 'sre-auth-session-secret', variable: 'AUTH_SESSION_SECRET_VAL'),
                    string(credentialsId: 'sre-auth-bootstrap-password', variable: 'AUTH_BOOTSTRAP_PASSWORD_VAL'),
                    string(credentialsId: 'sre-ci-webhook-token', variable: 'CI_WEBHOOK_TOKEN_VAL'),
                    string(credentialsId: 'groq-api-key', variable: 'GROQ_API_KEY')
                ]) {
                    sh '''
                        set -eu
                        KUBECONFIG_TMP="$WORKSPACE/kubeconfig.tmp"
                        SECRET_TMP=$(mktemp -d "$WORKSPACE/.deploy-secrets.XXXXXX")
                        MANIFEST_TMP=$(mktemp -d "$WORKSPACE/.deploy-manifests.XXXXXX")
                        cleanup() {
                          rm -f "$KUBECONFIG_TMP"
                          rm -rf "$SECRET_TMP"
                          rm -rf "$MANIFEST_TMP"
                        }
                        trap cleanup EXIT INT TERM
                        install -m 0600 "$KUBECONFIG_FILE" "$KUBECONFIG_TMP"
                        umask 077
                        printf '%s' "$DB_DEPLOY_USER" > "$SECRET_TMP/DB_USERNAME"
                        printf '%s' "$DB_DEPLOY_PASSWORD" > "$SECRET_TMP/DB_PASSWORD"
                        printf '%s' "$INTEGRATION_ENCRYPTION_KEY" > "$SECRET_TMP/INTEGRATION_ENCRYPTION_KEY"
                        printf '%s' "$AUTH_SESSION_SECRET_VAL" > "$SECRET_TMP/AUTH_SESSION_SECRET"
                        printf '%s' "$AUTH_BOOTSTRAP_PASSWORD_VAL" > "$SECRET_TMP/AUTH_BOOTSTRAP_PASSWORD"
                        printf '%s' "$CI_WEBHOOK_TOKEN_VAL" > "$SECRET_TMP/CI_WEBHOOK_TOKEN"
                        cp -R "$WORKSPACE/k8s/." "$MANIFEST_TMP/"
                        cat >> "$MANIFEST_TMP/kustomization.yaml" <<EOF
images:
  - name: docker.io/ramanred/sre-copilot-incident-service
    newName: $IMAGE_PREFIX-incident-service
    digest: $INCIDENT_IMAGE_DIGEST
  - name: docker.io/ramanred/sre-copilot-ai-copilot
    newName: $IMAGE_PREFIX-ai-copilot
    digest: $AI_IMAGE_DIGEST
  - name: docker.io/ramanred/sre-copilot-frontend
    newName: $IMAGE_PREFIX-frontend
    digest: $FRONTEND_IMAGE_DIGEST
EOF

                        kctl() {
                          docker run --rm -i \
                            -u "$(id -u):$(id -g)" \
                            -e HOME=/tmp \
                            --network host \
                            -v "$WORKSPACE:$WORKSPACE" \
                            -e KUBECONFIG="$KUBECONFIG_TMP" \
                            rancher/kubectl:v1.36.2@sha256:06c7a7a9772737494ae1e0c3af90f1b5385630c147e47c1c7cea92f4bed55fbe "$@"
                        }

                        kctl apply --validate=false -f "$WORKSPACE/k8s/namespace-config.yml"

                        kctl create secret generic sre-db-secret \
                          --from-file=DB_USERNAME="$SECRET_TMP/DB_USERNAME" \
                          --from-file=DB_PASSWORD="$SECRET_TMP/DB_PASSWORD" \
                          -n sre-copilot --dry-run=client -o yaml | \
                        kctl apply --validate=false -f -

                        kctl create secret generic sre-integration-secret \
                          --from-file=INTEGRATION_ENCRYPTION_KEY="$SECRET_TMP/INTEGRATION_ENCRYPTION_KEY" \
                          --from-file=AUTH_SESSION_SECRET="$SECRET_TMP/AUTH_SESSION_SECRET" \
                          --from-file=AUTH_BOOTSTRAP_PASSWORD="$SECRET_TMP/AUTH_BOOTSTRAP_PASSWORD" \
                          --from-file=CI_WEBHOOK_TOKEN="$SECRET_TMP/CI_WEBHOOK_TOKEN" \
                          -n sre-copilot --dry-run=client -o yaml | \
                        kctl apply --validate=false -f -

                        printf '%s' "$GROQ_API_KEY" | \
                        kctl create secret generic sre-ai-secret \
                          --from-file=OPENAI_API_KEY=/dev/stdin \
                          -n sre-copilot --dry-run=client -o yaml | \
                        kctl apply --validate=false -f -

                        # Render registry content digests before applying. This
                        # avoids an intermediate or final mutable-tag rollout.
                        kctl apply --validate=false -k "$MANIFEST_TMP"

                        kctl rollout status deployment/sre-postgres -n sre-copilot --timeout=180s
                        kctl rollout status deployment/ai-copilot-service -n sre-copilot --timeout=180s
                        kctl rollout status deployment/incident-service -n sre-copilot --timeout=180s
                        kctl rollout status deployment/sre-frontend -n sre-copilot --timeout=180s

                        kctl run "sre-smoke-$BUILD_NUMBER" -n sre-copilot \
                          --image=curlimages/curl:8.12.1 --restart=Never --rm -i -- \
                          sh -ec '
                            curl -fsS http://ai-copilot-service:8082/actuator/health/readiness
                            curl -fsS http://incident-service:8081/actuator/health/readiness
                            curl -fsS http://incident-service:8081/api/v1/incidents/health
                            curl -fsS http://sre-frontend/health
                          '
                        kctl get pods -n sre-copilot
                    '''
                }
            }
        }
    }

    post {
        always {
            script {
                def branch = env.RELEASE_BRANCH ?: ''
                if (['main', 'master'].contains(branch)) {
                    def result = currentBuild.currentResult == 'SUCCESS' ? 'SUCCESS' : 'FAILURE'
                    def shortSha = env.GIT_COMMIT ? env.GIT_COMMIT.take(7) : (env.IMAGE_TAG ?: 'unknown')
                    withEnv(["PIPELINE_RESULT=${result}", "SHORT_SHA=${shortSha}"]) {
                        withCredentials([string(credentialsId: 'sre-ci-webhook-token', variable: 'CI_WEBHOOK_TOKEN_VAL')]) {
                            sh '''
                        curl --fail --silent --show-error --connect-timeout 3 --max-time 10 \
                          -X POST "$SRE_WEBHOOK_URL" \
                          -H 'Content-Type: application/json' \
                          -H "Authorization: Bearer $CI_WEBHOOK_TOKEN_VAL" \
                          --data-binary @- <<EOF || echo 'SRE telemetry webhook was not reachable; build result is unchanged.'
                        {
                          "pipelineName": "sre-copilot-ci-cd",
                          "buildNumber": ${BUILD_NUMBER},
                          "ciTool": "JENKINS",
                          "status": "${PIPELINE_RESULT}",
                          "gitCommit": "${SHORT_SHA}",
                          "gitBranch": "${RELEASE_BRANCH:-unknown}",
                          "author": "jenkins-agent",
                          "commitMessage": "CI/CD Pipeline Build #${BUILD_NUMBER}",
                          "durationSeconds": 0,
                          "testsPassed": 0,
                          "testsFailed": 0,
                          "vulnerabilitiesDetected": 0,
                          "environment": "demo",
                          "logSnippet": "Go quality, container security, and deployment checks completed with result ${PIPELINE_RESULT}"
                        }
EOF
                        '''
                        }
                    }
                } else {
                    echo "Skipping deployment telemetry for non-release branch ${branch ?: 'unknown'}."
                }
            }
        }
        success {
            echo 'All applicable quality, security, publishing, and deployment stages completed.'
        }
        failure {
            echo 'Pipeline failed; inspect the first failing quality, security, rollout, or smoke stage.'
        }
    }
}
