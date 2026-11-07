# App image (design doc item 7). The node:, golang: and pnpm@ versions are checked by make check-pins.
# Every base image carries its digest (the multi-arch index), so a tag moved upstream is never built
# (guard TestSupplyChainPins); Dependabot's docker entry for / bumps the distroless one.
ARG VERSION=dev

FROM node:26.13.0-alpine@sha256:8996bb5821d62a8fd9bb27c1cb07462f3fdce6e47d640c6a881ead0b064ce216 AS web
ARG VERSION
RUN npm install -g pnpm@12.16.1
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN VERSION=$VERSION pnpm build

FROM golang:1.27.3-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS go
ARG VERSION
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/
COPY --from=web /src/web/build web/build
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$VERSION" -o /out/open-customer ./cmd/open-customer \
	&& mkdir -p /out/data/media /out/backup-status

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=go /out/open-customer /usr/local/bin/open-customer
COPY --from=go --chown=65532:65532 /out/data /data
COPY --from=go --chown=65532:65532 /out/backup-status /backup-status
# The base image's nonroot user, written out so the config scan (Trivy DS-0002) sees it.
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/open-customer"]
CMD ["serve"]
