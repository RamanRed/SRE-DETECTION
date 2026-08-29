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
        // ──────────────────────────────────────────────────────────
        stage('Maven Build & Test') {
            parallel {
                stage('incident-service') {
                    steps {
                        sh """
                            mkdir -p /var/jenkins_home/.m2/repository
                            docker run --rm \
                              --volumes-from jenkins \
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
                              --volumes-from jenkins \
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
                          --volumes-from jenkins \\
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
                          --volumes-from jenkins \\
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
        // ──────────────────────────────────────────────────────────
        stage('Deploy to K3s') {
            steps {
                withCredentials([file(credentialsId: 'k3s-kubeconfig', variable: 'KUBECONFIG_FILE')]) {
                    sh """
                        # Update image tags in manifests
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/incident-service.yml || true
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/ai-copilot.yml || true
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/frontend.yml || true

                        # Jenkins extracts secret files to /tmp which is NOT mounted in the
                        # kubectl container via --volumes-from. Copy it into the workspace
                        # (under /var/jenkins_home) so the container can reach it.
                        cp \${KUBECONFIG_FILE} ${WORKSPACE}/kubeconfig.tmp
                        chmod 600 ${WORKSPACE}/kubeconfig.tmp

                        # K3s TLS cert is issued for private IPs only (10.0.1.23, etc.), not
                        # the public EC2 IP. Patch the kubeconfig to skip TLS verification so
                        # all kubectl commands succeed without repeating the flag each time.
                        sed -i '/certificate-authority-data:/d' ${WORKSPACE}/kubeconfig.tmp
                        sed -i '/server:/a\\    insecure-skip-tls-verify: true' ${WORKSPACE}/kubeconfig.tmp

                        # Create namespace + base config first
                        docker run --rm \\
                          --volumes-from jenkins \\
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \\
                          bitnami/kubectl:latest \\
                          apply -f ${WORKSPACE}/k8s/namespace-config.yml

                        # Create GROQ API key secret from Jenkins credentials (idempotent)
                        docker run --rm \\
                          --volumes-from jenkins \\
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \\
                          bitnami/kubectl:latest \\
                          create secret generic sre-groq-secret \\
                          --from-literal=GROQ_API_KEY=${GROQ_API_KEY} \\
                          -n sre-copilot \\
                          --dry-run=client -o yaml | \\
                        docker run --rm -i \\
                          --volumes-from jenkins \\
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \\
                          bitnami/kubectl:latest \\
                          apply -f -

                        # Deploy all remaining manifests to AWS EC2 K3s cluster
                        docker run --rm \\
                          --volumes-from jenkins \\
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \\
                          bitnami/kubectl:latest \\
                          apply -f ${WORKSPACE}/k8s/

                        echo '=== Deployment to AWS K3s Cluster Successful ==='
                        docker run --rm \\
                          --volumes-from jenkins \\
                          -e KUBECONFIG=${WORKSPACE}/kubeconfig.tmp \\
                          bitnami/kubectl:latest \\
                          get pods -n sre-copilot || true

                        # Clean up temp kubeconfig from workspace
                        rm -f ${WORKSPACE}/kubeconfig.tmp
                    """
                }
            }
        }

    }

    post {
        success {
            echo "✅ Pipeline completed successfully! Live on AWS: http://16.16.175.206"
        }
        failure {
            echo "❌ Pipeline failed"
        }
    }
}
