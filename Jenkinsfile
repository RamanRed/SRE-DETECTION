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
                            docker run --rm \
                              --volumes-from jenkins \
                              -w ${WORKSPACE}/apps/incident-service \
                              maven:3.9-eclipse-temurin-17 \
                              sh -c "mvn clean test -DskipTests=false --no-transfer-progress && chmod -R 777 ${WORKSPACE}"
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
                            docker run --rm \
                              --volumes-from jenkins \
                              -w ${WORKSPACE}/apps/ai-copilot-service \
                              maven:3.9-eclipse-temurin-17 \
                              sh -c "mvn clean test -DskipTests=false --no-transfer-progress && chmod -R 777 ${WORKSPACE}"
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

                        # Deploy via containerized kubectl to AWS EC2 K3s cluster
                        docker run --rm \
                          --volumes-from jenkins \
                          -e KUBECONFIG=${KUBECONFIG_FILE} \
                          bitnami/kubectl:latest \
                          apply -f ${WORKSPACE}/k8s/

                        echo '=== Deployment to AWS K3s Cluster Successful ==='
                        docker run --rm \
                          --volumes-from jenkins \
                          -e KUBECONFIG=${KUBECONFIG_FILE} \
                          bitnami/kubectl:latest \
                          get pods -n sre-copilot || true
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
