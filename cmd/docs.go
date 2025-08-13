package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Generate documentation for the application",
	Long:   `This command group contains utilities for generating documentation.`,
	Hidden: true,
}

var docsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all command documentation to a Markdown file",
	Long:  `Recursively traverses all application commands and exports their full help text into a single, well-formatted Markdown file.`,
	Run: func(cmd *cobra.Command, args []string) {
		outputFile := viper.GetString("docs.export.output")
		f, err := os.Create(outputFile)
		if err != nil {
			printError(fmt.Errorf("failed to create output file: %w", err))
			return
		}
		defer f.Close()

		generationTime := time.Now().Format("2006-01-02 15:04:05 MST")
		fmt.Fprintln(f, "# Code Prompt Core - API Documentation")
		fmt.Fprintf(f, "\n> Generated on: %s\n\n", generationTime)

		err = generateDocForCmd(rootCmd, f)
		if err != nil {
			printError(fmt.Errorf("failed to generate documentation: %w", err))
			return
		}
		fmt.Printf("Documentation successfully generated in %s\n", outputFile)
	},
}

func generateDocForCmd(cmd *cobra.Command, w io.Writer) error {
	if !cmd.IsAvailableCommand() || cmd.IsAdditionalHelpTopicCommand() {
		return nil
	}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.Help()
	level := strings.Count(cmd.CommandPath(), " ")
	header := strings.Repeat("#", level+2)
	if level == 0 {
		fmt.Fprintf(w, "# %s (Root Command)\n\n", cmd.CommandPath())
	} else {
		fmt.Fprintf(w, "\n---\n\n%s %s\n\n", header, cmd.CommandPath())
	}
	fmt.Fprintf(w, "```text\n%s\n```", buf.String())
	for _, subCmd := range cmd.Commands() {
		err := generateDocForCmd(subCmd, w)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(docsCmd)
	docsCmd.AddCommand(docsExportCmd)
	docsExportCmd.Flags().StringP("output", "o", "APIDocumentation.md", "Output file for the generated Markdown documentation")
	viper.BindPFlag("docs.export.output", docsExportCmd.Flags().Lookup("output"))
}
