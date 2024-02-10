package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

const (
	parallelCalls  = 100
	targetURL      = "http://bacv.org"
	proxyURL       = "http://user:pass@localhost:10700"
	requestTimeout = 10
)

func main() {
	startTime := time.Now()

	proxy, err := url.Parse(proxyURL)
	if err != nil {
		log.Fatalf("Failed to parse proxy URL: %v", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
	}

	client := &http.Client{
		Transport: transport,
	}

	var wg sync.WaitGroup
	var errorCount int32
	wg.Add(parallelCalls)

	for i := 0; i < parallelCalls; i++ {
		go func(i int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Request %d failed: %v", i, err)
				atomic.AddInt32(&errorCount, 1)
				return
			}
			defer resp.Body.Close()

			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				log.Printf("Error reading response of request %d: %v", i, err)
				atomic.AddInt32(&errorCount, 1)
			}

			fmt.Printf("Completed request %d\n", i)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	totalRequests := float64(parallelCalls)
	errorRate := float64(errorCount) / totalRequests

	fmt.Printf("All curl calls completed in %v.\n", duration)
	fmt.Printf("Error rate: %.2f%%\n", errorRate*100)
}
