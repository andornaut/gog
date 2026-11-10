# syntax=docker/dockerfile:1.25
ARG GO_VERSION
FROM koalaman/shellcheck:v0.11.0 AS shellcheck
FROM node:24.22.2 AS markdown
RUN npm install --global --ignore-scripts --no-audit --no-fund markdownlint-cli2@0.24.0
FROM golangci/golangci-lint:v2.15.0 AS golangci
FROM goreleaser/goreleaser:v2.20.0 AS goreleaser
FROM golang:${GO_VERSION}
COPY --from=shellcheck /bin/shellcheck /usr/local/bin/shellcheck
COPY --from=golangci /usr/bin/golangci-lint /usr/local/bin/golangci-lint
COPY --from=goreleaser /usr/bin/goreleaser /usr/local/bin/goreleaser
COPY --from=markdown /usr/local/bin/node /usr/local/bin/node
COPY --from=markdown /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN ln -s /usr/local/lib/node_modules/markdownlint-cli2/markdownlint-cli2-bin.mjs /usr/local/bin/markdownlint-cli2
WORKDIR /opt/module
COPY go.mod go.sum ./
RUN GOMODCACHE=/opt/go-mod go mod download
COPY tests/container/check /opt/check/run
RUN chmod 755 /opt/check/run
