# Multi-stage build. Produces a ~20 MB static image.

FROM golang:1.25-alpine AS builder

ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w \
      -X github.com/Nagendhra-web/Immortal/internal/version.Version=${VERSION} \
      -X github.com/Nagendhra-web/Immortal/internal/version.GitCommit=${COMMIT} \
      -X github.com/Nagendhra-web/Immortal/internal/version.BuildDate=${DATE}" \
    -o /immortal ./cmd/immortal

FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata \
 && addgroup -S immortal && adduser -S immortal -G immortal

COPY --from=builder /immortal /usr/local/bin/immortal

USER immortal
EXPOSE 7777
VOLUME /data

HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -q --spider http://127.0.0.1:7777/api/health || exit 1

ENTRYPOINT ["immortal"]
CMD ["start", "--data-dir", "/data", "--api-port", "7777"]
