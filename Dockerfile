FROM golang:alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /mailflow ./cmd/mailflow

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /mailflow /usr/local/bin/mailflow
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/mailflow", "-data=/data"]
