FROM node:20-alpine AS frontend
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.23-alpine AS backend
RUN apk add --no-cache git ca-certificates
WORKDIR /app
COPY go.mod ./
COPY . .
COPY --from=frontend /web/dist /app/internal/web/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /copilot-proxy ./cmd/copilot-proxy

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /copilot-proxy .
COPY config.example.json .
EXPOSE 15432
ENTRYPOINT ["/app/copilot-proxy"]
