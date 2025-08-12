# Code Prompt Core - API Documentation

> Generated on: 


# code-prompt-core (Root Command)

```text
A standalone command-line tool that can be called by various user interfaces to analyze codebases.

Usage:
  code-prompt-core [command]

Available Commands:
  analyze     Analyze the cached data of a project
  cache       Manage the project file cache
  completion  Generate the autocompletion script for the specified shell
  config      Manage generic key-value configurations stored in the database
  content     Retrieve file contents
  help        Help about any command
  profiles    Manage filter profiles
  project     Manage projects within the database

Use "code-prompt-core [command] --help" for more information about a command.

```

---

### code-prompt-core analyze

```text
The "analyze" command group provides tools to filter, query, and generate insights from the cached project data without re-scanning the file system.

Usage:
  code-prompt-core analyze [command]

Available Commands:
  filter      Filter, sort, and paginate the cached file metadata
  stats       Generate statistics about the project's cached files
  tree        Generate a file structure tree from the cache

Use "code-prompt-core analyze [command] --help" for more information about a command.

```

---

#### code-prompt-core analyze filter

```text
Filters the cached file metadata based on various criteria provided via flags.

The main filtering mechanism is --filter-json, which accepts a JSON string with the following keys:
- "excludedExtensions": An array of strings. To exclude files with no extension, use the special string "no_extension". Example: ["go", "md", "no_extension"]
- "excludedPrefixes": An array of strings representing path prefixes to exclude. Example: ["cmd/"]
- "isTextOnly": A boolean that, if true, only includes text files.

Sorting and pagination are supported via dedicated flags.

Example Usage:
  # Get the 50 largest files, sorted by size descending
  ./code-prompt-core.exe analyze filter --db my.db --project-path /p/project --sort-by size_bytes --sort-order desc --limit 50

  # Get all text files, excluding .md files and files in the 'vendor/' directory
  ./code-prompt-core.exe analyze filter --db my.db --project-path /p/project --filter-json '{"isTextOnly":true, "excludedExtensions":[".md"], "excludedPrefixes":["vendor/"]}'

Usage:
  code-prompt-core analyze filter [flags]

Flags:
      --db string             Path to the database file
      --filter-json string    JSON string with filter conditions
      --limit int             Limit the number of results (-1 for no limit) (default -1)
      --offset int            Offset for pagination
      --project-path string   Path to the project
      --sort-by string        Column to sort by (relative_path, filename, size_bytes, line_count) (default "relative_path")
      --sort-order string     Sort order (asc or desc) (default "asc")

```

---

#### code-prompt-core analyze stats

```text
Generates statistical information about the project's current cache.
It groups files by their extension and provides counts, total size, and total lines for each type, as well as overall totals.

Example Usage:
  ./code-prompt-core.exe analyze stats --db my.db --project-path /path/to/project

Usage:
  code-prompt-core analyze stats [flags]

Flags:
      --db string             Path to the database file
      --project-path string   Path to the project

```

---

#### code-prompt-core analyze tree

```text
Generates a file structure tree based on the cached data, optionally annotating nodes based on filter criteria.
The default output is a structured JSON object, ideal for UIs. A plain text format is also available.

Example (JSON output, annotated):
  ./code-prompt-core.exe analyze tree --db my.db --project-path /p/proj --filter-json '{"excludedExtensions":[".md"]}'

Example (Text output, annotated):
  ./code-prompt-core.exe analyze tree --db my.db --project-path /p/proj --filter-json '{"excludedExtensions":[".md"]}' --format=text

Usage:
  code-prompt-core analyze tree [flags]

Flags:
      --db string             Path to the database file
      --filter-json string    A temporary JSON string with filter conditions to use for annotating the tree
      --format string         Output format for the tree (json or text) (default "json")
      --profile-name string   Name of a saved filter profile to use for annotating the tree
      --project-path string   Path to the project

```

---

### code-prompt-core cache

```text
The "cache" command group contains subcommands for creating, updating, and checking the status of a project's file metadata cache.

Usage:
  code-prompt-core cache [command]

Available Commands:
  status      Quickly checks if the project cache is stale
  update      Create or update the cache for a project (full or incremental)

Use "code-prompt-core cache [command] --help" for more information about a command.

```

---

#### code-prompt-core cache status

```text
Quickly checks if the project's cache is stale by comparing file system metadata with the database records.
This is a very fast, read-only operation designed to give a UI a quick signal on whether a "cache update" is needed.
It checks for new files, deleted files, and modified files based on their last modification time.

This check respects .gitignore, but it IGNORES the default preset exclusions, because the creation of a new
'node_modules' directory is itself a change that makes the cache stale.

Usage:
  code-prompt-core cache status [flags]

Flags:
      --db string             Path to the database file
      --no-git-ignores        Disable .gitignore file parsing
      --project-path string   Path to the project

```

---

#### code-prompt-core cache update

```text
Performs a fresh (full) or incremental scan of the specified project and updates the cache.

By default, this command automatically ignores common dependency directories (like node_modules, venv, target), 
only scans text files, and respects .gitignore rules. Use flags to modify this behavior.

Parameters:
  --db <path> (string, required)
    Path to the database file.

  --project-path <path> (string, required)
    Path to the project's root directory.

  --incremental (bool, optional)
    If set, performs an efficient incremental scan. This is much faster for subsequent scans.

  --no-preset-excludes (bool, optional)
    Disable the default exclusion of common dependency directories. Use this if you need to scan inside
    folders like 'node_modules', 'venv', 'target', etc.

  --include-binary (bool, optional)
    If set, the scan will include binary files. The default behavior is to ignore them.

  --no-git-ignores (bool, optional)
    Disable automatic parsing of .gitignore files.

Example Usage:
  # Perform an initial scan using all smart defaults
  ./code-prompt-core.exe cache update --db my.db --project-path /path/to/project

  # Perform a faster, incremental scan
  ./code-prompt-core.exe cache update --db my.db --project-path /path/to/project --incremental

  # Scan everything, including binaries and node_modules
  ./code-prompt-core.exe cache update --db my.db --project-path /path/to/project --include-binary --no-preset-excludes

Usage:
  code-prompt-core cache update [flags]

Flags:
      --db string             Path to the database file
      --include-binary        Include binary files in the scan (default is text-only)
      --incremental           Perform an incremental scan instead of a full one
      --no-git-ignores        Disable .gitignore file parsing
      --no-preset-excludes    Disable the default exclusion of common dependency directories
      --project-path string   Path to the project

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

Use "code-prompt-core config [command] --help" for more information about a command.

```

---

#### code-prompt-core config get

```text
Gets the value for a given key

Usage:
  code-prompt-core config get [flags]

Flags:
      --db string    Path to the database file
      --key string   The configuration key to get

```

---

#### code-prompt-core config set

```text
Sets a value for a given key

Usage:
  code-prompt-core config set [flags]

Flags:
      --db string      Path to the database file
      --key string     The configuration key
      --value string   The configuration value to set

```

---

### code-prompt-core content

```text
Retrieve file contents

Usage:
  code-prompt-core content [command]

Available Commands:
  get         Batch gets the content of specified files for a project

Use "code-prompt-core content [command] --help" for more information about a command.

```

---

#### code-prompt-core content get

```text
Retrieves the contents of multiple files at once. Provide a list of files with --files-json, or a filter with --profile-name or --filter-json.

Usage:
  code-prompt-core content get [flags]

Flags:
      --db string             Path to the database file
      --filter-json string    A temporary JSON string with filter conditions to use
      --profile-name string   Name of a saved filter profile to use
      --project-path string   Path to the project

```

---

### code-prompt-core profiles

```text
Manage filter profiles

Usage:
  code-prompt-core profiles [command]

Available Commands:
  delete      Delete a filter profile
  list        List all profiles for a project
  load        Load a filter profile
  save        Save a filter profile

Use "code-prompt-core profiles [command] --help" for more information about a command.

```

---

#### code-prompt-core profiles delete

```text
Delete a filter profile

Usage:
  code-prompt-core profiles delete [flags]

Flags:
      --db string             Path to the database file
      --name string           Name of the profile
      --project-path string   Path to the project

```

---

#### code-prompt-core profiles list

```text
List all profiles for a project

Usage:
  code-prompt-core profiles list [flags]

Flags:
      --db string             Path to the database file
      --project-path string   Path to the project

```

---

#### code-prompt-core profiles load

```text
Load a filter profile

Usage:
  code-prompt-core profiles load [flags]

Flags:
      --db string             Path to the database file
      --name string           Name of the profile
      --project-path string   Path to the project

```

---

#### code-prompt-core profiles save

```text
Saves or updates a filter configuration as a named profile.
If a profile with the same name already exists for the project, it will be overwritten.
The --data flag accepts a JSON string with the same structure as the --filter-json flag in the 'analyze:filter' command.

Example:
--data '{"excludedExtensions":[".tmp", ".bak"], "isTextOnly":true}'

Usage:
  code-prompt-core profiles save [flags]

Flags:
      --data string           JSON data for the profile
      --db string             Path to the database file
      --name string           Name of the profile
      --project-path string   Path to the project

```

---

### code-prompt-core project

```text
The "project" command group allows you to add, list, and delete project records in the database.

Usage:
  code-prompt-core project [command]

Available Commands:
  add         Adds a new project to the database without scanning
  delete      Delete a project and all its associated data
  list        List all projects stored in the database

Use "code-prompt-core project [command] --help" for more information about a command.

```

---

#### code-prompt-core project add

```text
This lightweight command creates a project record, allowing profile management or other configurations before the first scan.
It will not return an error if the project already exists.

Example Usage:
./code-prompt-core.exe project add --db my.db --project-path /path/to/my-new-project

Usage:
  code-prompt-core project add [flags]

Flags:
      --db string             Path to the database file
      --project-path string   Path to the project

```

---

#### code-prompt-core project delete

```text
Deletes a project record from the database.
Due to 'ON DELETE CASCADE' in the database schema, this will also automatically delete all associated file metadata and saved profiles for that project.

Example Usage:
./code-prompt-core.exe project delete --db my.db --project-path /path/to/project-to-delete

Usage:
  code-prompt-core project delete [flags]

Flags:
      --db string             Path to the database file
      --project-path string   Path to the project

```

---

#### code-prompt-core project list

```text
Retrieves and displays a list of all projects currently managed in the specified database file, along with their last scan timestamp.

Example Usage:
./code-prompt-core.exe project list --db my.db

Usage:
  code-prompt-core project list [flags]

Flags:
      --db string   Path to the database file

```

---

