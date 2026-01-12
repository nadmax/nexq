FROM golang:1.25.5-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o worker ./cmd/worker/

FROM alpine:3.22 AS final
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/worker ./
ENTRYPOINT [ "./worker" ]
