package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "code-prompt-core",
	Short: "A high-performance, cross-platform code analysis kernel.",
	Long:  `A standalone command-line tool that can be called by various user interfaces to analyze codebases.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
