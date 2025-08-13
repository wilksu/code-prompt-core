package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "code-prompt-core",
	Short: "A high-performance, cross-platform code analysis kernel.",
	Long: `Code Prompt Core is a standalone command-line tool that can be called by various user interfaces to analyze codebases.

It serves as the backend engine, handling file system scanning, data caching, analysis, and report generation.
All configurations can be managed via a central configuration file or overridden by command-line flags.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/code-prompt-core/config.yaml)")
	rootCmd.PersistentFlags().String("db", "code_prompt.db", "Path to the database file")
	viper.BindPFlag("db", rootCmd.PersistentFlags().Lookup("db"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		configPath := filepath.Join(home, ".config", "code-prompt-core")
		viper.AddConfigPath(configPath)
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		ensureDefaultConfig(configPath)
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintln(os.Stderr, "Error reading config file:", err)
		}
	}
}

func ensureDefaultConfig(configPath string) {
	configFilePath := filepath.Join(configPath, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.MkdirAll(configPath, 0755); err != nil {
			fmt.Fprintln(os.Stderr, "Error creating config directory:", err)
			return
		}
	}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		defaultConfig := []byte(`# Default configuration for code-prompt-core
# You can override any command-line flag here.
# The key is constructed as command.subcommand.flagname

db: code_prompt.db

cache:
  update:
    incremental: false
    batch-size: 100
`)
		os.WriteFile(configFilePath, defaultConfig, 0644)
	}
}
