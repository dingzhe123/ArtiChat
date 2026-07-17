# ArtiChat 协作过程记录

## 项目信息

- **项目名称**: ArtiChat — AI 智能问答文章网站
- **仓库地址**: https://github.com/dingzhe123/ArtiChat
- **技术栈**: Go + net/http + SQLite + DeepSeek API
- **协作模式**: 分步开发，每步测试通过后提交，确认后继续下一步

## 开发时间线

### 第一步：项目骨架与 HTTP 服务框架

**目标**: 搭建可运行的 Go HTTP 服务，完成路由和基础页面。

**关键决策**:
- 选择 Go 1.22+ 增强路由 `net/http.ServeMux`（`GET /articles/{id}` 模式），无需第三方路由库
- 前端采用 `html/template` 模板渲染，不做 SPA，确保 SEO 友好
- 使用 `modernc.org/sqlite` 纯 Go 驱动，免 CGO、跨平台

**模板架构问题与解决**: 最初使用 `template.ParseGlob("templates/*.html")` 加载全部模板，导致多个文件中的 `{{define "content"}}` 互相覆盖，始终只渲染最后一页的内容。解决方案是为每个处理器构建独立的模板集（`template.ParseFiles("base.html", "specific.html")`），每个集合只包含 base.html 和当前页面的内容模板。

**提交**: `bc461fa` feat: 搭建 Go HTTP 服务框架与路由

---

### 第二步：数据模型与 SQLite 存储

**目标**: 实现 Article 数据模型和完整的 CRUD 持久化。

**关键决策**:
- 标签以 JSON 数组存储在 SQLite TEXT 字段，灵活且无需额外关联表
- 启用 WAL 模式提升并发读取性能
- 时间字段以 RFC3339 字符串存储，方便 SQLite 直接比较

**提交**: `b219b08` feat: 实现文章数据模型与 SQLite 存储层

---

### 第三步：CRUD API 与管理后台

**目标**: 实现文章增删改查 API 和可视化管理页面。

**关键决策**:
- 管理后台采用混合模式：服务端渲染初始表格 + JS fetch 处理交互
- Markdown 渲染使用 `goldmark` 库，服务端完成转换，客户端拿到直接渲染
- Basic Auth 以闭包中间件实现，复用逻辑，保护管理路由
- `/api/articles` 端点使用 JSON 格式，与页面路由分离

**提交**: `1b7f3f0` feat: 实现文章 CRUD API 与管理后台页面

---

### 第四步：SEO 优化

**目标**: 完善 SEO 标签，确保搜索引擎友好。

**实现内容**:
- `<link rel="canonical">` 规范链接（每个页面自动从 request 构造）
- Open Graph 标签：动态 `og:type`（首页 `website`，文章页 `article`）
- Twitter Card 标签
- JSON-LD 结构化数据：首页 `WebSite`、列表页 `CollectionPage + ItemList`、详情页 `Article`
- 文章描述自动清洗 Markdown 语法，输出纯文本
- 新增 `handlers/seo.go` 统一管理 SEO 工具函数

**修复的问题**: 文章列表 JSON-LD 中 URL 路径重复（`/articles/articles/1`），原因是 canonical URL 已含 `/articles` 又拼接了一次。

**提交**: `bc261f4` feat: 完善 SEO 优化

---

### 第五步：RAG 检索与大模型问答

**目标**: 接入 LLM，实现基于文章内容的智能问答。

**关键决策与问题**:

1. **API 网关切换**: 原配置的 HELYLLM 网关（`115.190.217.1:38080`）API Key 失效且不支持 Embeddings 端点（返回 405）。切换到 DeepSeek API（`api.deepseek.com`），Chat Completion 正常，但同样不支持 Embedding。

2. **Embedding 不可用的应对**: 实现了双路径检索机制——优先尝试向量检索（余弦相似度），失败时自动降级到关键词匹配。

3. **关键词检索演进**:
   - v1: 单字分词 + 命中率。`"并发模型"` 拆成单字，无法匹配 `"并发编程"`
   - v2: Bigram 分词 + Jaccard 相似度。长文本被惩罚，召回偏低
   - v3: Unigram + Bigram 混合 + 最长公共子串加分。`"并发模型"` 可命中 `"并发编程"` chunk
   - 同时引入中文停用词表（约 90 个），过滤无效分词

4. **Embedding 超时优化**: 初始使用 120 秒超时会导致搜索长时间卡住。为 Embedding 请求单独设置 8 秒超时，快速回退到关键词检索。

5. **安全加固**:
   - `http.MaxBytesReader` 限制请求体大小（文章 1MB，聊天 32KB）
   - 聊天问题长度限制 2000 字符
   - 错误消息脱敏：客户端返回通用提示，真实错误写服务端日志

6. **配置外迁**: API Key 从源码移除，改为从 `.env` 文件/环境变量加载。`.env` 加入 `.gitignore`，提供 `.env.example` 模板。

**提交**: `901a9ae` feat: 实现 RAG 检索与大模型问答 API

---

### 第六步：前端聊天组件

**目标**: 在页面右下角实现问答聊天窗口。

**实现内容**:
- 浮动按钮 + 弹出面板，动画效果
- 消息渲染支持简单 Markdown（加粗、代码块、标题）
- 参考来源可折叠展示，显示检索相似度
- 发送中显示加载动画，发送按钮禁用防重复提交
- 错误状态提供重试按钮
- 键盘快捷键：`Ctrl+K` 或 `/` 打开，`Escape` 关闭
- 移动端响应式适配

**提交**: `402bfad` feat: 实现前端聊天组件，优化关键词检索召回

---

### 第七步：整体联调与样式优化

**目标**: 完善用户体验细节。

**实现内容**:
- 主页展示最新 5 篇文章（有文章时），无文章时显示欢迎页
- 文章详情页添加面包屑导航和预估阅读时间（400字/分钟）
- 管理后台：标签芯片输入（回车添加、点击删除）、字数统计、空状态优化
- 页面表情符号全部移除，保持简洁专业风格

**提交**: `e1a2779` feat: 整体联调与样式优化

---

### 第八步：代码质量整理

**目标**: 统一代码风格。

**实现内容**:
- 全部 Go 文件注释改为中文
- 所有模板和 JS 文件移除 Emoji 表情符号

**提交**: `8e435b4` refactor: 注释改为中文

---

### 第九步：文档编写

**目标**: 编写项目文档，方便他人了解和使用。

**实现内容**:
- README.md：项目介绍、快速开始、环境变量、项目结构、API 文档、RAG 架构
- COLLABORATION.md（本文件）：完整的协作过程记录

**提交**: `03633d4` docs: 添加 README 与协作过程文档

---

### 第十步：端到端测试与闭环修复

**目标**: 实际运行服务，测试全部端点，修复发现的问题。

**全量端点测试结果**: 10 个端点全部通过（页面/API/认证/404/问答/索引重建）。

**测试中发现的 Bug**:

1. **文章摘要残留 Markdown 语法** — 模板使用 `{{printf "%.200s" .Content}}` 直接截取原始 Markdown，导致 `#`、`##`、`**` 等标记出现在摘要中。
   - 修复：将 `StripMarkdown` 和 `Truncate` 导出为模板函数，注册到 `template.FuncMap`，模板改为 `{{Truncate (StripMarkdown .Content) 200}}`
   - 同时修复 Markdown 清洗正则缺少多行模式（`(?m)`），导致段落中间的标题和列表项无法清除

2. **404 页面返回纯文本** — `http.NotFound(w, r)` 输出 `404 page not found` 纯文本，无 HTML 结构。
   - 修复：新增 `serveNotFound()` 函数，返回带站点头部/导航/返回链接的完整 HTML 页面

3. **主页实际为 SSR 而非纯静态** — 原 `HomeHandler` 查询数据库获取最新 5 篇文章，与 CLAUDE.md 中"纯静态页面"的要求不符。
   - 修复：去掉 `HomeHandler` 对 `ArticleService` 的依赖，不查询数据库，只渲染静态的欢迎页 + 功能介绍卡片。文章浏览统一走 `/articles` 页面。

**提交**: `0f50038` fix: 端到端测试、主页改造为纯静态、修复摘要与404问题

---

### 第十一步：SEO 深化优化

**目标**: 查漏补缺，使站点对搜索引擎更加友好。

**实现内容**:
- **robots.txt** — 声明全站允许爬取，指向 sitemap
- **sitemap.xml** — 动态生成，含主页（1.0）、列表页（0.8）、文章页（0.6）三级优先级
- **h1 去重** — Goldmark 渲染的 Markdown 内容中标题自动降一级（h1→h2），确保每页只有一个 h1
- **标题层级修正** — 主页 feature 卡片从 h3 改为 h2，形成 h1→h2 正常层级

**提交**: `4823d8a` fix: SEO 优化 — robots.txt/sitemap、h1 去重、标题层级修正

---

## 问题与解决汇总

| 问题 | 解决方案 |
|------|----------|
| 模板 `{{define}}` 冲突 | 每个处理器独立模板集（`parseSet` 模式） |
| Windows 终端 GBK 破坏中文 | 使用 `--data-binary @file` 发送 API 请求 |
| HELYLLM API Key 失效 + 无 Embedding | 切换到 DeepSeek API |
| Embedding 端点不存在 | 实现关键词检索 fallback（混合分词 + LCS） |
| Embedding 请求卡住 120 秒 | 单独设置 8 秒短超时 |
| 错误消息泄露内部信息 | 客户端返回通用提示，真实错误写 log |
| API Key 硬编码 | 迁移到 .env 文件 |
| 首页无内容显得空洞 | ~~改为纯静态欢迎页~~（后修正：首页应保持纯静态） |
| 文章列表 JSON-LD URL 重复 | `TrimSuffix` 去除重复路径段 |
| 表格标题过长撑坏布局 | `printf "%.30s"` 截断显示 |
| 文章摘要残留 Markdown 语法 | 注册 StripMarkdown/Truncate 模板函数 + 正则多行模式 |
| 404 页面为纯文本 | 新增 serveNotFound()，返回完整 HTML 页面 |
| 主页实际为 SSR 非纯静态 | 去掉 DB 查询，改为纯静态欢迎页 + 功能卡片 |
| 文章页存在两个 h1 | Goldmark 渲染后标题降级（h1→h2） |
| 主页 h3 跳过 h2 层级 | h3 改为 h2，形成 h1→h2 正确层级 |
| 缺少 robots.txt / sitemap.xml | 新增路由，sitemap 动态生成文章 URL |

## 最终项目结构

```
ai-article-site/
├── main.go
├── go.mod / go.sum
├── .env.example                # 环境变量模板
├── README.md                    # 项目说明与使用文档
├── COLLABORATION.md             # 协作过程记录
├── config/
│   └── config.go               # 配置管理（.env 加载 + 环境变量）
├── models/
│   └── article.go              # Article + ArticleChunk 模型
├── handlers/
│   ├── home.go                 # 主页
│   ├── article.go              # 文章页面 + CRUD API
│   ├── admin.go                # 管理后台
│   ├── chat.go                 # 问答 + 重建索引 API
│   ├── auth.go                 # Basic Auth 中间件
│   └── seo.go                  # SEO 工具函数
├── services/
│   ├── article_service.go      # 文章持久化
│   ├── rag_service.go          # RAG：分块 + 检索
│   └── llm_service.go          # LLM API 客户端
├── middleware/
│   └── middleware.go            # 日志 + CORS
├── static/
│   ├── css/style.css
│   └── js/
│       ├── chat.js             # 聊天组件
│       └── admin.js            # 管理后台交互
├── templates/
│   ├── base.html               # 基础布局
│   ├── home.html               # 主页
│   ├── article_list.html       # 文章列表
│   ├── article_detail.html     # 文章详情
│   └── admin.html              # 管理后台
└── data/
    └── site.db                 # SQLite（运行时生成，已 gitignore）
```
