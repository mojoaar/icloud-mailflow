FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /mailflow ./cmd/mailflow

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /mailflow /usr/local/bin/mailflow
COPY web/static /app/web/static
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/mailflow", "-data=/data"]
