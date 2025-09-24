package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	var (
		outPath     string
		eventName   string
		eventDomain string
	)

	flag.StringVar(&outPath, "out", "", "path to write aggregated metrics JSON")
	flag.StringVar(&eventName, "event-name", tasksEventName, "observability event name to collect")
	flag.StringVar(&eventDomain, "event-domain", tasksEventDomain, "observability event domain to match")
	flag.Parse()

	if outPath == "" {
		fmt.Fprintln(os.Stderr, "-out is required")
		os.Exit(2)
	}

	collector := newCollector(eventName, eventDomain)
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if len(line) != 0 {
			collector.ingest(line)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "read logs: %v\n", err)
			os.Exit(1)
		}
	}

	summary := collector.summary()

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		os.Exit(1)
	}

	file, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		fmt.Fprintf(os.Stderr, "write summary: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(summary.ShortString())
}
