package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Generate documentation for the application",
	Long:  `This command group contains utilities for generating documentation.`,
	// We hide this command from the main help output to keep it clean for end-users.
	// It's a developer/CI tool.
	Hidden: true,
}

var docsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all command documentation to a Markdown file",
	Long: `Recursively traverses all application commands and exports their full help text into a single, well-formatted Markdown file.

This utility is useful for generating documentation for wikis or repository viewing.

Example Usage:
  # Export documentation to the default file (APIDocumentation.md)
  ./code-prompt-core.exe docs export

  # Export documentation to a custom file
  ./code-prompt-core.exe docs export --output my-docs.md
`,
	Run: func(cmd *cobra.Command, args []string) {
		outputFile, _ := cmd.Flags().GetString("output")

		f, err := os.Create(outputFile)
		if err != nil {
			printError(fmt.Errorf("failed to create output file: %w", err))
			return
		}
		defer f.Close()

		fmt.Fprintln(f, "# Code Prompt Core - API Documentation")
		fmt.Fprintf(f, "\n> Generated on: %s\n\n", cmd.Version) // Using version for timestamp
		fmt.Fprintln(f, "")

		// Recursively generate docs for all commands starting from the root
		err = generateDocForCmd(rootCmd, f)
		if err != nil {
			printError(fmt.Errorf("failed to generate documentation: %w", err))
			return
		}

		fmt.Printf("Documentation successfully generated in %s\n", outputFile)
	},
}

// generateDocForCmd recursively generates markdown for a command and its children.
func generateDocForCmd(cmd *cobra.Command, w io.Writer) error {
	// Skip the 'docs' command itself and the default 'help' command
	if !cmd.IsAvailableCommand() || cmd.IsAdditionalHelpTopicCommand() {
		return nil
	}

	// Use a buffer to capture the help output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.Help()

	// Create a title based on the command path
	// Level 1 is #, Level 2 is ##, etc.
	level := strings.Count(cmd.CommandPath(), " ")
	header := strings.Repeat("#", level+2) // Start with ## for subcommands

	if level == 0 { // Root command
		fmt.Fprintf(w, "# %s (Root Command)\n\n", cmd.CommandPath())
	} else {
		fmt.Fprintf(w, "%s %s\n\n", header, cmd.CommandPath())
	}

	fmt.Fprintf(w, "```text\n%s\n```\n\n---\n\n", buf.String())

	// Recurse for subcommands
	for _, subCmd := range cmd.Commands() {
		err := generateDocForCmd(subCmd, w)
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	// Add a version field to the root command, which we can use in the docs
	rootCmd.Version = "1.0.0 - " + "2025-08-11" // You can make this dynamic during build time later

	rootCmd.AddCommand(docsCmd)
	docsCmd.AddCommand(docsExportCmd)
	docsExportCmd.Flags().StringP("output", "o", "APIDocumentation.md", "Output file for the generated Markdown documentation")
}
