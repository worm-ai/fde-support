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

	ingestCmd := &cobra.Command{
		Use:   "ingest manifest.yaml",
		Short: "Ingest knowledge sources and run quality gates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := app.IngestManifestFile(args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				bytes, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(bytes))
			} else {
				fmt.Printf("ingest %s: %d records, %d ingested\n", result.Status, result.TotalRecords, result.TotalIngested)
			}
			if result.Status == "blocked" {
				os.Exit(1)
			}
			return nil
		},
	}

	var evalEnvName string
	evaluateCmd := &cobra.Command{
		Use:   "evaluate manifest.yaml",
		Short: "Evaluate a Solution against golden cases",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := app.EvaluateManifestFile(args[0], evalEnvName)
			if err != nil {
				return err
			}
			if jsonOutput {
				bytes, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(bytes))
			} else {
				fmt.Printf("evaluate %s: %d/%d cases passed\n", report.Solution, report.PassedCases, report.TotalCases)
				for name, value := range report.Metrics {
					fmt.Printf("  %s: %.4f\n", name, value)
				}
				for _, gate := range report.GateResults {
					status := "PASS"
					if !gate.Passed {
						status = "FAIL"
					}
					fmt.Printf("  gate %s: %s (%.4f >= %.4f, %s)\n", gate.Metric, status, gate.Actual, gate.Min, gate.Severity)
				}
			}
			for _, gate := range report.GateResults {
				if !gate.Passed && gate.Severity == "block" && gate.Schedule == "onRelease" {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	evaluateCmd.Flags().StringVar(&evalEnvName, "env", "poc", "delivery environment name")

	var envName string
	var addr string
	var templateName string
	runCmd := &cobra.Command{
		Use:   "run [manifest.yaml]",
		Short: "Run a local Solution Runtime",
		Args: func(cmd *cobra.Command, args []string) error {
			if templateName != "" && len(args) == 0 {
				return nil
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			manifestPath := ""
			if len(args) > 0 {
				manifestPath = args[0]
			}
			if templateName != "" {
				resolved, err := app.ResolveTemplatePath(templateName)
				if err != nil {
					return err
				}
				manifestPath = resolved
			}
			return app.RunHTTP(ctx, manifestPath, envName, addr)
		},
	}
	runCmd.Flags().StringVar(&envName, "env", "poc", "delivery environment name")
	runCmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "HTTP listen address")
	runCmd.Flags().StringVar(&templateName, "template", "", "template name from templates/")

	var releaseEnvName string
	releaseCmd := &cobra.Command{
		Use:   "release manifest.yaml",
		Short: "Run release checks and generate deployment artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := app.ReleaseManifestFile(args[0], releaseEnvName)
			if err != nil {
				return err
			}
			if jsonOutput {
				bytes, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(bytes))
			} else {
				fmt.Printf("release %s: passed=%v\n", report.Env, report.Passed)
				for _, check := range report.Checks {
					status := "PASS"
					if !check.Passed {
						status = "FAIL"
					}
					fmt.Printf("  %s: %s %s\n", check.Name, status, check.Message)
				}
			}
			if !report.Passed {
				os.Exit(1)
			}
			return nil
		},
	}
	releaseCmd.Flags().StringVar(&releaseEnvName, "env", "production", "delivery environment name")

	publishCmd := &cobra.Command{
		Use:   "component-publish component-dir",
		Short: "Package a custom component for sharing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := app.PublishComponentFile(args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				bytes, _ := json.MarshalIndent(map[string]any{"artifact": path}, "", "  ")
				fmt.Println(string(bytes))
			} else {
				fmt.Printf("component artifact generated at %s\n", path)
			}
			return nil
		},
	}

	root.AddCommand(validateCmd, ingestCmd, evaluateCmd, releaseCmd, runCmd, publishCmd)
	return root
}
