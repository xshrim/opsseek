FROM node:22-alpine AS frontend-builder

ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ARG NPM_REGISTRY=https://registry.npmmirror.com
WORKDIR /src
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories
RUN apk add --no-cache bash make
COPY Makefile ./
COPY frontend/package.json frontend/package-lock.json frontend/
RUN cd frontend && npm ci --registry="${NPM_REGISTRY}"
COPY frontend/ frontend/
RUN make frontend-build

FROM golang:1.26-alpine AS backend-source

ARG GOPROXY=https://goproxy.cn|https://proxy.golang.org|direct
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

ENV GOPROXY=${GOPROXY}
WORKDIR /src
RUN sed -i "s|https://dl-cdn.alpinelinux.org/alpine|${ALPINE_MIRROR}|g" /etc/apk/repositories
RUN apk add --no-cache bash make

COPY backend/go.mod backend/go.sum backend/
RUN cd backend && go mod download
COPY Makefile ./
COPY backend/ backend/

FROM backend-source AS backend-builder
COPY --from=frontend-builder /src/frontend/dist backend/webui/dist/
RUN make -o webui-assets backend-build VERSION="${VERSION}" COMMIT="${COMMIT}" BUILD_TIME="${BUILD_TIME}"

FROM alpine:3.22 AS runtime-base

# Repository previews and diagnostics use Git CLI for remote and Bundle reads.
RUN apk add --no-cache ca-certificates git openssh-client \
    && addgroup -g 65532 nonroot \
    && adduser -D -u 65532 -G nonroot nonroot \
    && git --version
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
COPY --from=backend-builder /src/backend/bin/opskeeper-api /app/opskeeper-api
ENTRYPOINT ["/app/opskeeper-api"]

FROM runtime-base AS app
EXPOSE 8080
COPY --from=backend-builder /src/backend/bin/opskeeper-app /app/opskeeper-app
ENTRYPOINT ["/app/opskeeper-app"]

FROM runtime-base AS worker
COPY --from=backend-builder /src/backend/bin/opskeeper-worker /app/opskeeper-worker
ENTRYPOINT ["/app/opskeeper-worker"]

FROM runtime-base AS scheduler
COPY --from=backend-builder /src/backend/bin/opskeeper-scheduler /app/opskeeper-scheduler
ENTRYPOINT ["/app/opskeeper-scheduler"]

FROM nginx:1.27-alpine AS frontend-runtime
COPY --from=frontend-builder /src/frontend/dist /usr/share/nginx/html
COPY deploy/nginx/frontend.conf /etc/nginx/conf.d/default.conf

FROM frontend-runtime AS frontend

FROM backend-source AS docker-mcp-builder
RUN make docker-mcp-build VERSION="${VERSION}" COMMIT="${COMMIT}" BUILD_TIME="${BUILD_TIME}"

FROM backend-source AS host-mcp-builder
RUN make host-mcp-build VERSION="${VERSION}" COMMIT="${COMMIT}" BUILD_TIME="${BUILD_TIME}"

FROM backend-source AS kubernetes-mcp-builder
RUN make kubernetes-mcp-build VERSION="${VERSION}" COMMIT="${COMMIT}" BUILD_TIME="${BUILD_TIME}"

FROM runtime-base AS docker-mcp
EXPOSE 8811
COPY --from=docker-mcp-builder /src/backend/bin/opskeeper-docker-mcp /app/opskeeper-docker-mcp
ENTRYPOINT ["/app/opskeeper-docker-mcp"]

FROM runtime-base AS host-mcp
EXPOSE 8813
COPY --from=host-mcp-builder /src/backend/bin/opskeeper-host-mcp-agent /app/opskeeper-host-mcp-agent
ENTRYPOINT ["/app/opskeeper-host-mcp-agent"]

FROM runtime-base AS kubernetes-mcp
EXPOSE 8812
COPY --from=kubernetes-mcp-builder /src/backend/bin/opskeeper-kubernetes-mcp /app/opskeeper-kubernetes-mcp
ENTRYPOINT ["/app/opskeeper-kubernetes-mcp"]
