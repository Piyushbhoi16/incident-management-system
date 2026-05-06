FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ims-api ./cmd/api

FROM alpine:3.22

RUN addgroup -S ims && adduser -S ims -G ims

WORKDIR /app
COPY --from=builder /bin/ims-api /app/ims-api

USER ims

EXPOSE 8080

ENTRYPOINT ["/app/ims-api"]
