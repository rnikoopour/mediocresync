FROM node:25.8.1-alpine AS web
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS builder
ARG VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/ui/dist ui/dist/
RUN touch ui/dist/.gitkeep
RUN GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o bin/mediocresync ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/bin/mediocresync ./mediocresync

ENV LISTEN_ADDR=:8080
ENV DB_PATH=/data/mediocresync.db
ENV LOG_LEVEL=info
VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["./mediocresync"]
