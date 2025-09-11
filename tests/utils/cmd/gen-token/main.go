package main

import (
        "fmt"
        "log"
        "os"

        testutil "prismtestutil"
)

func main() {
        userID := "perf-user"
        if len(os.Args) > 1 {
                userID = os.Args[1]
        }
        tok, err := testutil.TestToken(userID)
        if err != nil {
                log.Fatalf("generate token: %v", err)
        }
        fmt.Print(tok)
}
