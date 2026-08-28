# ─── Stage 1: Build ────────────────────────────────────────────────────────
FROM maven:3.9-eclipse-temurin-17 AS builder

WORKDIR /workspace

# Copy proto contract first
COPY proto/ /workspace/proto/

# Copy service source
COPY apps/incident-service/pom.xml ./apps/incident-service/
COPY apps/incident-service/src    ./apps/incident-service/src

WORKDIR /workspace/apps/incident-service

RUN mvn dependency:go-offline -q
RUN mvn clean package -DskipTests -q

# ─── Stage 2: Runtime ──────────────────────────────────────────────────────
FROM eclipse-temurin:17-jre-alpine AS runtime

RUN addgroup -S sre && adduser -S sreuser -G sre
USER sreuser

WORKDIR /app

COPY --from=builder /workspace/apps/incident-service/target/*.jar app.jar

# Expose REST API port and management port
EXPOSE 8081

ENV JAVA_OPTS="-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0 -XX:+ExitOnOutOfMemoryError"

ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]
