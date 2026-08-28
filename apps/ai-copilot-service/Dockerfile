# ─── Stage 1: Build ────────────────────────────────────────────────────────
FROM maven:3.9-eclipse-temurin-17 AS builder

WORKDIR /workspace

# Copy proto contract first (shared dependency)
COPY proto/ /workspace/proto/

# Copy service source
COPY apps/ai-copilot-service/pom.xml ./apps/ai-copilot-service/
COPY apps/ai-copilot-service/src    ./apps/ai-copilot-service/src

WORKDIR /workspace/apps/ai-copilot-service

# Download dependencies (layer-cached)
RUN mvn dependency:go-offline -q

# Build and package (skip tests during Docker build; tests run in CI)
RUN mvn clean package -DskipTests -q

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────
FROM eclipse-temurin:17-jre-alpine AS runtime

# Security: run as non-root
RUN addgroup -S sre && adduser -S sreuser -G sre
USER sreuser

WORKDIR /app

COPY --from=builder /workspace/apps/ai-copilot-service/target/*.jar app.jar

# Expose gRPC and management ports
EXPOSE 9090 8082

# JVM tuning for container environments
ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0 -XX:+ExitOnOutOfMemoryError"

ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]
