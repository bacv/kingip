package main

import (
	"context"
	"flag"
	"log"
	"os"
	"sync"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc/edge"
	"github.com/spf13/viper"
)

func main() {
	log.SetOutput(os.Stdout)

	var (
		relayAddrFlag string
		configFile    string
	)

	flag.StringVar(&relayAddrFlag, "relayAddr", "127.0.0.1:5555", "UDP address for the relay")
	flag.StringVar(&configFile, "config", "", "Path to configuration file")
	flag.Parse()

	if configFile != "" {
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			log.Fatalf("Error reading config file: %v", err)
		}
	}

	relayAddr := viper.GetString("relayAddr")
	if relayAddr == "" {
		relayAddr = relayAddrFlag
	}

	dialerConfig := quic.DialerConfig{
		Addr: relayAddr,
		Regions: map[string]string{
			"red": "http://red.com",
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
