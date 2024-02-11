package main

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/bacv/kingip/lib/quic"
	"github.com/bacv/kingip/svc"
	"github.com/bacv/kingip/svc/gateway"
	"github.com/bacv/kingip/svc/store"
	"github.com/namsral/flag"
	"github.com/spf13/viper"
)

func main() {
	log.SetOutput(os.Stdout)

	listenRelayAddrString := flag.String("listenRelay", "127.0.0.1:4444", "Address for relay listener")
	listenProxyAddrString := flag.String("listenProxy", "127.0.0.1:10700", "Address for user cons")
	region := flag.String("region", "red", "Gateway region")
	configFile := flag.String("config", "", "Path to config file")

	flag.Parse()

	if *configFile != "" {
		*listenRelayAddrString = viper.GetString("listenRelayAddr")
		*listenProxyAddrString = viper.GetString("listenProxyAddr")
		*region = viper.GetString("region")
	}

	listenRelayAddr, err := net.ResolveUDPAddr("udp", *listenRelayAddrString)
	if err != nil {
		log.Fatal(err)
	}

	listenUserRedAddr, err := net.ResolveTCPAddr("tcp", *listenProxyAddrString)
	if err != nil {
		log.Fatal(err)
	}

	listenerConfig := quic.ListenerConfig{
		Addr: listenRelayAddr,
	}

	proxyConfig := gateway.ProxyConfig{
		Region: svc.Region(*region),
		Addr:   listenUserRedAddr,
	}

	spawn(proxyConfig, listenerConfig)
}

func spawn(proxyConfig gateway.ProxyConfig, listenerConfig quic.ListenerConfig) {
	testUser := svc.NewUser("user", 1, svc.DefaultUserConfig())
	testUserAuth := svc.UserAuth{Name: testUser.Name(), Password: "pass"}

	mockStore := store.NewMockUserStore()
	mockStore.Users[testUserAuth] = testUser
	mockSessionStore := store.NewMockSessionStore()

	handler := gateway.NewGateway(mockStore, mockStore, mockSessionStore)

	listener := quic.NewListener(
		context.Background(),
		listenerConfig,
		handler.RegisterHandle,
		handler.RegionsHandle,
		handler.CloseHandle,
	)

	proxy, err := gateway.NewProxyServer(
		proxyConfig,
		handler.AuthHandle,
		handler.SessionHandle,
	)

	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := listener.Listen()
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		defer wg.Done()
		err := proxy.ListenUser()
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
