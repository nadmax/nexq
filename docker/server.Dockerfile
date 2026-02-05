FROM golang:1.25.7-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o server ./cmd/server/

FROM alpine:3.23 AS final
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/server ./
COPY web ./web/
ENTRYPOINT [ "./server" ]
