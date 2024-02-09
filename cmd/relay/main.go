package main

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/bacv/kingip/svc/relay"
	"github.com/namsral/flag"
)

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()

	gatewayAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4242")
	if err != nil {
		log.Fatal(err)
	}

	handler := relay.NewRelay()

	config := relay.Config{
		DialGatewayAddr:  gatewayAddr,
		ListenClientAddr: gatewayAddr,
		Regions: map[string]string{
			"blue": "http://blue.com",
			"red":  "http://red.com",
		},
	}

	server := relay.NewServer(
		config,
		handler.ClientHandle,
		handler.GatewayHandle,
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := server.DialGateway(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
