FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/api /usr/local/bin/api
COPY configs/ /etc/nullus/configs/
COPY db/migrations/ /etc/nullus/migrations/
EXPOSE 8080
CMD ["api"]
