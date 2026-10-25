# trilium CLI 实施计划

> 面向：负责实施的 agent
> 依据：[设计文档](design.md)（以下简称"设计"，引用写作"设计 §x.y"）
> 原则：**设计是规格**。实施中发现设计有问题或无法实现时，先停下来提出，确认后同步修改设计文档，不要自行偏离。

---

## 0. 开工前

### 0.1 需要向项目负责人确认的事项

| 事项 | 用途 | 未确认时 |
|---|---|---|
| Go module 路径（GitHub 的 owner/仓库名） | go.mod、`go install` 的说明、README | 先用 `github.com/OWNER/trilium-cli` 作为占位，发布前统一替换 |
| 开源许可证 | LICENSE、README | 建议 MIT。确认之前先不加 LICENSE 文件 |

### 0.2 环境

- Go 1.27 已安装（本机 `go1.27.1 darwin/arm64`）。
- 测试服务器：通过 `TRILIUM_URL` 和 `TRILIUM_ETAPI_TOKEN` 提供，向项目负责人索取，**不要把 token 写进仓库**。
- 服务器上已经有用户数据。E2E 测试只能在自己创建的沙盒笔记下进行（见 §3.3）。

### 0.3 工作约定（必须遵守）

1. **不改写内容**：不做 Markdown 和 HTML 之间的转换。唯一的例外是 `import x.md`，那是把 .md 打包上传，由 Trilium 自己转换（设计 §5.17）。
2. **永远不调用** `/auth/logout`（会让 token 失效），也不实现 `/auth/login`。
3. 输出格式**严格按照设计 §4.4、§4.5 和第 5 节**，用 golden 文件锁定，修改输出必须同时更新 golden 文件。
4. 用户看到的文字（帮助、错误、提示、README、skill）用英文；README 另有中文版。文风要友好清晰，避免黑话。
5. 每个阶段结束时：`go vet ./...`、`go test ./...` 全部通过，`golangci-lint run` 没有警告，然后提交一次（或若干次小提交）。提交信息用英文祈使句。
6. 新增依赖之前先确认是否真的需要。目前允许的依赖只有 cobra 和 golang.org/x/net。
7. 不在终端输出 ANSI 颜色和交互式提示。

---

## 1. 阶段与任务

每个阶段列出要做的任务和**验收标准**，验收全部通过后再进入下一阶段。

### P0 项目骨架

- `git init`；添加 `.gitignore`（`/dist`、`/trilium`、`*.zip`、`.env`）。
- `go mod init <module>`；go.mod 中写 `go 1.27`。
- 按设计 §8.2 创建目录结构，`cmd/trilium/main.go` 先能运行 `trilium --version`。
- `Makefile`，包含以下目标：
  - `build`：用 ldflags 注入 `version`、`commit`；
  - `test`；
  - `lint`；
  - `e2e`：`TRILIUM_E2E=1 go test ./e2e/...`；
  - `tokens`：见 §4。
- `.golangci.yml`：开启 govet、staticcheck、errcheck、revive 的基础规则即可。

**验收**：`make build && ./trilium --version` 输出 `trilium dev (commit …)`；`make lint test` 通过。

### P1 ETAPI 客户端（`internal/etapi`）

- `types.go`：按服务器自带的 OpenAPI 规范（`GET <TRILIUM_URL>/etapi/etapi.openapi.yaml`，gzip 编码）定义类型，字段名保持一致，可空字段用指针或 `omitempty`。
- `client.go`：
  - `New(baseURL, token, opts...)`；
  - 认证头为 `Authorization: <token>`；
  - **不手动设置 `Accept-Encoding`**（设计 §7 #1）；
  - 超时 30 秒；GET 请求的重试规则见设计 §8.3；
  - 错误解码成 `*etapi.Error{Status, Code, Message}`，非 JSON 响应也要处理；
  - User-Agent 为 `trilium-cli/<version>`。
- 每个接口一个方法，**一一对应，不含业务逻辑**：
  - notes：`CreateNote`、`GetNote`、`PatchNote`、`DeleteNote`、`GetContent`、`PutContent(ctx, id, body io.Reader, binary bool)`、`Search(params)`、`Export`、`Import`、`CreateRevision`、`ListRevisions`、`ListAttachments`、`Undelete`、`History`；
  - `GetRevision`、`GetRevisionContent`；
  - branches：`CreateBranch`（返回值中区分 200 和 201）、`GetBranch`、`PatchBranch`、`DeleteBranch`、`RefreshNoteOrdering`；
  - attachments：`CreateAttachment`、`GetAttachment`、`PatchAttachment`、`DeleteAttachment`、`GetAttachmentContent`、`PutAttachmentContent`；
  - attributes：`CreateAttribute`、`GetAttribute`、`PatchAttribute`、`DeleteAttribute`；
  - calendar：`Inbox`、`Day`、`Week`、`Month`、`Year`、`WeekFirstDay`；
  - system：`AppInfo`、`Metrics`（json 格式）、`Backup`。
- `CreateNote` 始终发送 `content` 字段（设计 §7 #7）。
- `PutContent` 只使用 `text/plain` 或 `application/octet-stream`（设计 §7 #6）。

**单元测试**（用 httptest）：

- 每个方法的路径、方法、请求头、请求体是否正确；
- **服务器强制返回 gzip 时能正确解压**：测试服务器忽略 Accept-Encoding，直接返回 gzip 内容；
- 错误解码：JSON 错误、HTML 502、空响应体；
- GET 请求在 503 后重试成功；POST 请求不重试。

**验收**：测试覆盖率不低于 80%；手动连接真实服务器时 `AppInfo` 返回正常。

### P2 基础模块

#### `internal/config`

- 读取三个环境变量。URL 规范化：去掉末尾的 `/` 和 `/etapi`，再拼上 `/etapi`。
- 缺少变量时返回带提示的错误，文案严格按设计 §3。
- 单元测试覆盖：带不带末尾斜杠、是否带 `/etapi`、缺失、URL 格式非法。

#### `internal/attrspec`

- 按设计 §4.7 解析：`Op{Kind: Set|Add|Remove, Type: label|relation, Name, Value, HasValue, Inheritable}`。
- 能把 Attribute 格式化成同样的写法（值含空白或引号时加双引号并转义），并保证**解析 → 格式化 → 再解析**结果一致。
- 名字校验规则：`^[\p{L}\p{N}_:]+$`。
- 用表驱动测试，至少 25 个用例：中文名字、带空格的值、带引号的值、值为空、`(inheritable)`、非法名字、只有 `#`、`-~rel=<id>` 等。

#### `internal/ref`

- `Parse(s) (Ref, error)`：区分 noteId、`root`、各种别名，对日期做合法性校验（`@2026-13-45` 以退出码 2 报错）。
- `Resolve(ctx, client, clock) (noteId string, alias string, err error)`：
  - 别名通过 calendar/inbox 接口解析；
  - `@today`、`@yesterday`、`@tomorrow` 按本地时区计算；
  - `@2026-W43` 用 ISO 周的写法。
- 测试中注入固定的 clock 和 fake client。

#### `internal/content`

- `ReadSource(arg string, file string, stdin io.Reader) ([]byte, source, error)`：
  - 三种来源只能用一种；
  - `-f -` 读取 stdin；
  - 不自动读 stdin。
- `mimes.go`：按设计 §6.3 建立扩展名映射表（以 Trilium 的 `mime_type.ts` 为准，从 GitHub 拉取源码核对每一项）；实现 `InferFromFile(path, data) (type, mime)` 和 `MimeFromTitle(title)`。
- `IsBinary(type, mime, data)`：规则见设计 §6.3。
- `CodeBlockClass(mime)`：按设计 §6.1 的规则实现（供 skill 的文档和测试使用）。
- `PlainText(html) string`：按设计 §4.3 的规则实现，基于 `golang.org/x/net/html`。用设计 §6.1 中的样例 HTML 做 golden 测试，覆盖每一种元素。
- `HasHTMLTags(s)`：用于"text 笔记内容不含 HTML 标签"的警告。

#### `internal/output`

- 纯函数，返回字符串或写入 `io.Writer`：
  - `NoteLine(note, long bool)`；
  - `TreeLines(...)`；
  - `GetHeader(note, attrs, attachments, opts)`；
  - 各种结果行（`Created`、`Appended`、`Edited`、`Updated`、`Deleted`、`Moved` …）；
  - `ErrorLines(err) (msg, hint)`；
  - `JSONL(v)`。
- 时间显示用本地时区，格式为 `YYYY-MM-DD HH:MM`，由 `utc*` 字段解析得到。
- 错误码到提示和退出码的映射表（设计 §4.6、§8.3）。
- 每种输出都做 golden 测试（`testdata/*.golden`），支持 `go test ./... -update` 更新 golden 文件。

**验收**：P2 的所有包都是纯逻辑或可注入依赖，单元测试覆盖率不低于 90%。

### P3 第 1 层命令（`internal/cli`）

- `root.go`：
  - 定义 `Runtime` 并注入依赖（设计 §8.4）；
  - 全局参数 `--json`；
  - **精简的帮助模板**：根帮助按设计 §5 开头的草稿，子命令的帮助只包含 Usage、一句话说明、参数和 1–3 个例子；
  - `trilium help search` 输出搜索语法速查（内容和 skill 的 `search.md` 共用一份 embed 的文本）。
- 按设计 §5.1–§5.8 实现 `search`、`ls`、`get`、`create`、`append`、`edit`、`put`、`rm`，逐条对照设计的行为说明和输出格式。重点注意：
  - `search`：请求 limit+1 判断是否还有更多；过滤 `_*`；加引号提示。
  - `ls`：只发两次请求；过滤祖先笔记本身；按 `childNoteIds` 排序；`-d` 输出树；`-n` 截断并提示。
  - `get`：用 `sync.WaitGroup.Go` 或 errgroup 并发请求；二进制不下载；`--text` 的头部要有标注；多个 note 按参数顺序输出。
  - `create`：`-f` 推断类型；`-t search` 设置 `#searchString`；`-a` 和 `TRILIUM_AGENT`；二进制分两步上传；`--id`。
  - `append`、`edit`：基于 blobId 的乐观并发和重试（设计 §5.5）；`edit` 成对的 `--old/--new` 要么全部成功、要么都不写。
  - `rm`：有子节点时要求 `-r`；在多个位置时提示；保护系统笔记。
- 命令层测试：用一个**内存版的 fake ETAPI 服务器**（`internal/etapi/etapitest`，基于 httptest），实现 notes、branches、attributes、attachments、content、search 的最小子集（search 只需支持测试中用到的几种查询形式即可）。断言输出与 golden 文件一致，并检查退出码。

**验收**：

- 第 1 层的每个命令至少有一个正常路径和一个错误路径的 golden 测试；
- 对真实服务器手动跑通设计 §5.1–§5.8 中的每个示例，输出格式与设计一致。

### P4 第 2 层命令

按设计 §5.9–§5.14 实现 `update`、`attr`、`mv`、`branch`（ls/add/set/rm）、`att`（ls/get/add/put/update/rm）、`rev`（ls/get/add）。重点注意：

- `attr`：先全部解析和校验，再执行；设置、追加、删除的语义；关系和可继承性的修改通过删除后重建实现。
- `mv` 和 `branch add`：环检测（设计 §5.11）；`mv` 先建后删。
- `branch rm`：删除最后一个分支时的保护。
- `branch set`：修改位置后调用 `refresh-note-ordering`。
- `att add`：role 为 image 时额外输出 `embed:` 行。
- `rev add`：创建后再查最新的修订 ID。

**验收**：fake 服务器上的 golden 测试，加上真实服务器上的手动验证；**特别要验证**环检测能拦住"把笔记克隆到自己的子节点下"。

### P5 第 3 层命令

按设计 §5.15–§5.18 实现 `history`、`export`、`import`（zip 和 md）、`undelete`、`info`、`backup`、`version`。

- `import x.md`：在内存中生成 zip 和 `!!!meta.json`（设计 §6.4），`appVersion` 取自 `/app-info`。
- **验证并补全设计 §6.1 中标注了"需要验证"的内部链接写法**：导入一个包含 `[t](#root/<id>)` 的 md，观察 Trilium 生成的 HTML，然后把结论更新到设计 §6.1 和 skill 的 `html.md`。

**验收**：在真实服务器上，`import` 一个包含设计 §6.1 中全部元素的 md 文件，生成的 HTML 与设计 §6.1 的表格一致。

### P6 E2E 测试（`e2e/`）

- 只在 `TRILIUM_E2E=1` 时运行，使用 `TRILIUM_URL` 和 `TRILIUM_ETAPI_TOKEN`。
- 测试流程：
  1. `TestMain` 在 root 下创建沙盒笔记 `trilium-cli-e2e-<随机>`；
  2. 所有测试都在沙盒内进行；
  3. 结束时 `rm -r` 删除沙盒（用 defer，测试失败也要执行）。
- **日历别名的测试**：先记录 root 下的子节点；测试结束后，如果出现了新的 Calendar 根笔记，就把它删除。
- 通过编译出的二进制来调用（`exec.Command`），覆盖：
  - 第 1–3 层每个命令的主路径；
  - 设计 §7 中每一条怪癖对应的处理（gzip、搜索加引号提示、环检测、JSON 类型的拒绝、`create` 的 content 字段）；
  - 二进制往返：上传 PNG 和随机字节文件后下载，逐字节比对；
  - 并发：两个 goroutine 同时 `append` 同一条笔记，最终两段内容都在。
- **不测试** `backup`（会在服务器磁盘上留下无法通过 API 清理的文件），只做单元测试。

**验收**：`make e2e` 全部通过，测试结束后 root 下的子节点和测试前完全一致。

### P7 Skill（`skills/trilium/`）

按设计第 9 节撰写 `SKILL.md` 和 4 个参考文档。

- 先写 `references/commands.md`：根据实际的 `--help` 输出整理，确保和实现一致。
- `html.md` 的每个例子都要先写入一条真实笔记，确认能正常显示（可以在 Trilium 网页端查看，或者 `get` 回来检查 HTML 没有被服务器改写）。
- SKILL.md 的每个"常用模式"示例都要在沙盒中运行一遍，把实际输出粘贴进去作为期望输出示例（ID 替换成占位符）。
- 自查清单：
  - [ ] description 写明了能做什么、什么时候用；
  - [ ] SKILL.md 不超过 200 行；
  - [ ] 引用只有一层；
  - [ ] 超过 100 行的参考文档有目录；
  - [ ] 每个示例都运行过；
  - [ ] "必须知道的坑"覆盖了设计 §9.2 的全部条目。
- **试用验证**：开一个全新的 agent 会话（不给任何本项目的上下文），只安装这个 skill，让它完成 5 个任务。记录它的调用次数和出错情况，据此修改 skill。任务如下：
  1. 找到某个主题的笔记并总结；
  2. 在今天的日记里记一条；
  3. 修改一篇长笔记中的一个段落；
  4. 给一组笔记打上状态标签；
  5. 上传一张图片并嵌入到笔记里。

**验收**：5 个任务都能完成，没有因为误解 skill 而产生的错误调用。

### P8 README

- 按设计第 10 节撰写 `README.md`（英文）和 `README.zh-CN.md`（中文），顶部互相链接。
- 所有示例命令和输出来自实际运行，不能凭想象编写。
- 写完后以"第一次接触 Trilium CLI 的普通用户"的视角通读一遍：能否在 5 分钟内完成安装、设置并跑出第一条命令。

### P9 发布准备

- `.goreleaser.yaml`：darwin、linux、windows 的 amd64 和 arm64，生成校验和。
- GitHub Actions：push 和 PR 时运行 lint 和 test；打 tag 时运行 goreleaser。
- 替换掉 module 路径的占位符；添加 LICENSE（确认之后）。

---

## 2. 推荐的实施顺序和提交节奏

P0 → P1 → P2 → P3 →（这时就可以开始 P6 的 E2E 框架，和后续阶段并行）→ P4 → P5 → P6 → P7 → P8 → P9。

每完成一个命令就提交一次，例如：`Add search command`、`Add attr command with set/add/remove semantics`。

---

## 3. 测试策略总览

| 层次 | 范围 | 工具 |
|---|---|---|
| 单元测试 | attrspec、ref、content、output、config、etapi 客户端 | 表驱动测试、httptest、golden 文件 |
| 命令测试 | 每个 CLI 命令的输出和退出码 | 内存版 fake ETAPI 服务器 + golden 文件 |
| E2E 测试 | 真实的 Trilium | 编译出的二进制 + 沙盒 + 自动清理 |
| 文档验证 | skill 和 README 中的示例 | 在沙盒中手动运行 |

### 3.1 golden 文件的约定

- 放在各包的 `testdata/<name>.golden` 中。
- `go test ./... -update` 重新生成。生成后必须人工检查 diff，确认变化符合设计。

### 3.2 fake 服务器的范围

只实现命令测试需要的接口和行为，但必须模拟这几条怪癖：大于 1KB 的响应强制 gzip、分支 ID 为 `<parent>_<child>`、搜索结果包含祖先笔记本身。

### 3.3 E2E 测试的安全规则

- 只在自己创建的沙盒笔记下写入。
- 不调用 `/auth/logout`，也不调用 `/backup`。
- 测试结束后核对 root 的子节点和测试前一致，不一致就让测试失败并打印残留的笔记。

---

## 4. Token 效率核查（`make tokens`）

写一个脚本 `scripts/token-audit.sh`：在沙盒中运行一组典型操作，统计每个命令的 stdout 字节数，并和直接调用 ETAPI 返回的字节数对比，生成表格。

| 场景 | 目标 |
|---|---|
| `get -m` 一条有 3 个属性的笔记 | ≤ 300 字节；ETAPI 原始 JSON 约 600 字节 |
| `search` 返回 10 条结果 | 平均每行 ≤ 60 字节，加上标题长度 |
| `create` / `append` / `edit` 的结果行 | ≤ 100 字节，加上标题长度 |
| `ls -d 2` 50 个节点 | ≤ 3 KB |

P3 和 P7 结束时各运行一次，把结果记录在 PR 描述或 `docs/token-audit.md` 中。超出目标时要么优化输出，要么说明原因。

---

## 5. 完成标准（v1）

- [ ] 设计第 5 节列出的全部命令都已实现，行为和输出与设计一致
- [ ] 设计 §7 的每一条怪癖都有对应的处理，并有测试覆盖
- [ ] `make lint test e2e` 全部通过；E2E 结束后服务器上没有残留
- [ ] skill 通过了 P7 的试用验证
- [ ] README 中英文版完成，示例都来自实际运行
- [ ] 达到 `make tokens` 的目标
- [ ] 实施中对设计所做的修改都已更新到 `docs/design.md`，并在 PR 中列出
