// Command gen-llmdocs regenerates the generated command catalog of the embedded
// LLM reference. It is run by go generate, and re-run in CI to prove the
// committed artifact still matches the command tree.
package main

import (
	"fmt"
	"os"

	"github.com/ideamans/go-llm-cli-kit/catalog"

	"github.com/ideamans/google-tag-manager-cli/cmd"
)

const outPath = "90-commands.md"

func main() {
	md := catalog.Markdown(cmd.Root(), catalog.Options{
		Title: "Command catalog",
		Intro: "Generated from the cobra command tree by `go generate ./...`.\n" +
			"Do not edit by hand — edit the command definitions instead.",
		Skip: []string{"llm"},
	})
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "gen-llmdocs:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes)\n", outPath, len(md))
}
