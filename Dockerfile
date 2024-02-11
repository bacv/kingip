FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

# Build gateway
WORKDIR /app/cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway .

# Build relay
WORKDIR /app/cmd/relay
RUN CGO_ENABLED=0 GOOS=linux go build -o relay .

# Build edge
WORKDIR /app/cmd/edge
RUN CGO_ENABLED=0 GOOS=linux go build -o edge .

FROM alpine:latest

COPY --from=builder /app/cmd/gateway /
COPY --from=builder /app/cmd/relay /
COPY --from=builder /app/cmd/edge /

ENTRYPOINT ["gateway"]


