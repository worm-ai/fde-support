package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"fde-support/internal/app"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var jsonOutput bool
	root := &cobra.Command{
		Use:   "solution",
		Short: "Solution-as-Code local runtime",
	}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output machine-readable JSON")

	validateCmd := &cobra.Command{
		Use:   "validate manifest.yaml",
		Short: "Validate a Solution Manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.ValidateManifestFile(args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				bytes, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(bytes))
			} else if result.Status == "ok" {
				fmt.Println("manifest valid")
			} else {
				fmt.Println("manifest invalid")
				for _, err := range result.Errors {
					fmt.Printf("- %s %s: %s\n", err.Code, err.Path, err.Message)
				}
			}
			if result.Status != "ok" {
				os.Exit(1)
			}
			return nil
		},
	}

	var envName string
	var addr string
	runCmd := &cobra.Command{
		Use:   "run manifest.yaml",
		Short: "Run a local Solution Runtime",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return app.RunHTTP(ctx, args[0], envName, addr)
		},
	}
	runCmd.Flags().StringVar(&envName, "env", "poc", "delivery environment name")
	runCmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "HTTP listen address")

	root.AddCommand(validateCmd, runCmd)
	return root
}
