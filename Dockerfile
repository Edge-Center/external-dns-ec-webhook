FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/root/.cache/go \
    GOOS=linux CGO_ENABLED=0 go mod download

COPY . .

ARG VERSION=latest
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X main.Version=${VERSION}" -o external-dns-ec-webhook .


FROM alpine:3.18

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/external-dns-ec-webhook /usr/local/bin/external-dns-ec-webhook

RUN chmod +x /usr/local/bin/external-dns-ec-webhook && \
    chown 65532:65532 /usr/local/bin/external-dns-ec-webhook

USER 65532

CMD ["external-dns-ec-webhook"]