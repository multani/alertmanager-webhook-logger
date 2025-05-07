FROM golang:1.24.3 as builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -installsuffix 'static' .

FROM scratch

COPY --from=builder /app/alertmanager-webhook-logger /alertmanager-webhook-logger

ENTRYPOINT ["/alertmanager-webhook-logger"]
