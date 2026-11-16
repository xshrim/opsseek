# Package artifacts built on the host; no source compilation in this file.
FROM alpine:3.22 AS runtime-base
RUN apk add --no-cache ca-certificates \
    && addgroup -g 65532 nonroot \
    && adduser -D -u 65532 -G nonroot nonroot
USER 65532:65532
ENV HOME=/home/nonroot
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.revision=${COMMIT} \
      org.opencontainers.image.created=${BUILD_TIME}

FROM runtime-base AS api
EXPOSE 8080
COPY backend/bin/opsseek-api /app/opsseek-api
ENTRYPOINT ["/app/opsseek-api"]

FROM runtime-base AS app
EXPOSE 8080
COPY backend/bin/opsseek-app /app/opsseek-app
ENTRYPOINT ["/app/opsseek-app"]

FROM runtime-base AS worker
COPY backend/bin/opsseek-worker /app/opsseek-worker
ENTRYPOINT ["/app/opsseek-worker"]

FROM runtime-base AS scheduler
COPY backend/bin/opsseek-scheduler /app/opsseek-scheduler
ENTRYPOINT ["/app/opsseek-scheduler"]

FROM nginx:stable-alpine-slim AS frontend
COPY frontend/dist/ /usr/share/nginx/html/
COPY deploy/nginx/frontend.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]

FROM runtime-base AS docker-mcp
EXPOSE 8811
COPY backend/bin/opsseek-docker-mcp /app/opsseek-docker-mcp
ENTRYPOINT ["/app/opsseek-docker-mcp"]

FROM runtime-base AS host-mcp
EXPOSE 8813
COPY backend/bin/opsseek-host-mcp-agent /app/opsseek-host-mcp-agent
ENTRYPOINT ["/app/opsseek-host-mcp-agent"]

FROM runtime-base AS kubernetes-mcp
EXPOSE 8812
COPY backend/bin/opsseek-kubernetes-mcp /app/opsseek-kubernetes-mcp
ENTRYPOINT ["/app/opsseek-kubernetes-mcp"]
