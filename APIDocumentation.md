# Code Analysis Kernel API Documentation

This document provides a detailed API specification for the Code Analysis Kernel, a standalone command-line tool. It is intended for developers building user interfaces or scripts that interact with the kernel.

## Global Conventions

*   All commands must receive a `--db <path>` parameter, pointing to the SQLite database file.
*   All successful output to `stdout` is a UTF-8 encoded JSON string with a standardized `Response` structure.
*   All logs, warnings, and error messages are output to `stderr` as a standardized `Response` JSON string.
*   Upon error, the program exits with a non-zero status code.

### Standardized JSON Response Structure

```json
{
  "status": "success" | "error",
  "data": <any_json_value_on_success>, // Omitted on error
  "message": <string_on_error> // Omitted on success
}
```

## Commands

---

### `cache update`

**Description**: Performs a fresh scan of the specified project and overwrites the old cache with the new results.

**Parameters**:

| Flag                 | Type   | Required | Description                                     |
| :------------------- | :----- | :------- | :---------------------------------------------- |
| `--db`               | string | Yes      | Path to the database file                       |
| `--project-path`     | string | Yes      | Path to the project                             |
| `--config-json`      | string | No       | JSON string with explicit exclusion rules       |
| `--no-default-ignores` | bool   | No       | Disable built-in default ignore rules           |
| `--no-git-ignores`   | bool   | No       | Disable automatic parsing of `.gitignore` files |
| `--text-only`        | bool   | No       | Only scan and cache text files                  |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": {
    "status": "cache updated",
    "filesScanned": 123
  }
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error initializing database: unable to open database file: no such file or directory"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe cache update --db my_projects.db --project-path /path/to/your/project
```

---

### `analyze filter`

**Description**: Filters the project's current cache based on complex dynamic criteria.

**Parameters**:

| Flag             | Type   | Required | Description                                     |
| :--------------- | :----- | :------- | :---------------------------------------------- |
| `--db`           | string | Yes      | Path to the database file                       |
| `--project-path` | string | Yes      | Path to the project                             |
| `--filter-json`  | string | Yes      | JSON string with filter conditions              |

**`--filter-json` Structure**:

The JSON object can contain the following keys:
- `"excludedExtensions"`: An array of strings representing file extensions to exclude (e.g., `[".go", ".md"]`).
- `"excludedPrefixes"`: An array of strings representing path prefixes to exclude.
- `"isTextOnly"`: A boolean value that, if `true`, only includes text files in the output.

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": [
    {
      "relative_path": "src/main.go",
      "filename": "main.go",
      "extension": ".go",
      "size_bytes": 1024,
      "line_count": 50,
      "is_text": true
    }
  ]
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error parsing filter JSON: invalid character 't' looking for beginning of value"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe analyze filter --db my_projects.db --project-path /path/to/your/project --filter-json '{"isTextOnly":true, "excludedExtensions":[".exe"]}'
```

---

### `analyze stats`

**Description**: Generates statistical information about the project's current cache.

**Parameters**:

| Flag             | Type   | Required | Description                       |
| :--------------- | :----- | :------- | :-------------------------------- |
| `--db`           | string | Yes      | Path to the database file         |
| `--project-path` | string | Yes      | Path to the project               |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": {
    "totalFiles": 100,
    "totalSize": 1048576,
    "totalLines": 50000,
    "byExtension": {
      ".go": {
        "fileCount": 50,
        "totalSize": 512000,
        "totalLines": 25000
      },
      ".md": {
        "fileCount": 10,
        "totalSize": 10240,
        "totalLines": 500
      }
    }
  }
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error finding project: sql: no rows in result set"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe analyze stats --db my_projects.db --project-path /path/to/your/project
```

---

### `analyze tree`

**Description**: Generates a pre-formatted, directly displayable UTF-8 string representing the project's file structure tree.

**Parameters**:

| Flag             | Type   | Required | Description                       |
| :--------------- | :----- | :------- | :-------------------------------- |
| `--db`           | string | Yes      | Path to the database file         |
| `--project-path` | string | Yes      | Path to the project               |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```
├── cmd
│   ├── cache.go
│   ├── project.go
│   └── root.go
├── go.mod
├── go.sum
└── main.go
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error finding project: sql: no rows in result set"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe analyze tree --db my_projects.db --project-path /path/to/your/project
```

---

### `profiles save`

**Description**: Saves a user-defined filter profile.

**Parameters**:

| Flag             | Type   | Required | Description                                     |
| :--------------- | :----- | :------- | :---------------------------------------------- |
| `--db`           | string | Yes      | Path to the database file                       |
| `--project-path` | string | Yes      | Path to the project                             |
| `--name`         | string | Yes      | Name of the profile                             |
| `--data`         | string | Yes      | JSON data for the profile (same as `filter-json`) |

**`--data` Structure**:

The JSON object can contain the following keys:
- `"excludedExtensions"`: An array of strings representing file extensions to exclude (e.g., `[".go", ".md"]`).
- `"excludedPrefixes"`: An array of strings representing path prefixes to exclude.
- `"isTextOnly"`: A boolean value that, if `true`, only includes text files in the output.

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": "Profile 'my-profile' saved successfully."
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error saving profile: UNIQUE constraint failed: profiles.project_id, profiles.profile_name"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe profiles save --db my_projects.db --project-path /path/to/your/project --name my-profile --data '{"isTextOnly":true}'
```

---

### `profiles list`

**Description**: Lists all saved profiles for a project.

**Parameters**:

| Flag             | Type   | Required | Description                       |
| :--------------- | :----- | :------- | :-------------------------------- |
| `--db`           | string | Yes      | Path to the database file         |
| `--project-path` | string | Yes      | Path to the project               |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": [
    {
      "name": "my-profile",
      "data": "{\"isTextOnly\":true}"
    }
  ]
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error finding project: sql: no rows in result set"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe profiles list --db my_projects.db --project-path /path/to/your/project
```

---

### `profiles load`

**Description**: Loads a saved filter profile.

**Parameters**:

| Flag             | Type   | Required | Description                       |
| :--------------- | :----- | :------- | :-------------------------------- |
| `--db`           | string | Yes      | Path to the database file         |
| `--project-path` | string | Yes      | Path to the project               |
| `--name`         | string | Yes      | Name of the profile               |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": {
    "isTextOnly": true
  }
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error loading profile: sql: no rows in result set"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe profiles load --db my_projects.db --project-path /path/to/your/project --name my-profile
```

---

### `profiles delete`

**Description**: Deletes a saved filter profile.

**Parameters**:

| Flag             | Type   | Required | Description                       |
| :--------------- | :----- | :------- | :-------------------------------- |
| `--db`           | string | Yes      | Path to the database file         |
| `--project-path` | string | Yes      | Path to the project               |
| `--name`         | string | Yes      | Name of the profile               |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": "Profile 'my-profile' deleted successfully."
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "no profile found with name: non-existent-profile"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe profiles delete --db my_projects.db --project-path /path/to/your/project --name my-profile
```

---

### `project list`

**Description**: Lists all projects stored in the database.

**Parameters**:

| Flag | Type   | Required | Description               |
| :--- | :----- | :------- | :------------------------ |
| `--db` | string | Yes      | Path to the database file |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": [
    {
      "project_path": "/path/to/your/project",
      "last_scan_timestamp": "2025-08-10T12:34:56Z"
    }
  ]
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "error initializing database: unable to open database file: no such file or directory"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe project list --db my_projects.db
```

---

### `project delete`

**Description**: Deletes a project and all its associated data from the database.

**Parameters**:

| Flag             | Type   | Required | Description                       |
| :--------------- | :----- | :------- | :-------------------------------- |
| `--db`           | string | Yes      | Path to the database file         |
| `--project-path` | string | Yes      | Path to the project               |

**Input (`stdin`)**: None

**Output (`stdout`) - Success Example**:

```json
{
  "status": "success",
  "data": "Project \\'/path/to/your/project\\' deleted successfully."
}
```

**Error Output (`stderr`) - Example**:

```json
{
  "status": "error",
  "message": "no project found with path: /path/to/non-existent-project"
}
```

**Example Usage**:

```bash
./code-prompt-core.exe project delete --db my_projects.db --project-path /path/to/your/project
```
