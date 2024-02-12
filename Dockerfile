FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

WORKDIR /app/cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build

WORKDIR /app/cmd/relay
RUN CGO_ENABLED=0 GOOS=linux go build

WORKDIR /app/cmd/edge
RUN CGO_ENABLED=0 GOOS=linux go build

WORKDIR /app/cmd/curl
RUN CGO_ENABLED=0 GOOS=linux go build

FROM alpine:latest

COPY --from=builder /app/cmd/gateway/gateway /gateway
COPY --from=builder /app/cmd/relay/relay /relay
COPY --from=builder /app/cmd/edge/edge /edge
COPY --from=builder /app/cmd/curl/curl /curl

ENTRYPOINT ["gateway"]
