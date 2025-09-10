package main

import (
	"fmt"
	"log"

	testutil "prismtestutil"
)

func main() {
	tok, err := testutil.TestToken("perf-user")
	if err != nil {
		log.Fatalf("generate token: %v", err)
	}
	fmt.Print(tok)
}
