# services/ctp_tick/Dockerfile
# CTP 行情订阅服务 — 多阶段构建（Linux cgo 编译）
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# CTP cgo 编译（仅在 Linux 容器内可行）
RUN CGO_ENABLED=1 GOOS=linux go build -tags ctp -o /ctp_tick ./cmd/main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY --from=builder /ctp_tick /usr/local/bin/ctp_tick
USER 1000:1000
EXPOSE 8081
CMD ["ctp_tick"]
