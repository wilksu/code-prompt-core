# Code Prompt Core - API Documentation

> Generated on: 2025-08-16 10:11:56 CST

# code-prompt-core (Root Command)

```text
Code Prompt Core is a standalone command-line tool that can be called by various user interfaces to analyze codebases.

It serves as the backend engine, handling file system scanning, data caching, analysis, and report generation.
All configurations can be managed via a central configuration file or overridden by command-line flags.

Usage:
  code-prompt-core [command]

Available Commands:
  analyze     Analyze the cached data of a project
  cache       Manage the project file cache
  completion  Generate the autocompletion script for the specified shell
  config      Manage generic key-value configurations stored in the database
  content     Retrieve file contents
  help        Help about any command
  profiles    Manage filter profiles for projects
  project     Manage projects within the database
  report      Generate reports from project data

Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core [command] --help" for more information about a command.

```
---

### code-prompt-core analyze

```text
The "analyze" command group provides tools to query and generate insights from the cached project data without re-scanning the file system. All analysis commands operate on the existing data in the database, making them very fast.

Usage:
  code-prompt-core analyze [command]

Available Commands:
  filter      Filter cached file metadata using JSON or a saved profile
  stats       Generate statistics about the project's cached files
  tree        Generate a file structure tree from the cache

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core analyze [command] --help" for more information about a command.

```
---

#### code-prompt-core analyze filter

```text
Filters the cached file metadata based on various criteria provided as a JSON string.

The filter JSON supports both simple and advanced rules:
{
  "includeExts": ["go", "md"],
  "excludePaths": ["vendor/"],
  "includeRegex": ["^cmd/"],
  "priority": "includes"
}

Example:
  code-prompt-core analyze filter --project-path /p/proj --filter-json '{"includeExts":[".go"]}'

Usage:
  code-prompt-core analyze filter [flags]

Flags:
      --filter-json string    JSON string with filter conditions
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core analyze stats

```text
Generates statistical information about the project's current cache.
It groups files by their extension and provides counts, total size, and total lines for each type, as well as overall totals. This command gives a high-level overview of the project's composition.

Example:
  code-prompt-core analyze stats --project-path /path/to/project

Usage:
  code-prompt-core analyze stats [flags]

Flags:
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core analyze tree

```text
Generates a file structure tree, optionally annotating it based on a filter.

The filter (from --filter-json or --profile-name) determines which files are marked as "included".
The filter JSON supports both simple and advanced rules:
{
  "excludeExts": ["md"],
  "includePaths": ["cmd/"]
}

Example (JSON output, annotated):
  code-prompt-core analyze tree --project-path /p/proj --filter-json '{"excludeExts":["md"]}'

Usage:
  code-prompt-core analyze tree [flags]

Flags:
      --filter-json string    A temporary JSON string with filter conditions
      --format string         Output format for the tree (json or text) (default "json")
      --profile-name string   Name of a saved filter profile to use for annotating the tree
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core cache

```text
Contains subcommands for creating, updating, and checking the status of a project's file metadata cache.

Usage:
  code-prompt-core cache [command]

Available Commands:
  update      Create or update the cache for a project (full or incremental)

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core cache [command] --help" for more information about a command.

```
---

#### code-prompt-core cache update

```text
Performs a scan of the specified project and updates the cache in the database.

This is the core data-gathering command. It can perform two types of scans:
1. Full Scan (default): Clears any existing data for the project and scans everything from scratch.
2. Incremental Scan (--incremental): Much faster for subsequent scans. It compares the file system with the last cached state and only processes new, modified, or deleted files.

This command intelligently ignores files specified in '.gitignore' and common dependency directories (like 'node_modules', 'vendor', etc.) by default. This behavior can be modified with flags.

All parameters for this command can be configured in your config file under the 'cache.update' key.
For example:
  cache:
    update:
      project-path: /path/to/my/project
      incremental: true
      batch-size: 200

Usage:
  code-prompt-core cache update [flags]

Flags:
      --batch-size int        Number of DB operations to batch in incremental scans (default 100)
      --include-binary        Include binary files in the scan
      --incremental           Perform an incremental scan
      --no-git-ignores        Disable .gitignore file parsing
      --no-preset-excludes    Disable default exclusion of dependency directories
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core completion

```text
Generate the autocompletion script for code-prompt-core for the specified shell.
See each sub-command's help for details on how to use the generated script.

Usage:
  code-prompt-core completion [command]

Available Commands:
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core completion [command] --help" for more information about a command.

```
---

#### code-prompt-core completion bash

```text
Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(code-prompt-core completion bash)

To load completions for every new session, execute once:

#### Linux:

	code-prompt-core completion bash > /etc/bash_completion.d/code-prompt-core

#### macOS:

	code-prompt-core completion bash > $(brew --prefix)/etc/bash_completion.d/code-prompt-core

You will need to start a new shell for this setup to take effect.

Usage:
  code-prompt-core completion bash

Flags:
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core completion fish

```text
Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	code-prompt-core completion fish | source

To load completions for every new session, execute once:

	code-prompt-core completion fish > ~/.config/fish/completions/code-prompt-core.fish

You will need to start a new shell for this setup to take effect.

Usage:
  code-prompt-core completion fish [flags]

Flags:
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core completion powershell

```text
Generate the autocompletion script for powershell.

To load completions in your current shell session:

	code-prompt-core completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.

Usage:
  code-prompt-core completion powershell [flags]

Flags:
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core completion zsh

```text
Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(code-prompt-core completion zsh)

To load completions for every new session, execute once:

#### Linux:

	code-prompt-core completion zsh > "${fpath[1]}/_code-prompt-core"

#### macOS:

	code-prompt-core completion zsh > $(brew --prefix)/share/zsh/site-functions/_code-prompt-core

You will need to start a new shell for this setup to take effect.

Usage:
  code-prompt-core completion zsh [flags]

Flags:
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core config

```text
This command allows setting and getting arbitrary key-value pairs, useful for storing GUI settings or other metadata.

Usage:
  code-prompt-core config [command]

Available Commands:
  get         Gets the value for a given key
  set         Sets a value for a given key

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core config [command] --help" for more information about a command.

```
---

#### code-prompt-core config get

```text
Gets the value for a given key

Usage:
  code-prompt-core config get [flags]

Flags:
      --key string   The configuration key to get

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core config set

```text
Sets a value for a given key

Usage:
  code-prompt-core config set [flags]

Flags:
      --key string     The configuration key
      --value string   The configuration value to set

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core content

```text
Retrieve file contents

Usage:
  code-prompt-core content [command]

Available Commands:
  get         Batch gets the content of specified files for a project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core content [command] --help" for more information about a command.

```
---

#### code-prompt-core content get

```text
Retrieves the contents of multiple files at once using regex filters.

Usage:
  code-prompt-core content get [flags]

Flags:
      --excludes string       Comma-separated regex for files to exclude
      --includes string       Comma-separated regex for files to include
      --priority string       Priority if a file matches both lists ('includes' or 'excludes') (default "includes")
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core profiles

```text
A filter profile is a saved set of filter rules that can be reused across different commands. This command group allows you to save, list, load, and delete these profiles.

Usage:
  code-prompt-core profiles [command]

Available Commands:
  delete      Delete a filter profile
  list        List all saved profiles for a project
  load        Load and display a specific filter profile
  save        Save or update a filter profile

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core profiles [command] --help" for more information about a command.

```
---

#### code-prompt-core profiles delete

```text
Deletes a named filter profile from a project. This action is irreversible.

Usage:
  code-prompt-core profiles delete [flags]

Flags:
      --name string           Name of the profile to delete
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core profiles list

```text
Retrieves and displays all filter profiles that have been saved for a specific project.

Usage:
  code-prompt-core profiles list [flags]

Flags:
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core profiles load

```text
Loads a single, named filter profile for a project and displays its JSON content.

Usage:
  code-prompt-core profiles load [flags]

Flags:
      --name string           Name of the profile to load
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core profiles save

```text
Saves a filter configuration as a named profile for a specific project. If a profile with the same name already exists, it will be updated.

The filter configuration must be provided as a JSON string via the --data flag.
The JSON structure supports both simple and advanced (regex) rules:
{
  "includePaths": ["cmd/", "pkg/database/database.go"],
  "excludePaths": ["vendor/"],
  "includeExts": ["go", "md"],
  "excludeExts": ["sum"],
  "includePrefixes": ["main"],
  "excludePrefixes": ["test_"],
  
  "includeRegex": ["\\.hbs$"],
  "excludeRegex": ["^\\.git/"],
  
  "priority": "includes"
}

- Simple rules (paths, exts, prefixes) are convenient for common cases.
- Regex rules (includeRegex, excludeRegex) provide maximum flexibility for advanced users.
- "priority": Optional. Can be "includes" or "excludes". Determines which rule wins if a file matches both lists. Defaults to "includes".

Example:
  code-prompt-core profiles save --project-path /p/my-proj --name "go-source" --data '{"includeExts":["go"], "excludePaths": ["vendor/"]}'

Usage:
  code-prompt-core profiles save [flags]

Flags:
      --data string           JSON data for the profile's filter rules
      --name string           Name of the profile to save
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core project

```text
The "project" command group allows you to add, list, and delete project records in the database. A project record must exist before you can manage its cache or profiles.

Usage:
  code-prompt-core project [command]

Available Commands:
  add         Adds a new project to the database without scanning
  delete      Delete a project and all its associated data
  list        List all projects stored in the database

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core project [command] --help" for more information about a command.

```
---

#### code-prompt-core project add

```text
This lightweight command creates a project record in the database, allowing profile management or other configurations before performing the first (potentially long) scan.
If the project already exists, this command will do nothing and will not return an error.

Example:
  code-prompt-core project add --project-path /path/to/my-new-project

Usage:
  code-prompt-core project add [flags]

Flags:
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core project delete

```text
Deletes a project record from the database.
Due to the database schema's 'ON DELETE CASCADE' setting, this will also automatically delete all associated file metadata and saved filter profiles for that project. This action is irreversible.

Example:
  code-prompt-core project delete --project-path /path/to/project-to-delete

Usage:
  code-prompt-core project delete [flags]

Flags:
      --project-path string   Path to the project

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core project list

```text
Retrieves and displays a list of all projects currently managed in the specified database file, along with the timestamp of their last scan.

Usage:
  code-prompt-core project list [flags]

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

### code-prompt-core report

```text
The "report" command group provides tools to generate rich, user-defined reports by combining various data points (stats, tree, file content) with Handlebars templates.

Usage:
  code-prompt-core report [command]

Available Commands:
  generate       Generate a report from a template
  list-templates List all available built-in report templates

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

Use "code-prompt-core report [command] --help" for more information about a command.

```
---

#### code-prompt-core report generate

```text
This command aggregates project statistics, file structure, and file contents, then uses a Handlebars template to generate a final report file.

You can filter the files included in the report using either a saved profile via '--profile-name' or a temporary filter via '--filter-json'. If both are provided, '--filter-json' takes precedence.

The filter JSON structure supports both simple and advanced rules:
{
  "includeExts": ["go"],
  "excludePaths": ["vendor/"],
  "priority": "includes"
}

If the '--output' flag is provided with a file path, the report is saved to that file. Otherwise, the report content is printed directly to the standard output.

Example (using a built-in template and a filter):
  code-prompt-core report generate --template summary.txt --filter-json '{"includeExts":["go"]}' --output report.txt

Usage:
  code-prompt-core report generate [flags]

Flags:
      --filter-json string    A temporary JSON string with filter conditions to use (overrides profile-name)
      --output string         Path to the output report file. If empty, prints to stdout.
      --profile-name string   Name of a saved filter profile to use for filtering content
      --project-path string   Path to the project
      --template string       Name of a built-in template or path to a custom .hbs file (default "summary.txt")

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```
---

#### code-prompt-core report list-templates

```text
Displays a list of all built-in templates that were compiled into the application, in JSON format.
These template names can be used directly with the '--template' flag of the 'report generate' command.

The returned JSON format is as follows:
{
  "status": "success",
  "data": [
    {
      "name": "summary.txt",
      "description": "A built-in report template."
    }
  ]
}

Usage:
  code-prompt-core report list-templates [flags]

Global Flags:
      --config string   config file (default is $HOME/.config/code-prompt-core/config.yaml)
      --db string       Path to the database file (default "code_prompt.db")

```