FROM golang:1.22-alpine AS builder
RUN apk --no-cache add git build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -ldflags="-w -s" -o /app/server ./cmd/main.go

FROM alpine:3.19

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/server /app/server

USER appuser

EXPOSE 3000
EXPOSE 3001

ENTRYPOINT ["/app/server"]
