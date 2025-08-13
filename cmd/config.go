package cmd

import (
	"code-prompt-core/pkg/database"
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage generic key-value configurations stored in the database",
	Long:  "This command allows setting and getting arbitrary key-value pairs, useful for storing GUI settings or other metadata.",
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Sets a value for a given key",
	Run: func(cmd *cobra.Command, args []string) {
		key := viper.GetString("config.set.key")
		value := viper.GetString("config.set.value")
		if key == "" {
			printError(fmt.Errorf("--key is required"))
			return
		}
		db, err := database.InitializeDB(viper.GetString("db"))
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()
		upsertSQL := `INSERT INTO kv_store (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value;`
		_, err = db.Exec(upsertSQL, key, value)
		if err != nil {
			printError(fmt.Errorf("error setting config for key '%s': %w", key, err))
			return
		}
		printJSON(fmt.Sprintf("Config for key '%s' was saved.", key))
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Gets the value for a given key",
	Run: func(cmd *cobra.Command, args []string) {
		key := viper.GetString("config.get.key")
		if key == "" {
			printError(fmt.Errorf("--key is required"))
			return
		}
		db, err := database.InitializeDB(viper.GetString("db"))
		if err != nil {
			printError(fmt.Errorf("error initializing database: %w", err))
			return
		}
		defer db.Close()
		var value string
		err = db.QueryRow("SELECT value FROM kv_store WHERE key = ?", key).Scan(&value)
		if err != nil {
			if err == sql.ErrNoRows {
				printError(fmt.Errorf("no config value found for key: %s", key))
			} else {
				printError(fmt.Errorf("error getting config for key '%s': %w", key, err))
			}
			return
		}
		printJSON(value)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configSetCmd)
	configSetCmd.Flags().String("key", "", "The configuration key")
	configSetCmd.Flags().String("value", "", "The configuration value to set")
	viper.BindPFlag("config.set.key", configSetCmd.Flags().Lookup("key"))
	viper.BindPFlag("config.set.value", configSetCmd.Flags().Lookup("value"))

	configCmd.AddCommand(configGetCmd)
	configGetCmd.Flags().String("key", "", "The configuration key to get")
	viper.BindPFlag("config.get.key", configGetCmd.Flags().Lookup("key"))
}
