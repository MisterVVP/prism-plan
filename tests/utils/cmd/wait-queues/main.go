package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
)

type queueList []string

func (q *queueList) String() string {
	if q == nil {
		return ""
	}
	return strings.Join(*q, ",")
}

func (q *queueList) Set(value string) error {
	if value == "" {
		return errors.New("queue name cannot be empty")
	}
	*q = append(*q, value)
	return nil
}

func newQueueClient(connStr, name string) (*azqueue.QueueClient, error) {
	clientOpts := azqueue.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    3,
				TryTimeout:    30 * time.Second,
				RetryDelay:    time.Second,
				MaxRetryDelay: 5 * time.Second,
				StatusCodes:   []int{408, 429, 500, 502, 503, 504},
			},
		},
	}
	return azqueue.NewQueueClientFromConnectionString(connStr, name, &clientOpts)
}

func pollQueues(ctx context.Context, interval time.Duration, stableRequired int, clients map[string]*azqueue.QueueClient) error {
	if stableRequired < 1 {
		stableRequired = 1
	}
	stableCounts := make(map[string]int, len(clients))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("waiting for %d queue(s) to drain", len(clients))

	checkOnce := func() (bool, error) {
		allStable := true
		for name, client := range clients {
			resp, err := client.GetProperties(ctx, nil)
			if err != nil {
				return false, fmt.Errorf("failed to get properties for %s: %w", name, err)
			}
			count := int32(0)
			if resp.ApproximateMessagesCount != nil {
				count = *resp.ApproximateMessagesCount
			}
			if count > 0 {
				log.Printf("queue %s has %d pending message(s)", name, count)
				stableCounts[name] = 0
				allStable = false
				continue
			}
			stableCounts[name]++
			if stableCounts[name] < stableRequired {
				allStable = false
			}
		}
		return allStable, nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		done, err := checkOnce()
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func main() {
	log.SetOutput(os.Stderr)
	var (
		connStr        string
		timeout        time.Duration
		interval       time.Duration
		stableRequired int
		queues         queueList
	)
	flag.StringVar(&connStr, "connection-string", "", "Azure Storage connection string")
	flag.DurationVar(&timeout, "timeout", 2*time.Minute, "maximum time to wait for queues to drain")
	flag.DurationVar(&interval, "interval", 2*time.Second, "polling interval")
	flag.IntVar(&stableRequired, "stable", 3, "number of consecutive empty polls required per queue")
	flag.Var(&queues, "queue", "queue name to monitor (repeatable)")
	flag.Parse()

	if connStr == "" {
		log.Fatal("connection-string is required")
	}
	if len(queues) == 0 {
		log.Fatal("at least one queue must be specified")
	}

	clients := make(map[string]*azqueue.QueueClient, len(queues))
	for _, name := range queues {
		client, err := newQueueClient(connStr, name)
		if err != nil {
			log.Fatalf("failed to create client for %s: %v", name, err)
		}
		clients[name] = client
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := pollQueues(ctx, interval, stableRequired, clients); err != nil {
		log.Fatalf("queue wait failed: %v", err)
	}

	log.Printf("all queues drained")
}
