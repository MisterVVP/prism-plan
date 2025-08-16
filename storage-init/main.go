package main

import (
	"context"
	"errors"
	"os"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue"
	log "github.com/sirupsen/logrus"
)

func main() {
	if dbg, err := strconv.ParseBool(os.Getenv("DEBUG")); err == nil && dbg {
		log.SetLevel(log.DebugLevel)
	}
	log.Info("storage init starting")

	connStr := os.Getenv("STORAGE_CONNECTION_STRING")
	if connStr == "" {
		log.Fatal("missing STORAGE_CONNECTION_STRING")
	}

	ctx := context.Background()

	if err := createTables(ctx, connStr, []string{
		os.Getenv("TASK_EVENTS_TABLE"),
		os.Getenv("USER_EVENTS_TABLE"),
		os.Getenv("TASKS_TABLE"),
		os.Getenv("USERS_TABLE"),
	}); err != nil {
		log.Fatalf("create tables: %v", err)
	}

	if err := createQueues(ctx, connStr, []string{
		os.Getenv("COMMAND_QUEUE"),
		os.Getenv("DOMAIN_EVENTS_QUEUE"),
	}); err != nil {
		log.Fatalf("create queues: %v", err)
	}

	log.Info("storage init complete")
}

func createTables(ctx context.Context, connStr string, names []string) error {
	svc, err := aztables.NewServiceClientFromConnectionString(connStr, nil)
	if err != nil {
		return err
	}
	for _, name := range names {
		if name == "" {
			continue
		}
		c := svc.NewClient(name)
		_, err := c.CreateTable(ctx, nil)
		if err != nil {
			var respErr *azcore.ResponseError
			if !(errors.As(err, &respErr) && respErr.ErrorCode == string(aztables.TableAlreadyExists)) {
				return err
			}
		}
	}
	return nil
}

func createQueues(ctx context.Context, connStr string, names []string) error {
	for _, name := range names {
		if name == "" {
			continue
		}
		q, err := azqueue.NewQueueClientFromConnectionString(connStr, name, nil)
		if err != nil {
			return err
		}
		_, err = q.Create(ctx, nil)
		if err != nil {
			var respErr *azcore.ResponseError
			if !(errors.As(err, &respErr) && respErr.ErrorCode == "QueueAlreadyExists") {
				return err
			}
		}
	}
	return nil
}
