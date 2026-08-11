FROM golang:1.26.5-alpine AS builder

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /app/mw-email-server \
    ./cmd/server

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o /app/create-admin \
    ./cmd/create-admin

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/mw-email-server /app/mw-email-server
COPY --from=builder /app/create-admin /app/create-admin

RUN mkdir -p /app/data

EXPOSE 8082

CMD ["/app/mw-email-server"]