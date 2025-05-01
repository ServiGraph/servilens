# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG MAIN_PATH
RUN go build -o /app/server ${MAIN_PATH}

# Final image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .

CMD ["./server"]