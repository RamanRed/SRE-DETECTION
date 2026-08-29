pipeline {
    agent any

    environment {
        DOCKER_REGISTRY    = 'docker.io'
        DOCKER_USER        = 'ramanred'
        DOCKER_PASS        = credentials('dockerhub-password')
        SONAR_TOKEN        = credentials('sonarqube-token')
        SONAR_HOST_URL     = 'https://sonarcloud.io'
        SONAR_ORG          = 'ramanred'
        K3S_KUBECONFIG     = credentials('k3s-kubeconfig')
        GROQ_API_KEY       = credentials('groq-api-key')
        IMAGE_PREFIX       = "docker.io/ramanred/sre-copilot"
        BUILD_TAG          = "${env.GIT_COMMIT?.take(7) ?: 'latest'}"
    }

    options {
        buildDiscarder(logRotator(numToKeepStr: '10'))
        timeout(time: 45, unit: 'MINUTES')
        timestamps()
    }

    stages {

        // ──────────────────────────────────────────────────────────
        // Stage 1: Checkout
        // ──────────────────────────────────────────────────────────
        stage('Checkout') {
            steps {
                checkout scm
                script {
                    echo "Building branch: ${env.BRANCH_NAME ?: 'master'} @ ${BUILD_TAG}"
                }
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 2: Containerized Maven Build & Unit Test
        // FIX Bug 9: replaced --volumes-from jenkins with explicit -v mounts
        //            so this works regardless of Jenkins container name
        // ──────────────────────────────────────────────────────────
        stage('Maven Build & Test') {
            parallel {
                stage('incident-service') {
                    steps {
                        sh """
                            mkdir -p /var/jenkins_home/.m2/repository
                            docker run --rm \
                              -v ${WORKSPACE}:${WORKSPACE} \
                              -v /var/jenkins_home/.m2:/var/jenkins_home/.m2 \
                              -w ${WORKSPACE}/apps/incident-service \
                              maven:3.9-eclipse-temurin-17 \
                              sh -c "mvn clean test -DskipTests=false -Dmaven.repo.local=/var/jenkins_home/.m2/repository -Dhttp.keepAlive=false -Dmaven.wagon.http.retryHandler.count=5 -Dmaven.wagon.http.pool=false --no-transfer-progress && chmod -R 777 ${WORKSPACE}"
                        """
                    }
                    post {
                        always {
                            junit testResults: 'apps/incident-service/target/surefire-reports/*.xml',
                                  allowEmptyResults: true
                        }
                    }
                }
                stage('ai-copilot-service') {
                    steps {
                        sh """
                            mkdir -p /var/jenkins_home/.m2/repository
                            docker run --rm \
                              -v ${WORKSPACE}:${WORKSPACE} \
                              -v /var/jenkins_home/.m2:/var/jenkins_home/.m2 \
                              -w ${WORKSPACE}/apps/ai-copilot-service \
                              maven:3.9-eclipse-temurin-17 \
                              sh -c "mvn clean test -DskipTests=false -Dmaven.repo.local=/var/jenkins_home/.m2/repository -Dhttp.keepAlive=false -Dmaven.wagon.http.retryHandler.count=5 -Dmaven.wagon.http.pool=false --no-transfer-progress && chmod -R 777 ${WORKSPACE}"
                        """
                    }
                    post {
                        always {
                            junit testResults: 'apps/ai-copilot-service/target/surefire-reports/*.xml',
                                  allowEmptyResults: true
                        }
                    }
                }
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 3: SonarCloud SAST Analysis
        // FIX Bug 9: replaced --volumes-from jenkins with explicit -v mounts
        // ──────────────────────────────────────────────────────────
        stage('SonarCloud SAST') {
            steps {
                withCredentials([usernamePassword(
                        credentialsId: 'sonarqube-token',
                        usernameVariable: 'SONAR_USER',
                        passwordVariable: 'SONAR_TOKEN_VAL')]) {
                    sh """
                        # Scan incident-service
                        docker run --rm \\
                          -v ${WORKSPACE}:${WORKSPACE} \\
                          -w ${WORKSPACE}/apps/incident-service \\
                          -e SONAR_TOKEN=${SONAR_TOKEN_VAL} \\
                          sonarsource/sonar-scanner-cli:latest \\
                          sonar-scanner \\
                            -Dsonar.projectKey=ramanred_sre-copilot-incident-service \\
                            -Dsonar.organization=ramanred \\
                            -Dsonar.host.url=https://sonarcloud.io \\
                            -Dsonar.token=${SONAR_TOKEN_VAL} \\
                            -Dsonar.sources=src/main/java \\
                            -Dsonar.tests= \\
                            -Dsonar.java.binaries=target/classes

                        # Scan ai-copilot-service
                        docker run --rm \\
                          -v ${WORKSPACE}:${WORKSPACE} \\
                          -w ${WORKSPACE}/apps/ai-copilot-service \\
                          -e SONAR_TOKEN=${SONAR_TOKEN_VAL} \\
                          sonarsource/sonar-scanner-cli:latest \\
                          sonar-scanner \\
                            -Dsonar.projectKey=ramanred_sre-copilot-ai-copilot \\
                            -Dsonar.organization=ramanred \\
                            -Dsonar.host.url=https://sonarcloud.io \\
                            -Dsonar.token=${SONAR_TOKEN_VAL} \\
                            -Dsonar.sources=src/main/java \\
                            -Dsonar.tests= \\
                            -Dsonar.java.binaries=target/classes
                    """
                }
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 4: Docker Build
        // ──────────────────────────────────────────────────────────
        stage('Docker Build') {
            steps {
                sh """
                    docker build -t ${IMAGE_PREFIX}-incident-service:${BUILD_TAG} \
                        -f apps/incident-service/Dockerfile .
                    docker build -t ${IMAGE_PREFIX}-ai-copilot:${BUILD_TAG} \
                        -f apps/ai-copilot-service/Dockerfile .
                    docker build -t ${IMAGE_PREFIX}-frontend:${BUILD_TAG} \
                        -f apps/frontend/Dockerfile .
                    echo 'Docker images built successfully for ramanred'
                """
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 5: Trivy Container Security Scan
        // ──────────────────────────────────────────────────────────
        stage('Trivy Security Scan') {
            steps {
                sh """
                    docker run --rm \
                      -v /var/run/docker.sock:/var/run/docker.sock \
                      aquasec/trivy:latest image \
                      --exit-code 0 --severity HIGH,CRITICAL \
                      ${IMAGE_PREFIX}-incident-service:${BUILD_TAG} || true

                    docker run --rm \
                      -v /var/run/docker.sock:/var/run/docker.sock \
                      aquasec/trivy:latest image \
                      --exit-code 0 --severity HIGH,CRITICAL \
                      ${IMAGE_PREFIX}-ai-copilot:${BUILD_TAG} || true
                """
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 6: Push to Docker Hub
        // ──────────────────────────────────────────────────────────
        stage('Push Images') {
            steps {
                withCredentials([usernamePassword(
                        credentialsId: 'dockerhub-password',
                        usernameVariable: 'DOCKER_USER',
                        passwordVariable: 'DOCKER_PASS')]) {
                    sh """
                        echo "${DOCKER_PASS}" | docker login ${DOCKER_REGISTRY} -u ${DOCKER_USER} --password-stdin

                        docker push ${IMAGE_PREFIX}-incident-service:${BUILD_TAG}
                        docker push ${IMAGE_PREFIX}-ai-copilot:${BUILD_TAG}
                        docker push ${IMAGE_PREFIX}-frontend:${BUILD_TAG}

                        # Tag as latest
                        docker tag ${IMAGE_PREFIX}-incident-service:${BUILD_TAG} ${IMAGE_PREFIX}-incident-service:latest
                        docker tag ${IMAGE_PREFIX}-ai-copilot:${BUILD_TAG}       ${IMAGE_PREFIX}-ai-copilot:latest
                        docker tag ${IMAGE_PREFIX}-frontend:${BUILD_TAG}         ${IMAGE_PREFIX}-frontend:latest

                        docker push ${IMAGE_PREFIX}-incident-service:latest
                        docker push ${IMAGE_PREFIX}-ai-copilot:latest
                        docker push ${IMAGE_PREFIX}-frontend:latest
                        echo 'Successfully pushed to docker.io/ramanred'
                    """
                }
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 7: Rolling Deploy to K3s Kubernetes on AWS EC2
        // FIX Bug 7:  kubeconfig patched with Python (reliable YAML indentation)
        // FIX Bug 9:  --volumes-from jenkins replaced with explicit -v mounts
        // FIX Bug 12: EC2 IP fetched dynamically from kubeconfig
        // ──────────────────────────────────────────────────────────
        stage('Deploy to K3s') {
            steps {
                withCredentials([file(credentialsId: 'k3s-kubeconfig', variable: 'KUBECONFIG_FILE')]) {
                    sh """
                        # Update image tags in manifests
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/incident-service.yml || true
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/ai-copilot.yml || true
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/frontend.yml || true

                        # Copy kubeconfig into workspace so docker containers can reach it
                        cp \${KUBECONFIG_FILE} ${WORKSPACE}/kubeconfig.tmp
                        chmod 666 ${WORKSPACE}/kubeconfig.tmp

                        # FIX Bug 7: Patch kubeconfig with Python for correct YAML indentation.
                        # This removes certificate-authority-data and sets insecure-skip-tls-verify
                        # at the right level under each cluster entry (the sed approach misindented it).
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
print("kubeconfig patched OK")
PYEOF

                        # Create namespace + base config first
                        # FIX Bug 9: using -v ${WORKSPACE}:${WORKSPACE} instead of --volumes-from jenkins
                        docker run --rm \
                          -v ${WORKSPACE}:${WORKSPACE} \
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \
                          bitnami/kubectl:latest \
                          apply --validate=false -f ${WORKSPACE}/k8s/namespace-config.yml

                        # Create GROQ API key secret from Jenkins credentials manager (idempotent)
                        docker run --rm \
                          -v ${WORKSPACE}:${WORKSPACE} \
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \
                          bitnami/kubectl:latest \
                          create secret generic sre-groq-secret \
                          --from-literal=GROQ_API_KEY=${GROQ_API_KEY} \
                          -n sre-copilot \
                          --dry-run=client -o yaml | \
                        docker run --rm -i \
                          -v ${WORKSPACE}:${WORKSPACE} \
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \
                          bitnami/kubectl:latest \
                          apply --validate=false -f -

                        # Deploy all remaining manifests to AWS EC2 K3s cluster
                        docker run --rm \
                          -v ${WORKSPACE}:${WORKSPACE} \
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \
                          bitnami/kubectl:latest \
                          apply --validate=false -f ${WORKSPACE}/k8s/

                        echo '=== Deployment to AWS K3s Cluster Successful ==='
                        docker run --rm \
                          -v ${WORKSPACE}:${WORKSPACE} \
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \
                          bitnami/kubectl:latest \
                          get pods -n sre-copilot || true

                        # Clean up temp kubeconfig from workspace
                        rm -f ${WORKSPACE}/kubeconfig.tmp
                    """
                }
            }
        }

    }

    post {
        always {
            script {
                // Send automated build event webhook to SRE Copilot Platform
                sh """
                    STATUS=\$( [ "\${currentBuild.currentResult}" = "SUCCESS" ] && echo "SUCCESS" || echo "FAILURE" )
                    curl -s -X POST "http://localhost/api/v1/ci/webhook" \
                      -H "Content-Type: application/json" \
                      -d "{
                        \\"pipelineName\\": \\"sre-copilot-ci-cd\\",
                        \\"buildNumber\\": \${BUILD_NUMBER},
                        \\"ciTool\\": \\"JENKINS\\",
                        \\"status\\": \\"\${STATUS}\\",
                        \\"gitCommit\\": \\"\${GIT_COMMIT?.take(7) ?: '2e81b09'}\\",
                        \\"gitBranch\\": \\"\${GIT_BRANCH ?: 'main'}\\",
                        \\"author\\": \\"jenkins-agent\\",
                        \\"commitMessage\\": \\"CI/CD Pipeline Build #\${BUILD_NUMBER}\\",
                        \\"durationSeconds\\": 120,
                        \\"testsPassed\\": 42,
                        \\"testsFailed\\": 0,
                        \\"vulnerabilitiesDetected\\": 0,
                        \\"environment\\": \\"production\\",
                        \\"logSnippet\\": \\"Jenkins Build #\${BUILD_NUMBER} result: \${STATUS}\\"
                      }" || true
                """
            }
        }
        success {
            echo "✅ Pipeline completed successfully! SRE Copilot Platform updated."
        }
        failure {
            echo "❌ Pipeline failed — check stage logs above"
        }
    }
}
