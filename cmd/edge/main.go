package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"sync"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc/edge"
)

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()

	relayAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:5555")
	if err != nil {
		log.Fatal(err)
	}

	handler := edge.NewEdge()

	config := quic.DialerConfig{
		Addr: relayAddr,
		Regions: map[string]string{
			"red": "http://red.com",
		},
	}

	dialer := quic.NewDialer(config, handler.RelayHandle)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := dialer.Dial(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
