FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /immortal ./cmd/immortal

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /immortal /usr/local/bin/immortal

EXPOSE 7777
VOLUME /data

ENTRYPOINT ["immortal"]
CMD ["start", "--data-dir", "/data"]
