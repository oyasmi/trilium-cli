[English](README.md) | **简体中文**

# trilium

*在终端里使用你的 Trilium 笔记——为 AI agent 打造，人用起来也顺手。*

`trilium` 是 [Trilium Notes](https://github.com/TriliumNext/Trilium) 的命令行工具。
它通过 Trilium 官方接口（ETAPI）连接你的服务器，让你在终端、脚本或 AI agent 里
搜索、阅读、编写和整理笔记。

## 它能做什么

- **为 AI agent 优化。** 输出简短、格式稳定：每条笔记一行、ID 在最前面，每次修改只输出
  一行结果。一次典型的 `get` 输出只有原始接口返回内容的四分之一左右，很多任务一次调用就够。
- **精确。** 笔记内容按字节原样读写，工具绝不在背后改写你的 HTML 或代码。
- **覆盖完整的 API。** 笔记、树结构（移动和克隆）、标签和关系、附件、修订版本、日记、
  历史记录、恢复删除、导入导出。
- **小心对待你的数据。** 拒绝把笔记移进它自己的子树、拒绝误删笔记的最后一个位置、拒绝用
  `create --id` 覆盖已有笔记；修改 Trilium 的系统笔记需要加 `--force`。出错时如果已经改动了
  一部分，错误信息会明确说明。删除的笔记可以恢复。
- **单个小巧的可执行文件。** 不需要运行环境，也没有配置文件：设置两个环境变量就能用。

## 安装

**交给你的 AI agent 来做。** 把下面这段文字复制给 Claude Code、Codex 或其他编程 agent。
没装过就安装 CLI 和 skill，装过就把两者都更新到最新版——以后想升级时再发一次即可。

```text
在这台电脑上安装或更新 `trilium` 命令行工具（https://github.com/oyasmi/trilium-cli）及其 agent skill。请逐步执行，最后告诉我你做了什么。

1. 检查现状：运行 `trilium --version`（记下版本；如果已安装，用 `command -v trilium`，Windows 上用 `where trilium` 查出可执行文件的位置）。再看看你自己的 skills 目录里是否已有名为 `trilium` 的 skill 目录。
2. 查最新版本：请求 https://api.github.com/repos/oyasmi/trilium-cli/releases/latest ，读取 `tag_name`（例如 v0.1.0；可执行文件显示的版本不带 "v"：`trilium 0.1.0 (commit …)`）。如果已安装的可执行文件已经是这个版本，就跳过第 5 步，但其余步骤照做：skill 可能过时或缺失。
3. 选择适合本机系统和 CPU 的文件：`trilium_<不带 v 的版本号>_<os>_<arch>.tar.gz`，os 为 darwin 或 linux，arch 为 amd64 或 arm64；Windows 上是 `…_windows_<arch>.zip`。把它和同一个 release 里的 `checksums.txt` 下载到一个新建的临时目录，用 checksums.txt 校验压缩包的 SHA-256，不一致就停止。
4. 解压。压缩包里有可执行文件 `trilium`（Windows 上是 `trilium.exe`），skill 在 `skills/trilium/` 目录。
5. 安装可执行文件：
   - 更新：直接替换原位置的旧文件。
   - 新装：放到 `~/.local/bin`（不存在就创建），Windows 上放到 `%LOCALAPPDATA%\Programs\trilium`。如果该目录不在 PATH 中，告诉我应该往 shell 配置文件里加哪一行，不要自己改配置文件。
   - macOS 上如果系统因为"来自互联网"而阻止运行，执行 `xattr -d com.apple.quarantine <路径>`。
   - 需要 sudo 或要写入我的主目录以外的位置时，先问我。
6. 安装 skill：把整个 `skills/trilium` 目录复制到你所属 agent 的 skills 目录（Claude Code：`~/.claude/skills/trilium`；Codex：`~/.codex/skills/trilium`；其他 agent：对应的目录，不确定就问我）。如果已有 `trilium` skill 目录，先和新的比较：有差异就整个替换旧目录（不要留下过时的参考文件），完全相同就保持不动。
7. 验证：`trilium --version` 必须显示新版本。如果已设置 TRILIUM_URL 和 TRILIUM_ETAPI_TOKEN，再运行 `trilium info`，确认显示 `auth:    ok`。如果没有设置，告诉我设置方法：在 Trilium 中打开 选项 → ETAPI 创建 token，然后 `export TRILIUM_URL=<我的 Trilium 地址>`、`export TRILIUM_ETAPI_TOKEN=<token>`。不要显示 token，除非我要求，否则不要把它写进任何文件。
8. 删除临时目录，然后汇报：可执行文件的旧版本 → 新版本（或"已是最新"）、skill 是新装 / 已更新 / 无变化、两者的路径，以及我还需要做的事。
```

**或者自己下载**：从 [Releases 页面](https://github.com/oyasmi/trilium-cli/releases) 下载适合你系统的压缩包，
解压后把 `trilium` 放到 `PATH` 中的某个目录（例如 `/usr/local/bin`）。

**或者用 Go 安装**（需要 Go 1.27 或更新版本）：

```bash
go install github.com/oyasmi/trilium-cli/cmd/trilium@latest
```

## 设置（3 步）

1. 在 Trilium 中打开 **选项 → ETAPI**，点击 **创建新的 ETAPI 令牌**，复制生成的 token。
2. 告诉 `trilium` 服务器地址和 token：

   ```bash
   export TRILIUM_URL=https://notes.example.com     # 就是你在浏览器里打开 Trilium 的地址
   export TRILIUM_ETAPI_TOKEN=<你的 token>
   ```

   把这两行加到 `~/.bashrc` 或 `~/.zshrc` 里，以后就不用再设置。
3. 检查是否可用：

   ```
   $ trilium info
   server:  https://notes.example.com (Trilium 0.106.1)
   auth:    ok
   notes:   424 active, 160 deleted
   inbox:   tA1b2C3d "Inbox"
   agent:   (not set)
   ```

## 快速上手

```
$ trilium search budget --in N2lwDR1ByORY
2CgAMHEAjVXT	text	Budget review

$ trilium get 2CgAMHEAjVXT
---
id: 2CgAMHEAjVXT
title: Budget review
type: text (text/html)
parents: N2lwDR1ByORY
attrs: #status=todo
children: 0
attachments: 0
created: 2026-10-25 18:42
modified: 2026-10-25 18:42
---
<p>Q3 budget approved.</p>

$ trilium create "Supplier call" "<p>Delivery moved to Friday.</p>" -p N2lwDR1ByORY
created J7UZ1SubYZw6 text "Supplier call" in N2lwDR1ByORY

$ trilium append @today "<p>Called the supplier.</p>"
appended HN04PrpcQg3H (@today) +27 bytes: 58 → 85 bytes

$ trilium ls N2lwDR1ByORY
2CgAMHEAjVXT	text	Budget review
OaVTKVwlYbCb	text	Launch checklist
J7UZ1SubYZw6	text	Supplier call
```

不加 `-p` 时，`create` 会把新笔记放进你的收件箱（`@inbox`）。

运行 `trilium help` 查看命令列表，`trilium help <命令>` 查看某个命令的详细说明。

## 日常用法

**搜索**——使用 Trilium 自己的搜索语法，`trilium help search` 里有速查表。

```bash
trilium search meeting notes
trilium search '#status=todo' --in <项目> --sort dateModified --desc
trilium search 'note.title="report.pdf"'      # 含 "." 或 "-" 的值要加引号
```

**阅读**——一次读多条；`--text` 快速查看纯文本。

```bash
trilium get <id1> <id2>
trilium get <id> --text
trilium get <id> -c > page.html                # 只输出原始内容
```

**写入**——text 笔记存的是 HTML。长内容最好先写到文件里。

```bash
trilium create "Release plan" -f plan.html -p <父笔记>
trilium create -f script.py -p <父笔记>        # .py 文件会变成代码笔记
trilium append <id> "<p>One more thing.</p>"
trilium put <id> -f page.html                  # 整体替换
```

**修改笔记的一部分**——精确查找替换，要么全部成功，要么都不改。

```
$ trilium edit 2CgAMHEAjVXT --old "<p>Q3 budget approved.</p>" --new "<p>Q3 budget approved (final).</p>"
edited 2CgAMHEAjVXT 1 replacement: 26 → 34 bytes
```

**整理**——标签、移动、克隆。

```
$ trilium attr 2CgAMHEAjVXT '#status=done'
set #status=done on 2CgAMHEAjVXT
$ trilium mv OaVTKVwlYbCb 1DKciU1K11Yy
moved OaVTKVwlYbCb "Launch checklist": N2lwDR1ByORY → 1DKciU1K11Yy
$ trilium branch add OaVTKVwlYbCb N2lwDR1ByORY
cloned OaVTKVwlYbCb into N2lwDR1ByORY (branch N2lwDR1ByORY_OaVTKVwlYbCb)
```

**附件**——上传后，把 `embed:` 那一行 HTML 放进笔记即可显示图片。

```
$ trilium att add 2CgAMHEAjVXT -f chart.png
added attachment ldExtmPZnwBR to 2CgAMHEAjVXT: "chart.png" image/png 69 bytes
embed: <img src="api/attachments/ldExtmPZnwBR/image/chart.png">
```

**日记**——`@today`、`@yesterday`、`@2026-10-25`、`@2026-10` ……

```bash
trilium append @today -f entry.html
trilium ls @2026-10                            # 某个月的每日笔记
```

**Markdown**——由 Trilium 自己把它转换成富文本笔记。

```
$ trilium import N2lwDR1ByORY plan.md
imported 2EfMSwP15Bnb "plan" into N2lwDR1ByORY
```

**撤销**——找到并恢复删除的笔记。

```bash
trilium history -n 20
trilium undelete <id>
```

## 配合 AI agent 使用

仓库里自带一个现成的 skill：[`skills/trilium`](skills/trilium/)。它教 agent 如何用好这个工具：
核心概念、Trilium 需要的 HTML 写法、常用模式和容易踩的坑。

- **Claude Code**：把这个目录复制到 `~/.claude/skills/trilium`（所有项目可用），
  或项目内的 `.claude/skills/trilium`。

  ```bash
  cp -r skills/trilium ~/.claude/skills/
  ```

- **Codex**：把这个目录复制到 `~/.codex/skills/trilium`。

  ```bash
  cp -r skills/trilium ~/.codex/skills/
  ```

- **其他 agent**：把 `skills/trilium/SKILL.md` 的内容加入系统提示词，并让 agent 能读取
  `references/` 下的文件。
- **多个 agent 共用一个知识库**：用 `TRILIUM_AGENT=<名字>` 给每个 agent 起名。它创建的
  每条笔记都会带上标签 `#agent=<名字>`，方便查看是谁写的：
  `trilium search '#agent="claude-1"'`。
- **一条笔记只由一个写入者修改。** Trilium 的接口没有条件写入，两个 agent 同时修改同一条笔记
  仍可能丢失更新。请让每个 agent 写自己的笔记（共享日志可以每条记录一条笔记）。

任何命令加上 `--json` 都会输出 JSON Lines（字段名与 ETAPI 一致），方便脚本处理。

## 笔记快捷方式

凡是需要笔记的地方，都可以写笔记 ID、`root`，或者：

| 写法 | 含义 |
|---|---|
| `@inbox` | 收件箱（带 `#inbox` 标签的笔记，没有的话就是今天的日记） |
| `@today` `@yesterday` `@tomorrow` | 当天的日记 |
| `@2026-10-25` | 指定日期的日记 |
| `@2026-W43` | 周笔记（需要在日历根笔记上设置 `#enableWeekNote`） |
| `@2026-10` / `@2026` | 月 / 年笔记 |

和 Trilium 本身的行为一致，日记笔记不存在时会自动创建。日期按你电脑的时区计算。

## 命令一览

| 分组 | 命令 |
|---|---|
| 日常 | `search` `ls` `get` `create` `append` `edit` `put` `rm` |
| 整理 | `update`（标题/类型）· `attr`（标签、关系）· `mv` · `branch`（克隆）· `att`（附件）· `rev`（修订版本） |
| 更多 | `history` · `export` · `import` · `undelete` · `info` · `backup` · `version` |

退出码：`0` 成功（包括“没有结果”），`1` 失败，`2` 用法错误或缺少设置，`3` 找不到。

## 常见问题

- **`TRILIUM_URL is not set` / `TRILIUM_ETAPI_TOKEN is not set`**——按“设置”一节导出这两个
  变量。地址就是你打开 Trilium 用的地址，末尾带不带 `/` 或 `/etapi` 都可以。
- **`could not reach TRILIUM_URL`**——检查地址、网络，以及服务器是否在运行。
- **`Not authenticated [NOT_AUTHENTICATED]`**——token 不对或已被删除。到 选项 → ETAPI
  重新创建一个。
- **搜不到东西**——含 `.` 或 `-` 的值必须加引号：`note.title="hello.py"`、
  `#date="2026-10-25"`。整个查询也要用引号包起来，避免被 shell 解释：
  `trilium search '#status=todo'`。
- **文字挤成了一行**——text 笔记存的是 HTML。用 `<p>…</p>` 包住段落；纯文本请用代码笔记；
  Markdown 可以用 `trilium import <父笔记> file.md` 转换。
- **加密笔记**无法通过 API 读取或修改。

## 兼容性

已在 Trilium（TriliumNext）**0.106.1** 上测试。更老的服务器可能缺少部分接口
（恢复删除、历史记录、修订版本列表、周笔记、metrics），其他命令不受影响。

## 参与贡献

欢迎提交 issue 和 pull request。设计说明见 [docs/design.md](docs/design.md)。

```bash
make build      # 生成 ./trilium
make test       # go vet + 单元测试和命令测试
make lint       # golangci-lint
make e2e        # 针对真实服务器的端到端测试（只在临时创建的沙盒笔记里写入）
```

## 许可证

[Apache License 2.0](LICENSE)
