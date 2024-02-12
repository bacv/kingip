package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc/edge"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	log.SetOutput(os.Stdout)

	var (
		hostname  string
		relayAddr string
		region    string
	)

	pflag.StringVar(&hostname, "hostname", "edge", "Hostname of the edge")
	pflag.StringVar(&relayAddr, "relayAddr", "127.0.0.1:5555", "UDP address for the relay")
	pflag.StringVar(&region, "region", "red", "Region of the edge")
	pflag.Parse()

	dialerConfig := quic.DialerConfig{
		Addr: relayAddr,
		Regions: map[string]string{
			region: hostname,
		},
	}

	if viper.IsSet("regions") {
		dialerConfig.Regions = viper.GetStringMapString("regions")
	}

	spawn(dialerConfig)
}

func spawn(dialerConfig quic.DialerConfig) {
	handler := edge.NewEdge()
	dialer := quic.NewDialer(dialerConfig, handler.RelayHandle)

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
