package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc"
	"github.com/bacv/kingip/svc/gateway"
	"github.com/bacv/kingip/svc/store"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	log.SetOutput(os.Stdout)

	var (
		listenRelayAddr string
		listenProxyAddr string
		region          string
		configFile      string
		listenerConfig  quic.ListenerConfig
		proxyConfigs    []gateway.ProxyConfig
	)

	pflag.StringVar(&listenRelayAddr, "listenRelayAddr", "127.0.0.1:4444", "Address for relay listener")
	pflag.StringVar(&listenProxyAddr, "listenProxyAddr", "127.0.0.1:10700", "Address for user cons")
	pflag.StringVar(&region, "region", "red", "Default gateway region")
	pflag.StringVar(&configFile, "config", "", "Path to config file")
	pflag.Parse()

	viper.BindPFlag("listenRelayAddr", pflag.Lookup("listenRelayAddr"))
	viper.BindPFlag("listenProxyAddr", pflag.Lookup("listenProxyAddr"))
	viper.BindPFlag("region", pflag.Lookup("region"))
	viper.SetConfigFile(configFile)

	if configFile != "" {
		if err := viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file: %s", err)
		}
		listenerConfig = quic.ListenerConfig{
			Addr: listenRelayAddr,
		}
		if err := viper.UnmarshalKey("proxies", &proxyConfigs); err != nil {
			log.Fatalf("Error unmarshaling proxies configuration: %s", err)
		}
	} else {
		listenerConfig = quic.ListenerConfig{
			Addr: listenRelayAddr,
		}
		proxyConfig := gateway.ProxyConfig{
			Region: svc.Region(region),
			Addr:   listenProxyAddr,
		}
		proxyConfigs = append(proxyConfigs, proxyConfig)
	}

	testUser := svc.NewUser("user", 1, svc.DefaultUserConfig())
	testUserAuth := svc.UserAuth{Name: testUser.Name(), Password: "pass"}

	unlimitedUser := svc.NewUser("unlimited", 2, svc.NewUserConfig(65000, 1_000_000_000, 24*time.Hour))
	unlimitedUserAuth := svc.UserAuth{Name: unlimitedUser.Name(), Password: "pass"}

	mockStore := store.NewMockUserStore()
	mockStore.Users[testUserAuth] = testUser
	mockStore.Users[unlimitedUserAuth] = unlimitedUser
	mockSessionStore := store.NewMockSessionStore()

	handler := gateway.NewGateway(mockStore, mockStore, mockSessionStore)

	var wg sync.WaitGroup
	spawnListener(&wg, listenerConfig, handler)
	spawnProxies(&wg, proxyConfigs, handler)
	wg.Wait()
}

func spawnProxies(wg *sync.WaitGroup, proxyConfigs []gateway.ProxyConfig, handler *gateway.Gateway) {
	for _, cfg := range proxyConfigs {
		wg.Add(1)
		go func(cfg gateway.ProxyConfig) {
			defer wg.Done()

			proxy, err := gateway.NewProxyServer(
				cfg,
				handler.AuthHandle,
				handler.SessionHandle,
			)
			if err != nil {
				log.Fatalf("Failed to start proxy for region %s: %v", cfg.Region, err)
			}

			err = proxy.ListenUser()
			if err != nil {
				log.Fatalf("Failed to listen on proxy for region %s: %v", cfg.Region, err)
			}
		}(cfg)
	}
}

func spawnListener(wg *sync.WaitGroup, listenerConfig quic.ListenerConfig, handler *gateway.Gateway) {
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
