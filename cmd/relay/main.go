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
	"github.com/spf13/viper"
)

func main() {
	log.SetOutput(os.Stdout)

	dialGateway := flag.String("dialGateway", "127.0.0.1:4444", "Address to dial gateway")
	listenRelay := flag.String("listenRelay", "127.0.0.1:5555", "Address for relay listener")
	configFile := flag.String("config", "", "Path to config file")

	flag.Parse()

	if *configFile != "" {
		viper.SetConfigFile(*configFile)
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file, %s", err)
		}
		dialGatewayAddr := viper.GetString("dialGatewayAddr")
		if dialGatewayAddr != "" {
			*dialGateway = dialGatewayAddr
		}
		listenRelayAddr := viper.GetString("listenRelayAddr")
		if listenRelayAddr != "" {
			*listenRelay = listenRelayAddr
		}
	}

	dialGatewayAddr, err := net.ResolveUDPAddr("udp", *dialGateway)
	if err != nil {
		log.Fatal(err)
	}

	listenRelayAddr, err := net.ResolveUDPAddr("udp", *listenRelay)
	if err != nil {
		log.Fatal(err)
	}

	dialerConfig := quic.DialerConfig{
		Addr: dialGatewayAddr,
		Regions: map[string]string{
			"blue": viper.GetString("regions.blue"),
			"red":  viper.GetString("regions.red"),
		},
	}

	listenerConfig := quic.ListenerConfig{
		Addr: listenRelayAddr,
	}

	spawn(dialerConfig, listenerConfig)
}

func spawn(dialerConfig quic.DialerConfig, listenerConfig quic.ListenerConfig) {
	handler := relay.NewRelay()
	dialer := quic.NewDialer(dialerConfig, handler.GatewayHandle)

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
