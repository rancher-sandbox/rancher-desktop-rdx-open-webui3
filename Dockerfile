FROM golang:1.21-alpine AS builder
ARG COSMO_VERSION=3.9.2
ENV CGO_ENABLED=0
# Install necessary tools
RUN apk update && \
    apk add --no-cache curl unzip

WORKDIR /backend
COPY backend/go.* .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY backend/. .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o bin/service

WORKDIR /installer
COPY installer/go.* .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY installer/. .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-w" -o bin/installer-linux-amd64
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-w" -o bin/installer-linux-arm64
RUN curl --fail --location --output cosmocc.zip \
    https://github.com/jart/cosmopolitan/releases/download/${COSMO_VERSION}/cosmocc-${COSMO_VERSION}.zip
RUN unzip cosmocc.zip
ENV PATH="$PATH:/installer/bin"
# Not sure why we need the extra `#"`, but it seems to fix cosmo's bootstrap script.
RUN apelink -V linux -S '#"' -o bin/installer-linux bin/installer-linux-*
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o bin/installer-darwin-amd64
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/installer-darwin-arm64
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go run github.com/randall77/makefat@7ddd0e42c8442593c87c1705a5545099604008e5 \
    bin/installer-darwin bin/installer-darwin-*
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=windows go build -trimpath -ldflags="-s -w" -o bin/installer-windows.exe

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
COPY --from=builder /installer/bin/installer-linux /linux/installer
COPY --from=builder /installer/bin/installer-darwin /darwin/installer
COPY --from=builder /installer/bin/installer-windows.exe /windows/installer.exe
COPY /searxng/limiter.toml /linux/searxng/limiter.toml
COPY /searxng/settings.yml /linux/searxng/settings.yml
COPY /searxng/uwsgi.ini /linux/searxng/uwsgi.ini
CMD /service -socket /run/guest-services/backend.sock
