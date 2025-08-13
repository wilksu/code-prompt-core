### **代码扫描与报告工具 - 内核项目技术方案蓝图**

**文档更新时间：** 2025年8月12日

#### **1. 项目愿景与核心架构**

  * **愿景**: 构建一个高性能、跨平台的代码分析内核。该内核作为独立的命令行工具存在，能够被多种用户界面（如现有的Python Tkinter GUI、未来的Web界面或其他脚本）调用。
  * **核心架构**: **单一DB文件驱动的自治内核**。
      * 内核的所有状态（包括多个项目的扫描缓存和过滤方案）都存储在一个由调用方（UI）指定的单一SQLite数据库文件中。
      * 内核本身是自治的，负责管理该DB文件内部的所有数据结构、业务逻辑和数据生命周期。
      * UI与内核之间通过标准化的命令行调用进行交互，数据交换主要使用JSON格式。

#### **2. 内核技术选型**

  * **开发语言**: **Go**
  * **关键依赖库推荐**:
      * **CLI框架**: `Cobra`
      * **SQLite驱动**: `modernc.org/sqlite`
      * **忽略规则处理**: `sabhiram/go-gitignore`
      * **模板引擎**: `aymerick/raymond` (用于报告生成)
      * **配置文件解析**: `yaml.v3`

#### **3. 统一数据库设计 (SQLite)**

内核在接收到`--db <path>`参数时，如果文件不存在，将自动按以下结构创建。

  * **`projects` 表**: 存储项目基本信息。

    ```sql
    CREATE TABLE IF NOT EXISTS projects (
        id                INTEGER PRIMARY KEY AUTOINCREMENT,
        project_path      TEXT NOT NULL UNIQUE,
        last_scan_timestamp TEXT NOT NULL
    );
    ```

  * **`file_metadata` 表**: 作为项目最新扫描结果的缓存。

    ```sql
    CREATE TABLE IF NOT EXISTS file_metadata (
        project_id    INTEGER NOT NULL,
        relative_path TEXT NOT NULL,
        filename      TEXT NOT NULL,
        extension     TEXT,
        size_bytes    INTEGER NOT NULL,
        line_count    INTEGER NOT NULL,
        is_text       BOOLEAN NOT NULL,
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
    );
    CREATE INDEX IF NOT EXISTS idx_file_metadata_project_id ON file_metadata(project_id);
    ```

  * **`profiles` 表**: 持久化存储用户自定义的过滤方案。

    ```sql
    CREATE TABLE IF NOT EXISTS profiles (
        id                  INTEGER PRIMARY KEY AUTOINCREMENT,
        project_id          INTEGER NOT NULL,
        profile_name        TEXT NOT NULL,
        profile_data_json   TEXT NOT NULL,
        UNIQUE (project_id, profile_name),
        FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
    );
    ```

#### **4. 命令行接口 (CLI) API 详细规范**

**全局约定**:

  * 所有命令都必须接收一个`--db <path>`参数，指向数据文件。
  * 所有成功输出到`stdout`的数据均为UTF-8编码的JSON字符串（`analyze:tree --format=text`除外）。
  * 所有日志、警告和错误信息都输出到`stderr`。发生错误时，程序以非零状态码退出。

-----

##### **A. 缓存管理 (Cache)**
... (内容与之前类似) ...

-----

##### **B. 数据分析 (Analysis)**
... (内容与之前类似) ...

-----

##### **C. 方案配置 (Profiles)**
... (内容与之前类似) ...

-----

##### **D. 项目与数据维护 (Project & DB Management)**
... (内容与之前类似) ...

-----

##### **E. 报告生成 (New)**

  * **`report:generate`**
      * **描述**: 基于 Handlebars 模板，聚合项目分析数据并生成一份综合报告。
      * **参数**:
          * `--db <path>` (必需)
          * `--project-path <path>` (必需)
          * `--template <path>` (必需): Handlebars 模板文件路径。
          * `--output <path>` (必需): 生成报告的输出文件路径。
          * `--profile-name <string>` 或 `--filter-json <JSON>` (可选): 用于筛选报告中包含的文件内容。
      * **核心逻辑**:
        1.  根据过滤参数，获取文件列表。
        2.  聚合数据：调用 `analyze:stats` 的逻辑获取统计数据，`analyze:tree` 的逻辑获取文件树结构，`content:get` 的逻辑获取文件内容。
        3.  将所有数据（项目路径、统计、文件树、文件内容、生成时间等）组装成一个上下文（Context）对象。
        4.  渲染指定的 Handlebars 模板。
        5.  将结果写入输出文件。
      * **成功输出**: `{"status": "report generated", "outputPath": "<path>"}`

#### **5. 忽略规则与配置**

1.  **分层体系**: 内核严格按照以下优先级合并规则：
      * **最高优先级**: 用户通过`--config-json`传入的显式规则。
      * **中优先级**: 项目内的`.gitignore`文件规则（可通过`--no-git-ignores`禁用）。
      * **最低优先级**: 内核的默认忽略规则（可通过`--no-default-ignores`禁用）。
2.  **数据库路径配置 (New)**: 数据库文件的路径采用三级优先级策略：
     * **最高优先级**: 命令行通过 `--db` 标志传入的路径。
     * **中优先级**: 全局配置文件中定义的 `db_path`。
     * **最低优先级**: 程序内置的默认值 (例如: `my_projects.db`)。
3.  **默认规则配置**:
      * 内核在启动时会尝试读取用户主目录下的全局配置文件 `~/.config/code-prompt-core/config.yaml`。
      * 该文件可以定义 `db_path` 和 `ignore_patterns` 等全局默认值。
      * 如果文件不存在，则使用内核内部硬编码的“出厂默认值”。

#### **6. 调用方 (UI) 实现指南**

  * **状态管理**: UI的核心状态是**当前DB文件的路径**。应用启动时应加载或让用户指定此路径。
  * **核心工作流**:
    1.  用户选择一个项目。
    2.  UI调用`project:list`检查该项目是否存在于DB中。
    3.  **若存在**: UI可显示上次扫描时间，并立即进入分析状态（调用`analyze:filter`等）。提供一个“刷新”按钮，该按钮会触发`cache:update`。
    4.  **若不存在**: UI必须调用`cache:update`进行首次扫描，成功后才进入分析状态。
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
                check=True,  # 如果内核返回非零状态码，则抛出异常
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
4.  **阶段四：高级功能与配置 (New)**
     * 实现 `report:generate` 命令。
     * 实现 `pkg/config` 中的分层配置加载逻辑。
     * 将所有命令改造为使用新的配置获取方式。
5.  **阶段五：UI集成与测试**
      * 修改现有Python GUI，按照第6节的指南，将其所有后端调用逻辑替换为对新内核的`subprocess`调用。
      * 编写单元测试和集成测试，确保API行为符合规范。
6.  **阶段六：打包与分发**
      * 编写CI/CD脚本，实现跨平台编译（Windows, macOS, Linux）。
      * 将内核二进制文件与Python GUI应用一同打包。