FROM golang:1.21-alpine AS builder
ENV CGO_ENABLED=0
WORKDIR /backend
COPY backend/go.* .
# Install necessary tools
RUN apk update && \
    apk add --no-cache curl unzip

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY backend/. .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o bin/service

FROM --platform=$BUILDPLATFORM node:21.6-alpine3.18 AS client-builder
WORKDIR /ui
# cache packages in layer
COPY ui/package.json /ui/package.json
COPY ui/package-lock.json /ui/package-lock.json
RUN --mount=type=cache,target=/usr/src/app/.npm \
    npm set cache /usr/src/app/.npm && \
    npm ci
# install
COPY ui /ui
RUN npm run build

FROM alpine
LABEL org.opencontainers.image.title="Open WebUI" \
    org.opencontainers.image.description="Open WebUI on Rancher Desktop" \
    org.opencontainers.image.vendor="SUSE LLC" \
    com.docker.desktop.extension.api.version="0.3.4" \
    com.docker.extension.screenshots="" \
    com.docker.desktop.extension.icon="" \
    com.docker.extension.detailed-description="" \
    com.docker.extension.publisher-url="" \
    com.docker.extension.additional-urls="" \
    com.docker.extension.categories="" \
    com.docker.extension.changelog=""

COPY --from=builder /backend/bin/service /
COPY docker-compose.yaml .
COPY metadata.json .
COPY open-webui.svg .
COPY --from=client-builder /ui/build ui
COPY --chmod=0755 binaries/unix/install-ollama.sh /linux/install-ollama.sh
COPY --chmod=0755 binaries/unix/install-ollama.sh /darwin/install-ollama.sh
COPY --chmod=0755 binaries/windows/install-ollama.exe /windows/install-ollama.exe
COPY /searxng/limiter.toml /linux/searxng/limiter.toml
COPY /searxng/settings.yml /linux/searxng/settings.yml
COPY /searxng/uwsgi.ini /linux/searxng/uwsgi.ini
CMD /service -socket /run/guest-services/backend.sock
