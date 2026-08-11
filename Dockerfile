FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/mw-email-server ./cmd/server

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/mw-email-server /app/mw-email-server

RUN mkdir -p /app/data

EXPOSE 8080

CMD ["/app/mw-email-server"]
