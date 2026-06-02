# Stage 1: Build Go binaries
FROM golang:1.26-alpine AS builder
WORKDIR /app
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ceddit .
RUN CGO_ENABLED=0 GOOS=linux go build -o ceddit-reconciler ./cmd/reconciler

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/ceddit /app/ceddit
COPY --from=builder /app/ceddit-reconciler /app/ceddit-reconciler
COPY --from=builder /app/sql /app/sql
EXPOSE 8081
CMD ["/app/ceddit"]
