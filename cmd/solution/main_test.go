package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpListsReleaseAndComponentPublish(t *testing.T) {
	output, err := commandOutput("--help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	for _, want := range []string{"release", "component-publish"} {
		if !strings.Contains(output, want) {
			t.Fatalf("root help missing %q:\n%s", want, output)
		}
	}
}

func TestRunHelpListsTemplateFlag(t *testing.T) {
	output, err := commandOutput("run", "--help")
	if err != nil {
		t.Fatalf("run help error = %v", err)
	}
	if !strings.Contains(output, "--template") {
		t.Fatalf("run help missing --template:\n%s", output)
	}
}

func TestRunTemplateAllowsMissingManifestArg(t *testing.T) {
	root := newRootCommand()
	runCmd, _, err := root.Find([]string{"run"})
	if err != nil {
		t.Fatalf("find run command: %v", err)
	}
	if err := runCmd.Args(runCmd, []string{}); err == nil {
		t.Fatalf("run without manifest or template error = nil, want error")
	}
	if err := runCmd.Flags().Set("template", "customer-support"); err != nil {
		t.Fatalf("set template flag: %v", err)
	}
	if err := runCmd.Args(runCmd, []string{}); err != nil {
		t.Fatalf("run --template args error = %v", err)
	}
}

func commandOutput(args ...string) (string, error) {
	cmd := newRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}
