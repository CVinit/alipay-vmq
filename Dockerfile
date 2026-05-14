FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /alipay-vmq ./cmd/alipay-vmq

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /alipay-vmq /usr/local/bin/alipay-vmq

EXPOSE 8081
ENTRYPOINT ["alipay-vmq"]
