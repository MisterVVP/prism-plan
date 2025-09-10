package main

import (
	"fmt"
	"log"

	integration "prismtest"
)

func main() {
	tok, err := integration.TestToken("perf-user")
	if err != nil {
		log.Fatalf("generate token: %v", err)
	}
	fmt.Print(tok)
}
