package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	testutil "prismtestutil"
)

func main() {
	var (
		count  = flag.Int("count", 1, "number of tokens to generate")
		prefix = flag.String("prefix", "perf-user", "prefix for generated user IDs when count > 1")
		start  = flag.Int("start", 1, "starting index for generated user IDs when count > 1")
		output = flag.String("output", "", "file to write generated tokens as a JSON array")
	)

	flag.Parse()

	if *count < 1 {
		log.Fatal("count must be at least 1")
	}

	if *start < 1 {
		log.Fatal("start index must be at least 1")
	}

	args := flag.Args()
	if len(args) > 0 && *count > 1 {
		log.Fatal("explicit user ID cannot be provided when generating multiple tokens")
	}

	tokens, err := generateTokens(*count, *prefix, *start, args)
	if err != nil {
		log.Fatalf("generate token: %v", err)
	}

	if *output != "" {
		if err := writeTokens(*output, tokens); err != nil {
			log.Fatalf("write tokens: %v", err)
		}
	}

	fmt.Print(tokens[0])
}

func generateTokens(count int, prefix string, start int, args []string) ([]string, error) {
	tokens := make([]string, count)

	for i := 0; i < count; i++ {
		var userID string
		if len(args) > 0 {
			userID = args[0]
		} else if count == 1 {
			userID = prefix
		} else {
			userID = fmt.Sprintf("%s-%d", prefix, start+i)
		}

		tok, err := testutil.TestToken(userID)
		if err != nil {
			return nil, err
		}

		tokens[i] = tok
	}

	return tokens, nil
}

func writeTokens(path string, tokens []string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	data, err := json.Marshal(tokens)
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return nil
}
