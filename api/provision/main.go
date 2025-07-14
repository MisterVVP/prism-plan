package main

import (
    "context"
    "errors"
    "log"
    "os"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore"
    "github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
)

func main() {
    conn := os.Getenv("STORAGE_CONNECTION_STRING")
    table := os.Getenv("TABLE_NAME")
    if conn == "" || table == "" {
        log.Fatal("STORAGE_CONNECTION_STRING and TABLE_NAME must be set")
    }
    svc, err := aztables.NewServiceClientFromConnectionString(conn, nil)
    if err != nil {
        log.Fatalf("service client: %v", err)
    }
    _, err = svc.CreateTable(context.Background(), table, nil)
    if err != nil {
        var respErr *azcore.ResponseError
        if errors.As(err, &respErr) && respErr.ErrorCode == string(aztables.TableAlreadyExists) {
            log.Printf("table %s already exists", table)
            return
        }
        log.Fatal(err)
    }
    log.Printf("table %s created", table)
}
