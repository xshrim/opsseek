# Package artifacts built on the host; no source compilation in this file.
FROM golang:1.26-alpine AS runtime-base
RUN apk add --no-cache ca-certificates git openssh-client \
    && addgroup -g 65532 nonroot \
    && adduser -D -u 65532 -G nonroot nonroot
USER 65532:65532
ENV HOME=/home/nonroot GIT_TERMINAL_PROMPT=0
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.revision=${COMMIT} \
      org.opencontainers.image.created=${BUILD_TIME}

FROM runtime-base AS api
EXPOSE 8080
COPY backend/bin/opskeeper-api /app/opskeeper-api
ENTRYPOINT ["/app/opskeeper-api"]

FROM runtime-base AS app
EXPOSE 8080
COPY backend/bin/opskeeper-app /app/opskeeper-app
ENTRYPOINT ["/app/opskeeper-app"]

FROM runtime-base AS worker
COPY backend/bin/opskeeper-worker /app/opskeeper-worker
ENTRYPOINT ["/app/opskeeper-worker"]

FROM runtime-base AS scheduler
COPY backend/bin/opskeeper-scheduler /app/opskeeper-scheduler
ENTRYPOINT ["/app/opskeeper-scheduler"]

FROM node:22-alpine AS frontend
RUN apk add --no-cache nginx
COPY frontend/dist/ /usr/share/nginx/html/
COPY deploy/nginx/frontend.conf /etc/nginx/http.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]

FROM runtime-base AS docker-mcp
EXPOSE 8811
COPY backend/bin/opskeeper-docker-mcp /app/opskeeper-docker-mcp
ENTRYPOINT ["/app/opskeeper-docker-mcp"]

FROM runtime-base AS host-mcp
EXPOSE 8813
COPY backend/bin/opskeeper-host-mcp-agent /app/opskeeper-host-mcp-agent
ENTRYPOINT ["/app/opskeeper-host-mcp-agent"]

FROM runtime-base AS kubernetes-mcp
EXPOSE 8812
COPY backend/bin/opskeeper-kubernetes-mcp /app/opskeeper-kubernetes-mcp
ENTRYPOINT ["/app/opskeeper-kubernetes-mcp"]
