package main

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc/relay"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	log.SetOutput(os.Stdout)

	var (
		hostname       string
		listenAddr     string
		gateways       []string
		regions        []string
		configFile     string
		dialerConfigs  []quic.DialerConfig
		listenerConfig quic.ListenerConfig
	)

	pflag.StringVar(&hostname, "hostname", "relay", "Hostname of the relay")
	pflag.StringVar(&listenAddr, "listenAddr", "127.0.0.1:5555", "Address for edge conns")
	pflag.StringArray("gateways", gateways, "Addresses of gateways")
	pflag.StringArray("regions", regions, "Relay regions")
	pflag.StringVar(&configFile, "config", "", "Path to config file")
	pflag.Parse()

	viper.BindPFlag("hostname", pflag.Lookup("hostname"))
	viper.BindPFlag("listenAddr", pflag.Lookup("listenAddr"))
	viper.BindPFlag("gateways", pflag.Lookup("gateways"))
	viper.BindPFlag("regions", pflag.Lookup("regions"))
	viper.SetConfigFile(configFile)

	if configFile != "" {
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file: %s", err)
		}
		if err := viper.UnmarshalKey("regions", &regions); err != nil {
			log.Fatalf("Error unmarshaling regions configuration: %s", err)
		}
		if err := viper.UnmarshalKey("gateways", &gateways); err != nil {
			log.Fatalf("Error unmarshaling gateways configuration: %s", err)
		}
		regions = viper.GetStringSlice("regions")
	}

	regions = viper.GetStringSlice("regions")
	gateways = viper.GetStringSlice("gateways")

	dialerRegions := make(map[string]string)
	for _, region := range regions {
		dialerRegions[region] = hostname
	}
	for _, addr := range gateways {
		dialerConfig := quic.DialerConfig{
			Addr:    addr,
			Regions: dialerRegions,
		}
		dialerConfigs = append(dialerConfigs, dialerConfig)
	}

	listenerConfig = quic.ListenerConfig{
		Addr: listenAddr,
	}

	handler := relay.NewRelay()

	var wg sync.WaitGroup
	spawnListener(&wg, listenerConfig, handler)
	spawnDialers(&wg, dialerConfigs, handler)
	wg.Wait()
}

func spawnDialers(wg *sync.WaitGroup, dialerConfigs []quic.DialerConfig, handler *relay.Relay) {
	for _, cfg := range dialerConfigs {
		wg.Add(1)
		go func(cfg quic.DialerConfig) {
			defer wg.Done()

			dialer := quic.NewDialer(cfg, handler.GatewayHandle)
			err := dialer.Dial(context.Background())
			if err != nil {
				log.Fatal(err)
			}
		}(cfg)
	}
}
func spawnListener(wg *sync.WaitGroup, listenerConfig quic.ListenerConfig, handler *relay.Relay) {
	listener := quic.NewListener(
		context.Background(),
		listenerConfig,
		handler.RegisterHandle,
		handler.RegionsHandle,
		handler.CloseHandle,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := listener.Listen()
		if err != nil {
			log.Fatal(err)
		}
	}()
}
