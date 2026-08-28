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
                    echo "Building branch: ${env.BRANCH_NAME} @ ${BUILD_TAG}"
                }
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 2: Build & Unit Test (Maven + JaCoCo)
        // ──────────────────────────────────────────────────────────
        stage('Maven Build & Test') {
            parallel {
                stage('incident-service') {
                    steps {
                        dir('apps/incident-service') {
                            sh 'mvn clean verify -Pcoverage --no-transfer-progress'
                        }
                    }
                    post {
                        always {
                            junit testResults: 'apps/incident-service/target/surefire-reports/*.xml',
                                  allowEmptyResults: true
                            jacoco execPattern: 'apps/incident-service/target/jacoco.exec',
                                   classPattern:  'apps/incident-service/target/classes',
                                   sourcePattern: 'apps/incident-service/src/main/java'
                        }
                    }
                }
                stage('ai-copilot-service') {
                    steps {
                        dir('apps/ai-copilot-service') {
                            sh 'mvn clean verify --no-transfer-progress'
                        }
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
        // Stage 3: SonarQube SAST Analysis
        // ──────────────────────────────────────────────────────────
        stage('SonarQube SAST') {
            when { branch 'main' }
            steps {
                withSonarQubeEnv('SonarCloud') {
                    dir('apps/incident-service') {
                        sh """
                            mvn sonar:sonar \
                              -Dsonar.projectKey=ramanred_sre-copilot-incident-service \
                              -Dsonar.projectName='sre-copilot-incident-service' \
                              -Dsonar.organization=${SONAR_ORG} \
                              -Dsonar.host.url=${SONAR_HOST_URL} \
                              -Dsonar.token=${SONAR_TOKEN} \
                              --no-transfer-progress
                        """
                    }
                    dir('apps/ai-copilot-service') {
                        sh """
                            mvn sonar:sonar \
                              -Dsonar.projectKey=ramanred_sre-copilot-ai-copilot \
                              -Dsonar.projectName='sre-copilot-ai-copilot' \
                              -Dsonar.organization=${SONAR_ORG} \
                              -Dsonar.host.url=${SONAR_HOST_URL} \
                              -Dsonar.token=${SONAR_TOKEN} \
                              --no-transfer-progress
                        """
                    }
                }
                timeout(time: 5, unit: 'MINUTES') {
                    waitForQualityGate abortPipeline: true
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
                    trivy image --exit-code 0 --severity HIGH,CRITICAL \
                        --format table \
                        ${IMAGE_PREFIX}-incident-service:${BUILD_TAG}

                    trivy image --exit-code 1 --severity CRITICAL \
                        --format json --output trivy-report.json \
                        ${IMAGE_PREFIX}-ai-copilot:${BUILD_TAG} || true
                """
            }
            post {
                always {
                    archiveArtifacts artifacts: 'trivy-report.json', allowEmptyArchive: true
                }
            }
        }

        // ──────────────────────────────────────────────────────────
        // Stage 6: Push to Docker Hub
        // ──────────────────────────────────────────────────────────
        stage('Push Images') {
            when { branch 'main' }
            steps {
                withCredentials([string(credentialsId: 'dockerhub-password', variable: 'DOCKER_PASS')]) {
                    sh """
                        echo "${DOCKER_PASS}" | docker login ${DOCKER_REGISTRY} -u ramanred --password-stdin

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
        // Stage 7: Rolling Deploy to K3s
        // ──────────────────────────────────────────────────────────
        stage('Deploy to K3s') {
            when { branch 'main' }
            steps {
                withCredentials([file(credentialsId: 'k3s-kubeconfig', variable: 'KUBECONFIG')]) {
                    sh """
                        # Update image tags in manifests
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/incident-service.yml
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/ai-copilot.yml
                        sed -i 's|:latest|:${BUILD_TAG}|g' k8s/frontend.yml

                        # Apply all K8s manifests
                        kubectl apply -f k8s/

                        # Wait for rolling updates to complete
                        kubectl rollout status deployment/incident-service -n default --timeout=120s
                        kubectl rollout status deployment/ai-copilot-service -n default --timeout=120s
                        kubectl rollout status deployment/sre-frontend -n default --timeout=60s

                        echo "=== Deployment Successful ==="
                        kubectl get pods -n default
                    """
                }
            }
        }
    }

    post {
        success {
            echo "✅ Pipeline completed successfully for ${env.BRANCH_NAME} @ ${BUILD_TAG}"
        }
        failure {
            echo "❌ Pipeline FAILED for ${env.BRANCH_NAME} @ ${BUILD_TAG}"
        }
        always {
            cleanWs()
        }
    }
}
