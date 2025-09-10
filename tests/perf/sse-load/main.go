package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func main() {
	streamURL := getenv("STREAM_URL", "http://localhost/stream")
	conns := getenvInt("SSE_CONNECTIONS", 200)
	duration := time.Duration(getenvInt("DURATION_SEC", 120)) * time.Second
	bearer := os.Getenv("TEST_BEARER")

	var events uint64
	var attempts uint64
	var failures uint64

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	client := &http.Client{}
	var wg sync.WaitGroup
	wg.Add(conns)
	for range conns {
		go func() {
			defer wg.Done()
			backoff := time.Second
			for {
				if ctx.Err() != nil {
					return
				}
				atomic.AddUint64(&attempts, 1)
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
				if err != nil {
					atomic.AddUint64(&failures, 1)
					time.Sleep(backoff)
					backoff = min(backoff*2, 5*time.Second)
					continue
				}
				if bearer != "" {
					req.Header.Set("Authorization", "Bearer "+bearer)
				}
				resp, err := client.Do(req)
				if err != nil || resp.StatusCode != http.StatusOK {
					if resp != nil {
						resp.Body.Close()
					}
					atomic.AddUint64(&failures, 1)
					time.Sleep(backoff)
					backoff = min(backoff*2, 5*time.Second)
					continue
				}
				backoff = time.Second
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "data:") {
						atomic.AddUint64(&events, 1)
					}
					if ctx.Err() != nil {
						resp.Body.Close()
						return
					}
				}
				resp.Body.Close()
				if ctx.Err() != nil {
					return
				}
				atomic.AddUint64(&failures, 1)
				time.Sleep(backoff)
				backoff = min(backoff*2, 5*time.Second)
			}
		}()
	}

	go func() {
		select {
		case <-time.After(60 * time.Second):
			if atomic.LoadUint64(&events) == 0 {
				fmt.Println("no events received in 60s")
				os.Exit(1)
			}
		case <-ctx.Done():
		}
	}()

	wg.Wait()
	failuresVal := atomic.LoadUint64(&failures)
	attemptsVal := atomic.LoadUint64(&attempts)
	eventsVal := atomic.LoadUint64(&events)
	failureRate := 0.0
	if attemptsVal > 0 {
		failureRate = float64(failuresVal) / float64(attemptsVal)
	}
	fmt.Printf("connections=%d duration_sec=%d events_received=%d connection_failures=%d\n", conns, int(duration.Seconds()), eventsVal, failuresVal)
	if eventsVal == 0 || failureRate > 0.01 {
		os.Exit(1)
	}
}
