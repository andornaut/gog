# syntax=docker/dockerfile:1.25
ARG GO_VERSION
FROM koalaman/shellcheck:v0.11.0 AS shellcheck
FROM node:24.22.2 AS markdown
WORKDIR /opt/markdown
COPY tests/container/package.json tests/container/package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
FROM golangci/golangci-lint:v2.15.0 AS golangci
FROM goreleaser/goreleaser:v2.20.0 AS goreleaser
FROM golang:${GO_VERSION}
COPY --from=shellcheck /bin/shellcheck /usr/local/bin/shellcheck
COPY --from=golangci /usr/bin/golangci-lint /usr/local/bin/golangci-lint
COPY --from=goreleaser /usr/bin/goreleaser /usr/local/bin/goreleaser
COPY --from=markdown /usr/local/bin/node /usr/local/bin/node
COPY --from=markdown /opt/markdown /opt/markdown
ENV PATH=/opt/markdown/node_modules/.bin:${PATH}
WORKDIR /opt/module
COPY go.mod go.sum ./
RUN GOMODCACHE=/opt/go-mod go mod download
COPY tests/container/check /opt/check/run
RUN chmod 755 /opt/check/run
