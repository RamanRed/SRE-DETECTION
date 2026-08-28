# ─── Stage 1: Build ────────────────────────────────────────────────────────
FROM node:20-alpine AS builder

WORKDIR /app

COPY apps/frontend/package*.json ./
RUN npm ci --silent

COPY apps/frontend/ ./
RUN npm run build

# ─── Stage 2: Nginx Reverse Proxy ──────────────────────────────────────────
FROM nginx:1.27-alpine AS runtime

# Remove default config
RUN rm /etc/nginx/conf.d/default.conf

COPY --from=builder /app/dist /usr/share/nginx/html

# Nginx config: proxy /api → incident-service, serve React SPA
COPY apps/frontend/nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD wget -qO- http://localhost/health || exit 1

CMD ["nginx", "-g", "daemon off;"]
