package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/pflag"
)

var (
	parallelCalls  int
	targetURL      string
	proxyURL       string
	requestTimeout time.Duration
)

func main() {
	pflag.IntVar(&parallelCalls, "parallel", 11, "Number of parallel calls")
	pflag.StringVar(&targetURL, "url", "http://httpbin.org/ip", "Target URL to request")
	pflag.StringVar(&proxyURL, "proxy", "http://unlimited:pass@localhost", "Proxy URL")
	pflag.DurationVar(&requestTimeout, "timeout", 10*time.Second, "Request timeout in seconds")
	pflag.Parse()

	startTime := time.Now()

	ports := []int{11700, 11070, 11007, 11770}
	//ports := []int{11700}
	clients := []*http.Client{}
	for _, port := range ports {
		pu := fmt.Sprintf("%s:%d", proxyURL, port)
		println(pu)
		proxy, err := url.Parse(pu)
		if err != nil {
			log.Fatalf("Failed to parse proxy URL: %v", err)
		}
		transport := &http.Transport{
			Proxy:           http.ProxyURL(proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: transport}
		clients = append(clients, client)
	}

	var wg sync.WaitGroup
	var errorCount int32
	wg.Add(parallelCalls)

	for i := 0; i < parallelCalls; i++ {
		go func(i int) {
			defer wg.Done()

			client := clients[i%len(clients)]
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				atomic.AddInt32(&errorCount, 1)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Request %d failed: %v", i, err)
				atomic.AddInt32(&errorCount, 1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				atomic.AddInt32(&errorCount, 1)
			}

			fmt.Printf("Completed request %d; Status: %d\n", i, resp.StatusCode)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Printf("All curl calls completed in %v.\n", duration)
	fmt.Printf("Error rate: %.2f%%\n", float64(errorCount)*100/float64(parallelCalls))
}
