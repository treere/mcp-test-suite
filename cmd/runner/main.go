package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/treere/mcp-test-suite/config"
	"github.com/treere/mcp-test-suite/runner"
)

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	r := runner.New(cfg)
	results := r.Run()

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Printf("Results: %d/%d passed\n", results.Passed, results.Total)
	fmt.Println("==========================================")

	if results.Failed > 0 {
		fmt.Println("\nFailed tests:")
		for _, t := range results.Tests {
			if !t.Passed {
				fmt.Printf("  - %s: %v\n", t.Name, t.Error)
			}
		}
		os.Exit(1)
	}

	fmt.Println("All tests PASSED!")
}

func init() {
	// Set default timeout for the test run
	time.Sleep(100 * time.Millisecond)
}
