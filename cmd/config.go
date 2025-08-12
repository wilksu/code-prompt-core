package cmd

import (
	"database/sql"
	"fmt"

	"code-prompt-core/pkg/database"

	"github.com/spf13/cobra"
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
		dbPath, _ := cmd.Flags().GetString("db")
		key, _ := cmd.Flags().GetString("key")
		value, _ := cmd.Flags().GetString("value")

		db, err := database.InitializeDB(dbPath)
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
		dbPath, _ := cmd.Flags().GetString("db")
		key, _ := cmd.Flags().GetString("key")

		db, err := database.InitializeDB(dbPath)
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
	configSetCmd.Flags().String("db", "", "Path to the database file")
	configSetCmd.MarkFlagRequired("db")
	configSetCmd.Flags().String("key", "", "The configuration key")
	configSetCmd.MarkFlagRequired("key")
	configSetCmd.Flags().String("value", "", "The configuration value to set")
	configSetCmd.MarkFlagRequired("value")

	configCmd.AddCommand(configGetCmd)
	configGetCmd.Flags().String("db", "", "Path to the database file")
	configGetCmd.MarkFlagRequired("db")
	configGetCmd.Flags().String("key", "", "The configuration key to get")
	configGetCmd.MarkFlagRequired("key")
}
