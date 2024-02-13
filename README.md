# KingIP

A proxy server that can handle multiple exit points.

## How it works

When a user connects via the proxy to the destination server (ie. `curl -x "http://user:pass@localhost:11700" https://httpbin.org/ip -L -v`), the request traverses these services:
  - Gateway decides to which region the request needs to be passed by the port the user connects to (ie. `:11700` is "red" region in the example;
  - Gateway then checks if it has any relay connections and creates new `quic` stream with the random relay it chooses;
  - Relay gets a proxy request from the gateway and checks if it has any edge connections for a chosen region;
  - Relay, if there are any edge connections from that region, creates a new `quic` strem with a chosen edge;
  - Once a "tunnel" from gateway to relay to edge is established - an inbound and outbound data is passed around from user to destination;

## How to run

To run, go to `cmd/{gateway,relay,edge}`folders and build 3 binaries. 

Start these services with the default configuration:
```bash
./cmd/gateway/gateway --config ./cmd/gateway/config.yml
./cmd/relay/relay --config ./cmd/relay/config.yml
./cmd/edge/edge
```

After running gateway, relay and edge - connect to the gateway via `http://user:pass@localhost:11700`:
```bash
curl -x http://user:pass@localhost:11700 http://httpbin.org/ip -L -v
```

### Docker Compose

After running `docker compose up`, user should be able to proxy requests through tree regions: red, green, blue and yellow.

```bash
# Red
curl -x http://user:pass@localhost:11700 http://httpbin.org/ip -L -v

# Green
curl -x http://user:pass@localhost:11070 http://httpbin.org/ip -L -v

# Blue
curl -x http://user:pass@localhost:11007 http://httpbin.org/ip -L -v

# Yellow
curl -x http://user:pass@localhost:11770 http://httpbin.org/ip -L -v
```

To connect to host network via the docker compose proxy use `host.docker.internal` instead of `localhost` in the curl request:
```bash
# Connects to localhost:8080 on host system
curl -x http://user:pass@localhost:11770 http://host.docker.internal:8080/test -L -v
```


## "Curl" util

`cmd/curl` has ability to run multiple requests at once. After building it in `cmd/curl` directory:

```bash
# Usage of `curl`:
#       --parallel int       Number of parallel calls (default 11)
#       --proxy string       Proxy URL (default "http://user:pass@localhost:11700")
#       --timeout duration   Request timeout in seconds (default 10s)
#       --url string         Target URL to request (default "http://httpbin.org/ip")

./cmd/curl/curl -proxy http://user:pass@localhost:11700 -url http://httpbin.org/ip 
```
