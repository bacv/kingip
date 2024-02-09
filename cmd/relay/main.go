package main

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc/relay"
	"github.com/namsral/flag"
)

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()

	dialGatewayAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4444")
	if err != nil {
		log.Fatal(err)
	}

	listenRelayAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:5555")
	if err != nil {
		log.Fatal(err)
	}

	handler := relay.NewRelay()

	dialerConfig := quic.DialerConfig{
		Addr: dialGatewayAddr,
		Regions: map[string]string{
			"blue": "http://blue.com",
			"red":  "http://red.com",
		},
	}

	dialer := quic.NewDialer(dialerConfig, handler.GatewayHandle)

	listenerConfig := quic.ListenerConfig{
		Addr: listenRelayAddr,
	}

	listener := quic.NewListener(
		context.Background(),
		listenerConfig,
		handler.RegisterHandle,
		handler.RegionsHandle,
		handler.CloseHandle,
	)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := dialer.Dial(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		err := listener.Listen()
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
