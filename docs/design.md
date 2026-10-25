# trilium CLI 设计文档

> 状态：v1 设计定稿（2026-10-25）
> 适用 Trilium 版本：基于 0.106.1 实测；更老的版本可能缺少部分接口（undelete、history、revisions 列表、week note、metrics）。

---

## 1. 背景与目标

Trilium Notes 提供了功能完整的 REST 接口 ETAPI，但它是为程序设计的：返回冗长的 JSON，常见操作需要多次调用，还有不少需要踩坑才知道的行为。`trilium` 是封装在 ETAPI 之上的命令行工具，**主要使用者是 AI agent**，同时也让普通用户能方便地使用。它会作为开源工具公开发布。

**目标**

1. **省 token**：agent 会大量调用这个工具。输出、输入和调用次数上的每一点节省，都会被放大成可观的成本差异。
2. **贴近 ETAPI 语义**：不发明新概念。note、branch、attribute、attachment、revision、calendar 和 ETAPI 一一对应，只在必要时做薄薄一层组合（比如 `mv`、`attr` 的"按名字设置"）。
3. **正交、分层**：常用功能简单易用，默认值合理；高级功能通过附加参数打开。
4. **精确**：读写的内容和 Trilium 里存的一个字节都不差，CLI 绝不在中间改写内容。
5. **面向公众**：README、帮助信息、错误提示都写得清楚友好，不炫技。

**非目标（v1 不做）**

- 不在客户端做 Markdown 和 HTML 之间的格式转换。唯一的例外是 `import` 一个 `.md` 文件：这是用户显式发起的导入，转换由 Trilium 自己的导入器完成（见 5.17）。
- 不支持加密笔记（ETAPI 本身就无法打开加密会话）。
- 不提供 `/auth/login`、`/auth/logout`。token 只从环境变量读取，CLI 永远不调用 logout，否则会让用户的 token 失效。
- 不提供配置文件、交互式界面和 TUI。

---

## 2. 设计原则

| 原则 | 具体做法 |
|---|---|
| 输出适度精简 | 列表每行一条，ID 在第一列；写操作输出一行结果，包含足够的验证信息（比如字节数变化），让 agent 不用回读；不输出 ANSI 颜色、进度条和装饰性文字 |
| 不改写内容 | 内容原样读写。`--text` 是显式的**只读**视图，输出时会标明它不是原始内容 |
| 减少调用次数 | `get` 一次返回元数据和正文；`ls` 一次带回标题；`create` 创建时就能带上属性；命令支持多个 ID；`@today` 这类别名省掉"先查 ID"的调用；`edit` 一次可以替换多处 |
| 防止输出爆炸 | `search`、`ls`、`history` 默认有条数上限，超出时提示还有更多；二进制内容永远不输出到终端 |
| 帮助按需展开 | `trilium help` 一屏以内；细节放在 `trilium help <cmd>` 和 `trilium help search` 里 |
| 可组合 | 写操作输出的 ID 可以被后续命令直接使用；加 `--json` 输出 JSON Lines，便于程序处理 |
| 可靠 | 读写内容走 `-f FILE`，避免 shell 转义问题；从不自动读取 stdin（agent 的 shell 里 stdin 状态不确定，自动读可能卡死） |

---

## 3. 配置

全部通过环境变量配置，没有内置默认服务地址。

| 变量 | 必需 | 说明 |
|---|---|---|
| `TRILIUM_URL` | 是 | Trilium 服务地址，例如 `https://notes.example.com`。允许带或不带末尾的 `/`，也允许用户写成以 `/etapi` 结尾，CLI 会统一规范化为 `<url>/etapi` |
| `TRILIUM_ETAPI_TOKEN` | 是 | ETAPI token，在 Trilium 的「选项 → ETAPI」中创建 |
| `TRILIUM_AGENT` | 否 | agent 身份名。设置后，`create` 会给新笔记加上标签 `#agent=<name>`（只在创建时添加） |

缺少变量时，以退出码 2 报错，提示要指明怎么解决，例如：

```
error: TRILIUM_ETAPI_TOKEN is not set.
hint: In Trilium, open Options → ETAPI and create a token, then:
      export TRILIUM_ETAPI_TOKEN=<your token>
```

```
error: TRILIUM_URL is not set.
hint: Set it to your Trilium server address, e.g.:
      export TRILIUM_URL=https://notes.example.com
```

认证方式：请求头 `Authorization: <token>`（原始 token，实测可用）。

时区：`@today` 等日期别名按**运行 CLI 的机器的本地时间**计算，遵从 `TZ` 环境变量。显示的时间也换算成本地时间。

---

## 4. 通用约定

### 4.1 笔记引用（`<note>` 参数）

所有接受笔记的参数都支持以下写法：

| 写法 | 含义 | ETAPI |
|---|---|---|
| `abc123XYZ` | noteId，满足 `[A-Za-z0-9_]{4,32}` | — |
| `root` | 根笔记 | — |
| `@inbox` | 收件箱：有 `#inbox` 标签的笔记 → 当天的日记 → root | `GET /inbox/{今天}` |
| `@today` `@yesterday` `@tomorrow` | 日笔记 | `GET /calendar/days/{date}` |
| `@2026-10-25` | 指定日期的日笔记 | 同上 |
| `@2026-W43` | 周笔记（需要日历根笔记上有 `#enableWeekNote`） | `GET /calendar/weeks/{week}` |
| `@2026-10` | 月笔记 | `GET /calendar/months/{month}` |
| `@2026` | 年笔记 | `GET /calendar/years/{year}` |

- 日历别名遵循 ETAPI 的语义：笔记不存在时会**自动创建**，帮助和 skill 里要写明这一点。
- 写操作的结果行里，如果用户传的是别名，要在实际 ID 后面附上别名，例如 `in e2GMUhHk0e9v (@inbox)`。
- 其他格式错误的引用以退出码 2 报错。
- 属性语法里关系的目标（`~rel=<note>`）、`--in`、`-p` 等参数同样支持别名。

### 4.2 内容输入

所有写内容的命令（`create`、`put`、`append`、`att add`、`att put`）都支持以下三种来源，**同一次调用只能用一种**：

| 方式 | 写法 | 适用场景 |
|---|---|---|
| 位置参数 | `trilium append X "<p>短内容</p>"` | 一行以内的短内容 |
| 文件 | `-f path/to/file` | **推荐用于长内容**，agent 最可靠的方式；也是二进制内容的唯一方式 |
| stdin | `-f -` | 管道或 heredoc |

- 不自动读取 stdin，必须显式写 `-f -`。
- `put`、`append` 没有提供任何内容来源时报错，并提示这三种写法。`create` 没有内容时会创建空笔记。
- 内容按字节原样发送。CLI 不加换行、不去空白、不转码。唯一的例外见 `append` 的换行规则（5.5）。
- 上传时的 Content-Type 规则：
  - image、file 类型，或者内容不是合法的 UTF-8 → `application/octet-stream`；
  - 其他情况 → `text/plain`；
  - **绝不使用 `application/json`**（服务器会返回 500）。

**text 类型的提示（只警告，不修改）**：如果写入 text 笔记的内容里没有任何 HTML 标签，就在 stderr 提示：

```
warning: text notes store HTML; this content has no tags and will render as one paragraph. Wrap it in <p>…</p>, or create a code note (-t code) for plain text.
```

### 4.3 内容输出

- **默认原样输出**：text 笔记输出原始 HTML，code 笔记输出源码。
- **`--text`：只读的纯文本视图**，可以用于 `get` 和 `rev get`：
  - 只对 text 类型生效；其他文本类型原样输出；二进制类型报错。
  - 输出的头部要标明 `content: plain-text view (read-only; not the stored HTML)`，避免 agent 误以为这是原始内容，拿去做 `edit` 匹配。
  - 转换规则：
    - 块级元素（`p`、`div`、`h1`–`h6`、`li`、`tr`、`blockquote`、`pre`、`figure`、`aside`、`section`、`hr`、`br`）前后换行；
    - 标题单独成行，前后各空一行；
    - `ul > li` 前加 `- `，`ol > li` 前加 `1. ` 并递增，嵌套列表每层缩进两个空格；
    - 待办项写成 `- [ ] ` 或 `- [x] `；
    - 表格的单元格用 ` | ` 分隔，每行一行；
    - `pre` 内的空白原样保留；
    - `img` 写成 `[image: <src>]`；
    - 内部链接（`href` 以 `#root/` 开头）写成 `链接文字 [<noteId>]`，外部链接只保留文字；
    - HTML 实体解码；连续的多个空行合并成一个。
- **二进制内容**（image 类型，或者 mime 不是文本的 file 类型）永远不输出到 stdout，没有 `-o` 时也不下载。输出时提示：`content: binary (image/png), save with: trilium get <id> -o <file>`。
- `-o FILE` 把原始内容写入文件，`-o -` 表示显式写到 stdout。

### 4.4 文本输出格式

**列表行**：用于 search、ls 等命令。字段之间用制表符分隔：

```
<id>\t<type>\t<title>[\t+<子节点数>][\t(archived)]
```

- 子节点数为 0 时省略这一列；有 `#archived` 标签时标记 `(archived)`。
- 标题、分支前缀、附件标题和属性值里的制表符、换行等控制字符在文本输出中转义为 `\t`、`\n`、`\r`、`\xNN`，保证"一条记录一行"（审查 F6）。只影响显示，存储内容不变。
- 加密笔记的标题显示为 `[protected]`（ETAPI 原样返回）。
- `-l` 长格式会在 type 之后插入 mime 和修改时间：

```
<id>\t<type>\t<mime>\t<modified>\t<title>[\t+N][\t(archived)]
```

时间格式统一为 `YYYY-MM-DD HH:MM`，用本地时区，由 `utcDate*` 字段换算得到。

**写操作结果行**：每个受影响的对象输出一行，以动词开头，标题加双引号。具体格式见第 5 节各命令。

**get 的元数据头部**：类似 YAML front matter，便于 LLM 理解：

```
---
id: tText01
title: 会议纪要
type: text (text/html)
parents: etapiSandbox01, xYz12345
attrs: #status=published #todo ~seeAlso=tCodePy01
children: 3
attachments: 1
created: 2026-10-25 16:55
modified: 2026-10-25 16:57
---
<原始内容>
```

- `attrs` 只列笔记自己的属性。加 `--all-attrs` 会多一行 `inherited: #project=etapi (from etapiSandbox01) …`。
- 没有属性时省略 `attrs` 行；子节点数、附件数为 0 时照常输出 `0`（这是有用的信息）。
- `get` 传多个 ID 时，按参数顺序依次输出多个块，块之间空一行。

**空结果**：stdout 不输出任何内容，stderr 输出 `no matches`（search、ls 等），退出码为 0。

**错误**：错误写到 stderr，格式为一行错误信息，必要时再加一行 `hint:`：

```
error: Note 'abc' not found. [NOTE_NOT_FOUND]
hint: find note IDs with: trilium search <words>
```

### 4.5 `--json`

全局参数。输出 **JSON Lines**：每个实体占一行紧凑的 JSON，字段名和结构保持 ETAPI 原样。

| 命令 | 每行内容 |
|---|---|
| search、ls | ETAPI `Note`。`ls -d` 额外加一个 `"depth"` 字段 |
| get | ETAPI `Note` 加上 `"content"`（二进制时省略；用 `--text` 时是文本视图，并附加 `"contentView":"text"`）和 `"attachments"`（Attachment 数组） |
| 写操作 | 写入后的 ETAPI 实体（Note、Attribute、Branch、Attachment）；二阶段上传（二进制 `create`、`att add`）上传后重新读取，输出最终状态。内容类写操作输出 `{"noteId","bytesBefore","bytesAfter","replacements"?}` |
| 保存文件（`-o FILE`） | `{"noteId"/"attachmentId"/"revisionId","file","bytes"}`。`-o -` 和不带 `-o` 的 `att get` 输出原始字节，是有意的例外 |
| 错误 | 仍然写到 stderr，格式为 `{"error":{"status","code","message"},"hint"?}` |

### 4.6 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功（包括空结果） |
| 1 | API 错误、网络错误、操作失败（例如 edit 没有匹配到） |
| 2 | 用法错误或配置错误（参数错误、缺少环境变量） |
| 3 | 找不到（`NOTE_NOT_FOUND`、`ATTACHMENT_NOT_FOUND` 等 `*_NOT_FOUND` 错误码） |

### 4.7 属性语法（`attr` 命令和 `create -a`）

直接采用 Trilium 自己的写法，每个参数是一个操作：

| 写法 | 含义 |
|---|---|
| `#name=value` | **设置**标签：已有同名的自有标签就修改它的值（多个同名时改第一个，并在 stderr 警告），没有就新建 |
| `#name` | 设置值为空的标签 |
| `+#name=value` | **追加**一个同名标签（Trilium 允许同名多值） |
| `-#name` | 删除所有同名的自有标签 |
| `-#name=value` | 只删除值匹配的那一个 |
| `~name=<note>` | 设置关系（ETAPI 不能修改关系的值，所以实现为先删旧的再建新的）。目标支持别名 |
| `+~name=<note>` | 追加关系 |
| `-~name` / `-~name=<note>` | 删除关系 |
| `#name(inheritable)=value` | 设置可继承的标签；关系同理。可继承性和现有属性不同时，删除后重建 |

- 值就是第一个 `=` 之后的全部内容。整个值可以用双引号包起来，例如 `'#title="a b"'`，此时支持 `\"`、`\\`、`\n`、`\t`、`\r` 转义（值里有换行、制表符时格式化输出也用这种写法，保证一行），这样 `get` 和 `attr` 输出的属性可以直接粘贴回来用。
- **名字校验**：只允许 Unicode 字母、数字、`_` 和 `:`。不合法时以退出码 2 报错，**不要**让服务器静默把空格替换成 `_`（实测服务器会这么做）。
- 在 `create -a` 中，"设置"和"追加"效果相同（新笔记还没有属性）。

### 4.8 系统笔记保护

系统笔记指 `root` 和 ID 以 `_` 开头的笔记（Trilium 的隐藏子树、启动器、选项等）。按"命令 × 目标"统一规定（审查后补全）：

| 命令 | 一律拒绝 | 需要 `--force` |
|---|---|---|
| `rm`、`mv`（被移动的笔记）、`branch rm` | 是 | — |
| `put`、`append`、`edit`、`update`、`attr` | — | 笔记是 `_…` |
| `create` | — | `-p` 或 `--id` 是 `_…` |
| `mv` | — | 新父节点是 `_…` |
| `branch add`、`branch set` | 克隆 `root` | 笔记或父节点是 `_…` |
| `att add/put/update/rm` | — | 附件所属笔记是 `_…` |
| `import` | — | 父节点是 `_…` |

- 读操作不受限制。

---

## 5. 命令参考

按使用频率分三层。根帮助（`trilium help`）的草稿：

```
trilium - work with Trilium Notes from the command line

Everyday
  search <query>          Find notes (Trilium search syntax)
  ls [note]               List child notes
  get <note>...           Show a note's details and content
  create <title> [text]   Create a note (default parent: @inbox)
  append <note> [text]    Add content to the end of a note
  edit <note>             Replace exact text in a note
  put <note> [text]       Replace a note's whole content
  rm <note>...            Delete notes

Organize
  update  attr  mv  branch  att (attachments)  rev (revisions)

More
  export  import  history  undelete  info  backup

<note> can be a note ID, "root", or a shortcut: @inbox @today @2026-10-25
Long content: use -f FILE (or -f - for stdin).
Details: trilium help <command>   Search syntax: trilium help search
```

全局参数：`--json`、`-h/--help`、`--version`。

---

### 第 1 层：日常

#### 5.1 `search <query...>`

```
trilium search <query...> [-n 20] [--in <note>] [--depth eqN|ltN|gtN]
               [--sort <field>] [--desc] [--fast] [--archived] [-l]
```

| 参数 | ETAPI | 默认值 |
|---|---|---|
| `<query...>` | `search`，多个参数用空格拼接 | 必填 |
| `-n, --limit` | `limit` | 20；0 表示不限制 |
| `--in` | `ancestorNoteId` | 全库 |
| `--depth` | `ancestorDepth`（原样传递） | — |
| `--sort` | `orderBy`（原样传递，例如 `title`、`dateModified`、`#priority`） | 按相关度 |
| `--desc` | `orderDirection=desc` | asc |
| `--fast` | `fastSearch=true`（不搜索内容） | false |
| `--archived` | `includeArchivedNotes=true` | false |

- 实际请求 `limit = N+1` 条，多出来就在 stderr 提示 `20 shown; more available (use -n)`。
- 过滤掉 ID 以 `_` 开头的结果（实测 `_hidden` 本身会出现在结果里）。
- **加引号提示**：结果为空、并且查询里有未加引号的、包含 `.` 或 `-` 的比较值时（例如 `note.title=hello.py`），在 stderr 提示：
  ```
  hint: values containing '.' or '-' must be quoted, e.g. note.title="hello.py"
  ```
- `trilium help search` 输出一份精简的搜索语法速查，内容和 skill 的 `references/search.md` 一致。

#### 5.2 `ls [note]`

```
trilium ls [note=root] [-d 1] [-n 100] [-l] [--all]
```

- 列出子笔记，**按树中的顺序**（notePosition）排列，包含已归档的笔记（会标记）。
- 默认隐藏 `_` 开头的子节点，加 `--all` 显示。
- `-d N`：输出 N 层的树，每层缩进两个空格。被克隆到多个位置的笔记在每个位置都显示。
- `-n`：最多输出的行数，默认 100，超出时在 stderr 提示还有多少条。
- 实现（深度为 1 时只需两次请求）：
  1. `GET /notes/{id}`，从 `childNoteIds` 拿到子节点的顺序；
  2. `GET /notes?search=note.noteId != ""&ancestorNoteId={id}&ancestorDepth=lt{N+1}&includeArchivedNotes=true`，一次拿到所有后代。注意结果里**包含祖先笔记本身**，要过滤掉；
  3. 用每条笔记的 `childNoteIds` 递归还原树的顺序。
  4. 搜索结果里没有的子节点（例如 Trilium 隐藏子树里的笔记，`ls _hidden` 时会遇到）逐个 `GET /notes/{id}` 补齐。
- 子节点数（`+N`）不计 `_` 开头的系统子节点（系统笔记自身除外），和默认隐藏它们的规则一致。

#### 5.3 `get <note...>`

```
trilium get <note...> [-c|--content] [-m|--meta] [--text] [--all-attrs] [-o FILE]
```

- 默认输出元数据头部和内容（格式见 4.4）。
- `-c`：只输出内容；`-m`：只输出元数据。两者互斥。
- `--text`：只读的纯文本视图（见 4.3）。
- `-o FILE`：把原始内容写入文件（只能和一个 note 一起用）。输出 `saved tX to FILE (N bytes)`。
- 实现：每个 note 并发请求 `GET /notes/{id}` 和 `/attachments`，拿到类型后再请求 `/content`（二进制时跳过）；多个 note 之间也并发。
- `-c` 原样输出内容字节，不补换行；带头部的默认视图在内容末尾没有换行时补一个换行，只为终端显示整洁，不影响 `-c` 和 `-o`。
- 加密笔记（`isProtected`）只输出头部并标注 `protected: yes`；`-c`、`--text` 报错。

#### 5.4 `create [title] [content]`

```
trilium create <title> [content] [-p <note>=@inbox] [-t text] [--mime M]
               [-f FILE] [-a ATTR]... [--id ID] [--pos N]
```

- **默认值**：父笔记 `@inbox`，类型 `text`。
- `-f FILE`：读入内容，并推断类型、mime 和标题（规则见第 6.3 节）。用了 `-f` 时标题可以省略，默认用文件名。显式的 `-t`、`--mime` 和标题优先于推断结果。
- `-t code` 但没有 `--mime` 时，按标题的扩展名推断 mime（`hello.py` → `text/x-python`），推断不出来就用 `text/plain`。
- `-t search`：内容参数作为搜索语句，创建后设置 `#searchString=<内容>` 标签（实测保存的搜索靠这个标签实现，笔记内容为空）。
- `-a`：可以重复使用，语法见 4.7。
- `--id`：指定 noteId（ETAPI 原生支持），用于安全重试。**实测 Trilium 0.106.1 对已存在的 noteId 不报错，而是覆盖原笔记的标题、类型和内容**，之后删除这条笔记还会弄坏服务端内存里的属性索引（相关标签的搜索报错，直到重启）。所以 CLI 先 `GET /notes/{id}`，只有返回 `NOTE_NOT_FOUND` 才创建，已存在时以退出码 1 报错（审查 F1）。检查和创建之间仍有竞争窗口：两个客户端同时用同一个新 ID 创建时不受保护。
- `--pos`：对应 `notePosition`。
- 设置了 `TRILIUM_AGENT` 时，自动加上 `#agent=<name>`。
- 实现：
  1. `POST /create-note`。`content` 字段必须带上，空内容也要传 `""`，否则服务器返回 500；
  2. 二进制类型先用空内容创建，再 `PUT /notes/{id}/content`，类型为 octet-stream；
  3. 每个属性各调用一次 `POST /attributes`。
- 输出：
  ```
  created tA1b2C3d text "会议纪要" in e2GMUhHk0e9v (@inbox)
  ```

#### 5.5 `append <note> [content]`

```
trilium append <note> [content] [-f FILE] [--top] [--force]
```

- 把内容加到原文末尾；`--top` 改为加到开头。
- **换行规则**：对非 text 类型（code 等），如果原文非空且不以 `\n` 结尾（用 `--top` 时是新内容不以 `\n` 结尾），自动补一个 `\n`，避免两行粘在一起。text 类型（HTML）只做字节拼接，不补任何东西。帮助里要写明这条规则。
- 二进制笔记拒绝执行。
- 实现：先读取 `blobId` 和内容，在本地拼接，写入前再 `GET /notes/{id}` 比对 `blobId`。如果发现被别人改过，就基于新内容重新计算，最多重试 3 次；仍然失败时报错 `note changed concurrently, please retry`。
  - 实施补充：两个写入者可能同时通过 `blobId` 检查然后互相覆盖。所以写入后再读一次内容：与写入的字节完全相同即成功；不同说明有别人紧接着写入，`append` 检查自己的片段是否还在，不在就随机退避后重做（计入 3 次）。
  - **局限（审查 F4）**：ETAPI 没有条件写入。两个写入者都通过检查、都读回自己的结果之后，后写的一方仍会覆盖前者，双方都无法察觉。所以 CLI 只承诺"尽力检测冲突"，文档要求同一条笔记只由一个写入者修改；共享日志改为每条记录一条笔记。
  - 一旦发出过写入，后续的任何错误都不能再说"nothing was changed"：报告 `… the result is unknown` 或 `your change may be missing`，并提示用 `get -c` 核对。
- 输出：
  ```
  appended tA1b2C3d +36 bytes: 2431 → 2467 bytes
  ```

#### 5.6 `edit <note>`

```
trilium edit <note> --old S --new S [--old S --new S]... [--all] [--force]
trilium edit <note> --old-file F --new-file F [--all]
```

- 在**原始内容**上做精确的字节匹配和替换，语义和 agent 熟悉的 str_replace 工具一致。
- `--old` 和 `--new` 可以成对重复，按顺序依次应用。**全部成功才写入**，任何一对失败都不写。
- 每个 `--old` 默认必须唯一匹配：
  - 0 处匹配：报错，退出码 1，并提示：
    ```
    hint: text must match exactly; text notes are raw HTML. View it with: trilium get -c <id>
    ```
  - 多处匹配：报错并给出次数，确实要全部替换就加 `--all`。
- `--new ""` 表示删除这段文字。片段较长或含特殊字符时，用 `--old-file/--new-file`。
- 二进制笔记拒绝执行；并发处理和 `append` 相同，但写入后发现内容被别人改了时**不重做**（替换可能已经不适用于新内容），而是报告结果未知（退出码 1）。
- 写入后只核对最终结果，不检查每一步的中间结果（`--old A --new B --old B --new C` 中的 `B` 本来就会被消耗；审查 F3）。
- 输出：
  ```
  edited tA1b2C3d 2 replacements: 2431 → 2467 bytes
  ```

#### 5.7 `put <note> [content]`

```
trilium put <note> [content] [-f FILE] [--force]
```

- 整体替换内容，对应 `PUT /notes/{id}/content`。不自动创建修订快照，需要时用 `rev add`。
- image 和 file 笔记可以用 `-f` 上传新的二进制内容。
- 输出：
  ```
  updated tA1b2C3d content: 2431 → 2467 bytes
  ```
  "替换前"的字节数取自写入前读到的内容长度。二进制内容不读取，替换前显示为 `?`。

#### 5.8 `rm <note...>`

```
trilium rm <note...> [-r]
```

- 对应 `DELETE /notes/{id}`。注意 ETAPI 会删除笔记**所有位置**的分支和**整棵子树**。
- 笔记有子节点而没加 `-r` 时报错：`has 3 children; use -r to delete them too`。
- 笔记在多个位置时，结果行会提示 `(was in 2 places; to remove one placement use: trilium branch rm)`。
- 拒绝删除系统笔记（4.8）。
- 输出：
  ```
  deleted tA1b2C3d "标题" (+3 children; undo: trilium undelete tA1b2C3d)
  ```

---

### 第 2 层：组织整理

#### 5.9 `update <note>`

```
trilium update <note> [--title T] [--type TYPE] [--mime M] [--force]
```

- 对应 `PATCH /notes/{id}`，只发送用户指定的字段。
- 输出：
  ```
  updated tA1b2C3d title: "旧" → "新", mime: text/plain → text/x-python
  ```

#### 5.10 `attr <note> [ops...]`

```
trilium attr <note>                     # 列出自有属性
trilium attr <note> --all               # 同时列出继承的属性
trilium attr <note> '#status=done' '-#todo' '~seeAlso=abc'
```

- 列出时每行一条：`<attributeId>\t<属性写法>`；继承的属性在末尾加 `\t(from <ownerId>)`（继承来的属性必然可继承，所以不再写 `(inheritable)`）。
- `attr` 自己解析参数，`-#todo`、`-~rel` 这类以 `-` 开头的操作不会被误认为选项；也支持用 `--` 结束选项。
- 实测 ETAPI 返回的 `attributes` **包含继承来的属性**（`noteId` 是拥有者），所以"自有属性"按 `noteId` 过滤。
- 操作语法见 4.7。所有操作先在本地解析和校验，关系目标先解析并确认存在，全部通过才开始执行。实现时只需 `GET /notes/{id}` 一次，然后按需调用 `POST`、`PATCH`、`DELETE /attributes`。
- 执行按顺序进行，不是事务：中途失败时，错误信息说明前面几个操作已经生效。
- 修改关系或改变可继承性时**先建新属性、成功后再删旧属性**（审查 F2），新建失败时旧值保持不变；删除旧值失败时报告"两个都在"并给出清理命令。
- 输出每个操作一行：
  ```
  set #status=done on tA1b2C3d
  added #tag=b on tA1b2C3d
  removed #todo from tA1b2C3d
  ```

#### 5.11 `mv <note> <new-parent>`

```
trilium mv <note> <new-parent> [--from <old-parent>] [--pos N]
```

- ETAPI 没有"移动"操作，实现为先 `POST /branches` 建新分支，成功后再 `DELETE` 旧分支（先建后删，中途失败也不会丢笔记）。
- 笔记只有一个父节点时自动确定从哪里移走；有多个时必须用 `--from`。
- **环检测**：新父节点是笔记本身，或者是它的后代时，拒绝执行。检测方法是一次搜索：`search=note.noteId="<newParent>"&ancestorNoteId=<note>`，结果非空就说明会成环。这是客户端必须补上的检查，实测服务端不拦截。
- 输出：
  ```
  moved tA1b2C3d "标题": e2GMUhHk0e9v → xYz12345
  ```

#### 5.12 `branch`

分支表示"笔记出现在树中的某个位置"，克隆就是给笔记再加一个分支。

```
trilium branch ls <note>                                         # 列出它所在的所有位置
trilium branch add <note> <parent> [--prefix P] [--pos N]        # 克隆到另一个位置
trilium branch set <note> <parent> [--prefix P] [--pos N] [--expanded=true|false]
trilium branch rm <note> <parent> [--force]                      # 从一个位置移除
```

- 通过 `<note> <parent>` 来定位分支。查找时先尝试 `GET /branches/{parent}_{note}`（实测分支 ID 是确定性的），失败再遍历笔记的 `parentBranchIds`。也可以直接传 branchId 作为唯一参数。
- `branch ls` 每行一条：`<branchId>\t<parentId>\t<pos>\t<prefix>`。
- `branch add` 要做环检测（同 `mv`）。如果分支已经存在，ETAPI 会返回 200 并更新它，CLI 的输出要说明这是更新而不是新建。
- `branch set` 修改了 `--pos` 后，自动调用 `POST /refresh-note-ordering/{parent}`。
- `branch rm` 删的是最后一个分支时，ETAPI 会连笔记一起删除。这种情况下拒绝执行，并提示用 `rm`；确实要删时加 `--force`。
- 输出：
  ```
  cloned tA1b2C3d into xYz12345 (branch xYz12345_tA1b2C3d)
  ```

#### 5.13 `att`（附件）

```
trilium att ls <note>
trilium att get <attachmentId> [-o FILE]
trilium att add <note> [-f FILE | content] [--title T] [--role file|image] [--mime M] [--pos N]
trilium att put <attachmentId> [-f FILE | content]
trilium att update <attachmentId> [--title T] [--role R] [--mime M] [--pos N]
trilium att rm <attachmentId>
```

- `att ls` 每行一条：`<id>\t<role>\t<mime>\t<size>\t<title>`，其中 size 来自 `contentLength`。
- `att add -f photo.png`：标题默认用文件名，mime 按扩展名推断，`image/*` 的 role 推断为 `image`，其他为 `file`。二进制内容先创建附件再 `PUT` 内容（ETAPI 的要求）。
- role 为 image 时，额外输出一行嵌入用的 HTML，方便 agent 直接写进 text 笔记：
  ```
  added attachment Kq9xYz1a to tA1b2C3d: "red.png" image/png 80 bytes
  embed: <img src="api/attachments/Kq9xYz1a/image/red.png">
  ```
- `att get`：文本内容直接输出；二进制内容必须用 `-o`。
- 不支持把附件挂在修订版本上（规范里写了支持，但实测服务器会拒绝）。

#### 5.14 `rev`（修订版本）

```
trilium rev ls <note>
trilium rev get <revisionId> [--text] [-o FILE]
trilium rev add <note>
```

- `rev ls` 每行一条：`<revisionId>\t<created>\t<size>\t<title>`，按时间倒序排列。
- `rev add` 调用 `POST /notes/{id}/revision`。这个接口返回 204，不带 ID，所以创建后要再 `rev ls` 一次，取最新那条的 ID 输出：`created revision R1a2b3c4 of tA1b2C3d`。
- `rev get` 的输出格式和 `get` 一致，头部字段换成修订版本的字段。

---

### 第 3 层：批量与管理

#### 5.15 `history`

```
trilium history [--in <note>] [-n 20]
```

- 对应 `GET /notes/history?ancestorNoteId=`。ETAPI 会返回全部记录，CLI 在客户端截取前 N 条，按时间倒序。
- 每行一条：`<date>\t<noteId>\t<title>[\tdeleted][\t(can undelete)]`。（原设计写作 `(undeletable)`，英文里容易读成"不可删除"，实施时改为 `(can undelete)`。）
- 如果原始数据里有完全相同的重复记录（实测出现过），去重后再输出。

#### 5.16 `export <note>`

```
trilium export <note> [-o FILE] [--format html|markdown]
```

- 对应 `GET /notes/{id}/export`。格式默认 `html`，和 ETAPI 的默认值一致。
- 输出文件默认是当前目录下的 `<标题>.zip`（标题会去掉不能用作文件名的字符）。
- 输出：`exported tA1b2C3d to ./标题.zip (8.9 KB)`。

#### 5.17 `import <parent> <file>`

```
trilium import <parent> <file.zip | file.md>
```

- `.zip`：原样上传到 `POST /notes/{id}/import`，内容类型为 octet-stream。
- `.md`：这是"把 Markdown 转换成 Trilium 富文本"的**唯一途径**，转换由 **Trilium 自己的导入器**完成，CLI 不做任何格式转换。实现方式：CLI 在内存中把这个文件和一份 `!!!meta.json` 打包成 zip，meta 中声明 `type: "text"`、`format: "markdown"`、标题为文件名去掉扩展名，然后上传。
  - 实测：单独把 .md 打包上传时，Trilium 会把它当成普通文件导入，不做转换；必须带上 meta。
  - meta 格式参考 6.4。帮助和 skill 里要明确说明转换是 Trilium 做的。
- 设置了 `TRILIUM_AGENT` 时，**不**给导入的笔记打标签（身份标记只在 `create` 时添加）。
- 输出：`imported tNew1234 "标题" into xYz12345`。

#### 5.18 其他

| 命令 | ETAPI | 输出 |
|---|---|---|
| `undelete <noteId>` | `POST /notes/{id}/undelete` | `restored tA1b2C3d "标题"` |
| `info` | `GET /app-info` + `GET /metrics?format=json` | 见下 |
| `backup [name=now]` | `PUT /backup/{name}` | `backup created on server: backup-now.db` |
| `version` / `--version` | — | `trilium 1.0.0 (commit abc1234)` |

`info` 同时用来检查配置是否正确，是 skill 里规定的第一步：

```
server:  https://notes.example.com (Trilium 0.106.1)
auth:    ok
notes:   400 active, 40 deleted
agent:   claude-1
```

`agent` 来自 `TRILIUM_AGENT`，未设置时显示 `(not set)`。

---

## 6. Trilium 内容约定（实测）

### 6.1 Trilium 富文本的 HTML 写法

以下 HTML 来自 Trilium 0.106.1 自己的 Markdown 导入器的输出，代表 Trilium 的规范写法。agent 往 text 笔记写 HTML 时应该照此书写。这些内容会写进 skill 的 `references/html.md`。

| 元素 | HTML |
|---|---|
| 标题 | `<h2>`–`<h6>`。Markdown 的 `#` 会被映射成 `<h2>`，因为 Trilium 把 h1 留给笔记标题 |
| 段落、强调 | `<p>`、`<strong>`、`<em>`、`<code>`、`<del>` |
| 链接 | `<a href="https://…">text</a>` |
| 列表 | `<ul><li>…<ul><li>嵌套</li></ul></li></ul>`、`<ol>` |
| 待办 | `<ul class="todo-list"><li><label class="todo-list__label"><input type="checkbox" disabled="disabled"><span class="todo-list__label__description">任务</span></label></li></ul>`。已完成的任务在 input 上加 `checked="checked"` |
| 代码块 | `<pre><code class="language-text-x-python">…</code></pre>`，未指定语言时为 `language-text-x-trilium-auto` |
| 表格 | `<figure class="table"><table><thead>…</thead><tbody>…</tbody></table></figure>` |
| 引用 | `<blockquote><p>…</p></blockquote>` |
| 公式 | `<span class="math-tex">\(x^2\)</span>`（实测导入单独一行的 `$x^2$` 时不包 `<p>`） |
| 提示框 | `<aside class="admonition note"><p>…</p></aside>`，类型有 note、tip、important、caution、warning |
| 分隔线 | `<hr>` |
| 附件图片 | `<img src="api/attachments/<attachmentId>/image/<文件名>">`（实测） |
| 内部链接 | `<a href="#root/<noteId>">文字</a>`（实测：Trilium 导入 `[文字](#root/<noteId>)` 生成的就是这种写法，链接文字由作者决定）。编辑器里的"引用链接"写作 `<a class="reference-link" href="#root/<noteId>">标题</a>`，显示时用目标笔记的标题。两种写法服务器都原样保存 |

**代码块的 class 规则**：`"language-" + mime.toLowerCase()`，再把所有 `[\W_]+` 替换成 `-`（Trilium 源码中的 `normalizeMimeTypeForCKEditor`）。例如 `text/x-python` → `language-text-x-python`。

### 6.2 笔记类型

- `create-note` 实际接受的类型：text、code、render、file、image、search、relationMap、book、noteMap、mermaid、canvas、webView、launcher、doc、contentWidget、mindMap、spreadsheet、llmChat。CLI 原样传递给服务器，由服务器校验。
- 各类型的默认 mime（实测）：
  - text → `text/html`
  - code → `text/plain`
  - mermaid → `text/vnd.mermaid`
  - canvas、mindMap、relationMap → `application/json`
  - search、book、render、webView、noteMap → 空
- 保存的搜索靠 `#searchString` 标签实现，内容为空。

### 6.3 根据文件推断类型（`create -f`）

mime 以 Trilium 源码 `packages/commons/src/lib/mime_type.ts` 为准。在 `internal/content/mimes.go` 中维护扩展名到 mime 的映射表。

| 扩展名 | 类型 | mime |
|---|---|---|
| `.html` `.htm` | text | text/html |
| `.md` `.markdown` | code | text/x-markdown（Trilium 的 `getMimeTypeFromFileName()` 给出的是 `text/x-gfm`，但它默认不在代码笔记的下拉列表里，所以保留 text/x-markdown） |
| `.mmd` `.mermaid` | mermaid | text/vnd.mermaid |
| `.txt` `.log` | code | text/plain |
| `.py` | code | text/x-python |
| `.js` / `.mjs` | code | text/javascript |
| `.ts` / `.tsx` / `.jsx` | code | application/typescript / text/typescript-jsx / text/jsx |
| `.json` | code | application/json |
| `.yaml` `.yml` / `.toml` / `.xml` | code | text/x-yaml / text/x-toml / text/xml |
| `.css` / `.sh` `.bash` / `.go` / `.rs` | code | text/css / text/x-sh / text/x-go / text/x-rustsrc |
| `.java` / `.c` `.h` / `.cpp` `.cc` `.hpp` / `.cs` | code | text/x-java / text/x-csrc / text/x-c++src / text/x-csharp |
| `.rb` / `.php` / `.sql` / `.kt` / `.swift` / `.lua` / `.r` | code | text/x-ruby / text/x-php / text/x-sql / text/x-kotlin / text/x-swift / text/x-lua / text/x-rsrc |
| `.diff` `.patch` / `.tex` / `.vue` / `.scala` / `.hs` / `.ex` `.exs` / `.pl` / `.ps1` / `.bat` / `.proto` / `.rst` / `.dart` / `.jl` / `.groovy` / `.nix` / `.erl` / `.clj` / `.ml` / `.ini` `.properties` / `Dockerfile` | code | 按 mime_type.ts 中的对应值 |
| `.png` `.jpg` `.jpeg` `.gif` `.webp` `.svg` `.bmp` `.ico` `.avif` | image | 用 `mime.TypeByExtension` 推断 |
| 其他 | file | 先用 `mime.TypeByExtension`，再用 `http.DetectContentType`，都不行就用 `application/octet-stream` |

实施说明：扩展名到 mime 的映射由 `scripts/gen_mimes.py` 从 `mime_type.ts` 生成（`internal/content/mimes_gen.go`），并复刻 `getMimeTypeFromFileName()` 的查找规则（`LANGUAGES_BY_EXTENSION` → 别名优先 → `mdLanguageCode`）。按这个规则 `.ini` 得到 `text/x-toml`。Trilium 不认识的 `.txt` `.log` `.patch` `.proto` `.jl` `.ml` 在 `mimes.go` 里单独补充。

**判断内容是否为二进制**（决定是否输出到终端）：

- image 类型 → 二进制；
- file 类型且 mime 不以 `text/` 开头，也不是 json、xml、javascript → 二进制；
- 其他类型 → 文本。下载后如果发现不是合法的 UTF-8，也按二进制处理。

### 6.4 Markdown 导入用的 `!!!meta.json`（实测可用）

```json
{"formatVersion":2,"appVersion":"<服务器版本>","files":[{"isClone":false,"noteId":"<随机12位>","notePath":["<同noteId>"],"title":"<标题>","notePosition":10,"prefix":null,"isExpanded":false,"type":"text","mime":"text/html","attributes":[],"format":"markdown","dataFileName":"<文件名>.md","attachments":[]}]}
```

- `appVersion` 取自 `/app-info`。
- 导入后 noteId 会被服务器重新生成，CLI 以响应中的 `note.noteId` 为准。

---

## 7. ETAPI 行为与服务端怪癖（客户端必须处理）

| # | 现象（实测） | 客户端的处理 |
|---|---|---|
| 1 | 响应超过约 1KB 时**强制 gzip**，即使请求头是 `Accept-Encoding: identity` 也一样（Express compression 中间件的行为） | 使用 Go 默认 Transport，并且**不要手动设置** `Accept-Encoding`：Transport 会自动请求和解压 gzip。手动设置了就必须自己解压 |
| 2 | Basic 认证的用户名必须是 `etapi`（规范说"可以随意填写"，与实际不符） | 只使用 `Authorization: <token>` 头 |
| 3 | 搜索的比较值中含 `.` 或 `-` 时，必须加引号，否则静默返回空结果 | `search` 的提示（5.1）；skill 中重点说明 |
| 4 | `POST /branches` 不检查环 | `mv`、`branch add` 在客户端做环检测 |
| 5 | 附件的 `ownerId` 不能是 revisionId | 不支持 |
| 6 | 用 `application/json` 类型 PUT 内容会返回 500 | 只用 `text/plain` 或 `application/octet-stream` |
| 7 | `create-note` 缺少 `content` 字段时返回 500 | 始终带上 `content`（可以是 `""`） |
| 8 | 属性名中的空格会被静默替换成 `_` | 客户端校验名字（4.7） |
| 9 | 搜索结果中会出现 `_hidden` 和 root | `search` 过滤掉 `_*` |
| 10 | `ancestorNoteId` 搜索的结果包含祖先笔记本身 | `ls` 过滤掉 |
| 11 | 分支 ID 是确定性的 `<parentId>_<childId>` | 用来快速定位分支，失败时退回遍历查找 |
| 12 | 日历接口在没有日历根笔记时会自动在 root 下创建 "Calendar" | 在帮助和 skill 中说明；E2E 测试要清理 |
| 13 | 服务器时区可能是 UTC，返回的 `dateCreated` 带的是服务器本地时区 | 显示时一律用 `utc*` 字段换算成本地时间 |
| 14 | `POST /notes/{id}/revision` 返回 204，不带 ID | `rev add` 创建后再查一次 |
| 15 | 修订版本和附件的 `contentLength` 按**字符**计算，不是字节（实测 `附件文本内容` 返回 6） | 内容类写操作的字节数由 CLI 自己按 `len([]byte)` 计算；`att ls`、`rev ls` 原样显示 `contentLength`，在帮助和 conventions.md 中写明"文本内容按字符计" |
| 16 | 修改分支时 `isExpanded` 可以改（规范说只能改 prefix 和 notePosition） | `branch set --expanded` 可用 |
| 17 | `GET /notes/history` 的 `current_isDeleted`、`current_isProtected` 是 `0`/`1` 整数，不是布尔值 | 客户端用兼容两种写法的 `etapi.Bool` 解码；`--json` 输出统一为布尔值 |
| 18 | 笔记的 `attributes` 包含继承来的属性（`noteId` 为拥有者） | `get`、`attr` 按 `noteId` 区分自有和继承；`attr` 的修改只作用于自有属性 |
| 19 | `create-note` 遇到已存在的 `noteId` 不报错，直接覆盖；之后删除该笔记会损坏服务端属性索引（搜索相关标签报 `GENERIC`，重启后恢复） | `create --id` 先检查是否存在（5.4）。fake 服务器按实测行为模拟覆盖；`e2e/contract_test.go` 逐条核对 fake 的其他假设 |

---

## 8. 架构

### 8.1 技术选型

- **Go 1.27**（go.mod 中写 `go 1.27`）。鼓励使用新的语言特性和标准库：range-over-func 迭代器、`for range n`、`sync.WaitGroup.Go`、`slices`/`maps`、`t.Context()` 等。
- **依赖尽量少**：
  - `github.com/spf13/cobra`：子命令；允许 flag 写在位置参数后面；**换成精简的帮助模板**。
  - `golang.org/x/net/html`：只用于 `--text` 视图。
  - 其余全部用标准库：net/http、encoding/json、archive/zip、mime。
- **发布**：单个静态二进制，用 goreleaser 构建 darwin、linux、windows 的 amd64 和 arm64 版本；另外支持 `go install`。

### 8.2 包结构

```
cmd/trilium/main.go           入口：组装依赖，执行，把错误映射为退出码
internal/etapi/               ETAPI 客户端，和接口一一对应，不含业务逻辑
  client.go                   基础 URL、认证、请求、错误解码、GET 的重试
  types.go                    Note、Branch、Attribute、Attachment、Revision、RecentChange、AppInfo、Metrics、Error
  notes.go branches.go attributes.go attachments.go revisions.go calendar.go system.go
internal/config/              读取和校验环境变量，规范化 URL
internal/ref/                 解析笔记引用和别名（依赖 etapi 和 clock）
internal/attrspec/            属性语法的解析和格式化（纯函数）
internal/content/             内容来源（参数、-f、-f -）、mime 映射表、类型推断、二进制判断、纯文本视图
internal/output/              列表行、结果行、get 头部、JSON Lines、错误和提示的格式化（纯函数）
internal/cli/                 cobra 命令：root.go 以及每个命令一个文件；精简的帮助模板
skills/trilium/               面向 agent 的 skill（见第 9 节）
docs/                         设计文档、实施计划
README.md, README.zh-CN.md
```

### 8.3 HTTP 客户端

- 超时 30 秒。
- 只对 GET 请求在网络错误、502、503、504 时重试 2 次，用指数退避。写请求不重试。
- 错误解码：
  - 响应是 JSON `{status, code, message}` → 转成 `*etapi.Error`；
  - 不是 JSON（例如 nginx 返回的 502 页面）→ `HTTP 502 from server`。
- 错误码到提示的映射集中维护在 `internal/output`：

| 错误码或情况 | 提示 |
|---|---|
| `NOT_AUTHENTICATED`（401） | check TRILIUM_ETAPI_TOKEN; tokens are created in Trilium under Options → ETAPI |
| `*_NOT_FOUND` | find note IDs with: trilium search <words>（退出码 3） |
| `NOTE_IS_PROTECTED` | protected notes can't be read or changed through ETAPI |
| 429 | too many failed attempts; wait a few minutes |
| 网络错误 | could not reach TRILIUM_URL (<url>) |

- `User-Agent: trilium-cli/<version>`。

### 8.4 运行时依赖注入

`cli.Runtime` 持有 etapi.Client、stdout、stderr、env、clock、JSON 模式开关。命令代码不直接访问 `os.*` 和 `time.Now()`，便于测试。

---

## 9. Skill 设计

skill 和 CLI 同等重要，甚至更重要：agent 能否用好这个工具，主要取决于 skill。skill 放在仓库的 `skills/trilium/` 下，由用户自行复制到自己 agent 的 skills 目录（README 中说明）。

### 9.1 遵循的最佳实践

- **渐进披露**：
  1. frontmatter 的 description 决定什么时候触发；
  2. `SKILL.md` 只放每次都需要的内容（目标约 150–200 行，上限 500 行）；
  3. 细节放在 `references/` 下按需读取，引用只有一层，不嵌套。
- **description 写清触发条件**：说明它能做什么，以及什么时候该用。
- 指令用祈使句，内容精确。所有示例都可以直接复制运行，并在实施阶段**先跑通再写进去**。
- 参考文档超过 100 行时，开头加目录。
- 用英文撰写（面向所有 agent），和 CLI 的帮助风格一致。

### 9.2 结构与内容

```
skills/trilium/
├── SKILL.md
└── references/
    ├── conventions.md  输出格式、错误、退出码和各命令的易错行为（参数见 --help）
    ├── search.md       搜索语法速查：全文、精确短语、标签和关系条件、note.* 属性、
    │                   父子和祖先条件、and/or/not、正则、排序和限制；
    │                   重点说明加引号的规则；每类语法配一个例子
    ├── html.md         Trilium 富文本的 HTML 写法（第 6.1 节的内容和完整例子）、
    │                   代码块 class 规则、附件图片的嵌入方法、什么时候改用 code 笔记或导入 .md
    └── workflows.md    较长的端到端场景：整理一棵笔记树、上传附件并嵌入、
                        批量打标签、导出导入、找回误删的笔记
```

**SKILL.md 的大纲**

```markdown
---
name: trilium
description: Read, search, create, edit, and organize notes in a Trilium Notes
  knowledge base with the `trilium` CLI. Use when the user mentions Trilium, or
  asks to find, save, update, or organize notes, journals, or knowledge kept in Trilium.
---

# Trilium

## Before you start        → 运行 `trilium info`；报错时的处理方式
## Core concepts           → note / branch(位置、克隆) / attribute(#label ~relation) /
                             attachment，各一句话；<note> 的写法和 @ 别名
## Content is stored as-is → text 笔记是 HTML（指向 references/html.md）；
                             code 笔记是纯文本；想写 Markdown 有两条路：
                             建 .md 的 code 笔记，或用 import 让 Trilium 转换
## Everyday commands       → 第 1 层命令，各一行示例
## Common patterns         → 见下
## Gotchas                 → 见下
## References              → 什么情况下读哪个文件
```

**常用模式**（每个配一段可以直接运行的示例）：

1. **先找后读**：`search` 后 `get id1 id2`（一次读多条）。
2. **快速记录**：`append @today -f note.html`；`create "标题" -f body.html`（默认放进收件箱）。
3. **局部修改长笔记**：`get -c` 看原文，再用 `edit --old --new`；不要整篇 `put`。
4. **写长内容**：先写到本地文件，再用 `-f` 读入。
5. **用标签管理状态**：`attr X '#status=done'`；`search '#status=todo' --in <project>`。
6. **整理**：`ls -d 2` 看结构，`mv` 移动，`branch add` 克隆到另一处。
7. **附件**：`att add X -f chart.png`，再把输出的 `embed:` 那一行 `append` 进笔记。
8. **从 Markdown 生成富文本**：`import <parent> doc.md`。
9. **多 agent 协作**：查自己写过的笔记 `search '#agent=<me>'`；用 `history --in <project>` 看最近的改动。
10. **读长笔记省 token**：`get --text` 只读文字；要修改时再用 `get -c` 拿原始 HTML。

**必须知道的坑**：

- 搜索值里有 `.` 或 `-` 要加引号。
- `rm` 会删掉笔记的整棵子树和它在所有位置的分支；只想从某个位置移除用 `branch rm`。误删可以用 `undelete` 恢复。
- `--text` 的输出不能用作 `edit --old` 的匹配文本。
- 日历别名会自动创建日笔记。
- 不要对二进制笔记使用 `get -c`，要用 `-o`。

### 9.3 skill 的验证

实施阶段要在真实服务器的沙盒里，把 SKILL.md 和 references 中的每个示例都跑一遍。示例里的 ID 用占位符写成 `<id>` 的形式，验证时代入真实 ID。

---

## 10. README 设计

- `README.md` 是英文主文档，`README.zh-CN.md` 是中文版。两份文档顶部互相链接（`English | 简体中文`），内容保持同步。
- 面向用户写，从"能做什么"讲到"怎么用"，技术细节放到最后。

**大纲**

1. 标题 + 一句话介绍：*Work with your Trilium notes from the terminal — built for AI agents, pleasant for humans.*
2. 它能做什么：3–5 条卖点，包括为 AI agent 优化（省 token、输出稳定）、精确读写、覆盖完整的 ETAPI 功能、单个二进制文件
3. 安装：下载预编译的二进制 / `go install`
4. 设置（3 步）：在 Trilium 中创建 token（选项 → ETAPI）→ 设置两个环境变量 → 运行 `trilium info` 验证
5. 快速上手：5 个最常用的命令及输出示例
6. 日常用法：搜索、阅读、记录、修改、整理、附件、日记，每类一个小例子
7. 配合 AI agent 使用：
   - 复制 `skills/trilium` 到 `~/.claude/skills/`（Claude Code）或项目的 `.claude/skills/`；
   - 其他 agent 可以把 SKILL.md 的内容加入系统提示词；
   - 用 `TRILIUM_AGENT` 区分不同的 agent
8. 笔记快捷方式（@inbox、@today …）
9. 命令一览表（三层）
10. 常见问题和排错（连不上、401、搜不到、关于 HTML 内容）
11. 兼容性（实测版本）、参与贡献、许可证

---

## 11. 以后可以考虑（v1 不做）

- 配置文件和多服务器 profile
- `put --if-blob <blobId>` 强一致的条件写入（需要 ETAPI 支持）
- Homebrew tap
- 在 CI 中用 Docker 运行 Trilium 来跑 E2E 测试
