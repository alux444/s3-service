FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/s3-service ./cmd/s3-service

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=builder /out/s3-service /app/s3-service
COPY --from=builder /src/db/migrations /app/db/migrations

EXPOSE 8080

USER app

ENTRYPOINT ["/app/s3-service"]
