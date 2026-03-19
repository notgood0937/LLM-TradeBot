package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"llmtradebot/internal/app"
	"llmtradebot/internal/config"
)

func main() {
	var (
		contextPath = flag.String("context", "examples/decision_context.json", "path to decision context json")
		printPrompt = flag.Bool("print-prompt", false, "print rendered market context before LLM call")
	)
	flag.Parse()

	cfg, err := config.Load(".env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	payloadBytes, err := os.ReadFile(*contextPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read context failed: %v\n", err)
		os.Exit(1)
	}

	var req app.DecisionRequest
	if err := json.Unmarshal(payloadBytes, &req); err != nil {
		fmt.Fprintf(os.Stderr, "parse context failed: %v\n", err)
		os.Exit(1)
	}

	service := app.NewDecisionService(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, renderedContext, err := service.Run(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decision failed: %v\n", err)
		os.Exit(1)
	}

	if *printPrompt {
		fmt.Println("===== Market Context =====")
		fmt.Println(renderedContext)
		fmt.Println("===== End Context =====")
	}

	pretty, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(pretty))
}
