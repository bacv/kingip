package main

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/bacv/kingip/svc"
	"github.com/bacv/kingip/svc/gateway"
	"github.com/bacv/kingip/svc/store"
	"github.com/namsral/flag"
)

func main() {
	log.SetOutput(os.Stdout)
	flag.Parse()

	testUser := svc.NewUser("user", 1, svc.DefaultUserConfig())
	testUserAuth := svc.UserAuth{Name: testUser.Name(), Password: "pass"}

	userStore := store.NewMockUserStore()
	userStore.Users[testUserAuth] = testUser

	handler := gateway.NewGateway(userStore)

	listenRelayAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4242")
	if err != nil {
		log.Fatal(err)
	}

	listenUserRedAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:10700")
	if err != nil {
		log.Fatal(err)
	}

	// listenUserGreenAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:10070")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// listenUserBlueAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:10007")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// listenUserYellowAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:10770")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	serverConfig := gateway.ServerConfig{
		Addr: listenRelayAddr,
	}

	proxyConfig := gateway.ProxyConfig{
		Region: "red",
		Addr:   listenUserRedAddr,
	}

	server, err := gateway.NewServer(
		context.Background(),
		serverConfig,
		handler.RegisterHandle,
		handler.RegionsHandle,
	)

	if err != nil {
		log.Fatal(err)
	}

	proxy, err := gateway.NewProxyServer(
		proxyConfig,
		handler.AuthHandle,
		handler.SessionHandle,
	)

	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := server.ListenRelay()
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := proxy.ListenUser()
		if err != nil {
			log.Fatal(err)
		}
	}()

	wg.Wait()
}
