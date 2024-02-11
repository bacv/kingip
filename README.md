# KingIP

A proxy server that can handle multiple exit points.

## How it works

When a user connects via the proxy to the destination server (ie. `curl -x "http://user:pass@localhost:10700" https://httpbin.org/ip -L -v`), the request traverses these services:
    - Gateway decides to which region the request needs to be passed by the port the user connects to (ie. `:10700` is "red" region in the example;
    - Gateway then checks if it has any relay connections and creates new `quic` stream with the random relay it chooses;
    - Relay gets a proxy request from the gateway and checks if it has any edge connections for a chosen region;
    - Relay if edge connection is available, then a new `quic` stream is created with the edge;
    - Once a "tunnel" from gateway to relay to edge is established (via `quic` streams) then a inbound and outbound data is passed around from user to destination;

## How to run

To run, go to `cmd/{gateway,relay,edge}`folders and build 3 binaries. After running gateway, relay and edge - connect to the gateway via `http://user:pass@localhost:10700`. To add more users, modify the `cmd/gateway/main.go` mock storage.

## "Curl" util

`cmd/curl` has the ability to run multiple requests at once, it tries to connect to the gateway with a default configuration.

