### **代码扫描与报告工具 - 内核项目技术方案蓝图**

**文档更新时间：** 2025年8月10日

#### **1. 项目愿景与核心架构**

  * **愿景**: 构建一个高性能、跨平台的代码分析内核。该内核作为独立的命令行工具存在，能够被多种用户界面（如现有的Python Tkinter GUI、未来的Web界面或其他脚本）调用。
  * **核心架构**: **单一DB文件驱动的自治内核**。
      * 内核的所有状态（包括多个项目的扫描缓存和过滤方案）都存储在一个由调用方（UI）指定的单一SQLite数据库文件中。
      * 内核本身是自治的，负责管理该DB文件内部的所有数据结构、业务逻辑和数据生命周期。
      * UI与内核之间通过标准化的命令行调用进行交互，数据交换主要使用JSON格式。

#### **2. 内核技术选型**

  * **开发语言**: **Go** 或 **Rust** (由您根据开发速度与性能控制的偏好最终决定)。
  * **关键依赖库推荐**:
      * **CLI框架**:
          * Go: `Cobra`, `urfave/cli`
          * Rust: `clap` (功能强大，推荐)
      * **SQLite驱动**:
          * Go: `go-sqlite3`
          * Rust: `rusqlite` (直接封装) 或 `sqlx` (提供编译时检查)
      * **文件与忽略规则处理**:
          * Go: 标准库 `filepath`，`doublestar` (用于glob匹配)
          * Rust: `walkdir` (目录遍历)，`ignore` (完美支持.gitignore及多层规则)

#### **3. 统一数据库设计 (SQLite)**

内核在接收到`--db <path>`参数时，如果文件不存在，将自动按以下结构创建。

  * **`projects` 表**: 存储项目基本信息。

    ```sql
    CREATE TABLE IF NOT EXISTS projects (
        id                INTEGER PRIMARY KEY AUTOINCREMENT,
        project_path      TEXT NOT NULL UNIQUE,
        last_scan_timestamp TEXT NOT NULL
    );
    ```

  * **`file_metadata` 表**: 作为项目最新扫描结果的缓存。

    ```sql
    CREATE TABLE IF NOT EXISTS file_metadata (
        project_id    INTEGER NOT NULL,
        relative_path TEXT NOT NULL,
        filename      TEXT NOT NULL,
        extension     TEXT,
        size_bytes    INTEGER NOT NULL,
        line_count    INTEGER NOT NULL,
        is_text       BOOLEAN NOT NULL,
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
    );
    CREATE INDEX IF NOT EXISTS idx_file_metadata_project_id ON file_metadata(project_id);
    ```

  * **`profiles` 表**: 持久化存储用户自定义的过滤方案。

    ```sql
    CREATE TABLE IF NOT EXISTS profiles (
        id                  INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id          INTEGER NOT NULL,
        profile_name        TEXT NOT NULL,
        profile_data_json   TEXT NOT NULL,
        UNIQUE (project_id, profile_name),
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
    );
    ```

#### **4. 命令行接口 (CLI) API 详细规范**

**全局约定**:

  * 所有命令都必须接收一个`--db <path>`参数，指向数据文件。
  * 所有成功输出到`stdout`的数据均为UTF-8编码的JSON字符串（`analyze:tree`除外）。
  * 所有日志、警告和错误信息都输出到`stderr`。发生错误时，程序以非零状态码退出。

-----

##### **A. 缓存管理 (Cache)**

  * **`cache:update`**
      * **描述**: 对指定项目执行一次全新的扫描，并用结果**覆盖**旧的缓存。
      * **参数**:
          * `--db <path>` (必需)
          * `--project-path <path>` (必需)
          * `--config-json <JSON>` (可选): 包含`excludedPaths`等显式排除规则。
          * `--no-default-ignores` (可选标志): 禁用内置的默认忽略规则。
          * `--no-git-ignores` (可选标志): 禁用对`.gitignore`文件的自动解析。
          * `--text-only` (可选标志): 只扫描并缓存文本文件。
      * **核心逻辑**:
        1.  连接DB，根据`project_path`找到或创建`project_id`。
        2.  执行`DELETE FROM file_metadata WHERE project_id = ?`，清空旧缓存。
        3.  应用三层忽略规则（详见第5节），遍历文件系统，对每个文件进行元数据提取、行数统计和文本/二进制检测。
        4.  将结果插入`file_metadata`表。
        5.  更新`projects`表的`last_scan_timestamp`字段。
      * **成功输出**: `{"status": "cache updated", "filesScanned": <int>}`

-----

##### **B. 数据分析 (Analysis)**

  * **`analyze:filter`**

      * **描述**: 对项目的当前缓存进行复杂的动态过滤。
      * **参数**:
          * `--db <path>` (必需)
          * `--project-path <path>` (必需)
          * `--filter-json <JSON>` (必需): 包含`excludedExtensions`, `excludedPrefixes`, `isTextOnly`等过滤条件。
      * **成功输出**: `[{...}, ...]` 文件元数据对象的JSON数组。

  * **`analyze:stats`**

      * **描述**: 生成项目当前缓存的统计信息。
      * **参数**: `--db <path>` (必需), `--project-path <path>` (必需)
      * **成功输出**: 包含总计和按扩展名分组统计的JSON对象。

  * **`analyze:tree`**

      * **描述**: 生成项目当前缓存的文件结构树。
      * **参数**: `--db <path>` (必需), `--project-path <path>` (必需)
      * **成功输出**: 预格式化的、可以直接显示的UTF-8字符串。

-----

##### **C. 方案配置 (Profiles)**

  * `profiles:list`, `profiles:load`, `profiles:save`, `profiles:delete`
      * **描述**: 对过滤方案进行增、删、改、查、列。
      * **关键参数**: `--db <path>`, `--project-path <path>`, `--name <string>`, `[--data <JSON>]`

-----

##### **D. 项目与数据维护 (Project & DB Management)**

  * `project:list`, `project:delete`, `db:compact`
      * **描述**: 管理DB中的项目列表及数据库文件自身。
      * **关键参数**: `--db <path>`, `[--project-path <path>]`

#### **5. 忽略规则与配置**

1.  **分层体系**: 内核严格按照以下优先级合并规则：
      * **最高优先级**: 用户通过`--config-json`传入的显式规则。
      * **中优先级**: 项目内的`.gitignore`文件规则（可通过`--no-git-ignores`禁用）。
      * **最低优先级**: 内核的默认忽略规则（可通过`--no-default-ignores`禁用）。
2.  **默认规则配置**:
      * 内核在启动时会尝试读取用户主目录下的全局配置文件（例如 `~/.config/reporter/config.toml`）。
      * 如果该文件存在，则使用其中定义的`ignore_patterns`作为默认规则。
      * 如果不存在，则使用内核内部硬编码的“出厂默认值”。

#### **6. 调用方 (UI) 实现指南**

  * **状态管理**: UI的核心状态是**当前DB文件的路径**。应用启动时应加载或让用户指定此路径。
  * **核心工作流**:
    1.  用户选择一个项目。
    2.  UI调用`project:list`检查该项目是否存在于DB中。
    3.  **若存在**: UI可显示上次扫描时间，并立即进入分析状态（调用`analyze:filter`等）。提供一个“刷新”按钮，该按钮会触发`cache:update`。
    4.  **若不存在**: UI必须调用`cache:update`进行首次扫描，成功后才进入分析状态。
  * **与内核交互 (Python示例)**:
    ```python
    import subprocess
    import json

    def call_kernel(args):
        try:
            # 核心是使用 subprocess.run
            process = subprocess.run(
                ['/path/to/reporter-core'] + args,
                capture_output=True,
                text=True,
                check=True,  # 如果内核返回非零状态码，则抛出异常
                encoding='utf-8'
            )
            # 实时读取stderr可以在Popen中实现，这里简化为结束后读取
            if process.stderr:
                print(f"[KERNEL LOG]: {process.stderr}")

            # 解析stdout的JSON输出
            return json.loads(process.stdout) if process.stdout else {"status": "success"}

        except FileNotFoundError:
            print("错误: 未找到内核可执行文件。")
            return None
        except subprocess.CalledProcessError as e:
            print(f"错误: 内核执行失败，状态码 {e.returncode}")
            print(f"错误信息: {e.stderr}")
            return None
        except json.JSONDecodeError:
            print(f"错误: 无法解析内核的输出: {process.stdout}")
            return None

    # 调用示例
    # db_path = "/path/to/main.db"
    # project_path = "/path/to/my_project"
    # filter_result = call_kernel([
    #     'analyze:filter',
    #     '--db', db_path,
    #     '--project-path', project_path,
    #     '--filter-json', '{"isTextOnly": true}'
    # ])
    ```

#### **7. 开发实施路线图**

1.  **阶段一：内核基础搭建**
      * 选择Go或Rust，建立项目结构。
      * 引入CLI和SQLite库。
      * 实现DB初始化逻辑（根据Schema创建表）。
      * 实现`project:list`命令，作为最基础的连通性测试。
2.  **阶段二：核心功能实现**
      * 实现`cache:update`命令的完整扫描逻辑（文件遍历、元数据提取、文本检测、行数统计）。
      * 实现三层忽略规则体系。
      * 实现`analyze:filter`, `analyze:stats`, `analyze:tree`命令。
3.  **阶段三：辅助功能完善**
      * 实现完整的`profiles:*`命令集。
      * 实现`project:delete`和`db:compact`。
4.  **阶段四：UI集成与测试**
      * 修改现有Python GUI，按照第6节的指南，将其所有后端调用逻辑替换为对新内核的`subprocess`调用。
      * 编写单元测试和集成测试，确保API行为符合规范。
5.  **阶段五：打包与分发**
      * 编写CI/CD脚本，实现跨平台编译（Windows, macOS, Linux）。
      * 将内核二进制文件与Python GUI应用一同打包。