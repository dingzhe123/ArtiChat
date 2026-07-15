# ArtiChat — AI 智能问答文章网站

一个带智能问答机器人的文章网站。用户可以浏览文章，也可以向 AI 机器人提问，机器人会基于已推送的文章内容给出回答。

## 功能特性

- **文章系统**：Markdown 写作，支持创建、编辑、删除，列表分页浏览
- **管理后台**：可视化文章管理，标签芯片输入，Basic Auth 保护
- **智能问答**：基于 RAG 架构，检索相关文章片段后由大模型生成回答
- **RAG 检索**：支持向量检索（需 Embedding API）+ 关键词检索自动降级
- **流式输出**：问答回复支持 SSE 流式传输，打字机效果
- **SEO 友好**：服务端渲染，Open Graph 标签，JSON-LD 结构化数据
- **响应式设计**：适配桌面端与移动端

## 技术栈

| 层面 | 技术 |
|------|------|
| 后端 | Go 1.22+, net/http 增强路由 |
| 模板 | html/template（SSR） |
| 数据库 | SQLite（modernc.org/sqlite，纯 Go） |
| Markdown | goldmark |
| 大模型 | OpenAI 兼容 API（DeepSeek 等） |

## 快速开始

### 环境要求

- Go 1.22+
- OpenAI 兼容的 LLM API（如 DeepSeek）

### 安装运行

```bash
# 克隆仓库
git clone https://github.com/dingzhe123/ArtiChat.git
cd ArtiChat

# 配置环境变量
cp .env.example .env
# 编辑 .env，填入你的 LLM_API_KEY

# 启动服务
go run main.go
```

服务默认运行在 `http://localhost:8080`。

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 服务端口 | `8080` |
| `DB_PATH` | SQLite 数据库路径 | `data/site.db` |
| `LLM_API_KEY` | 大模型 API 密钥（**必填**） | — |
| `LLM_BASE_URL` | API 基础地址 | `https://api.deepseek.com/v1` |
| `LLM_MODEL` | 对话模型 | `deepseek-chat` |
| `EMBEDDING_MODEL` | Embedding 模型 | `text-embedding-ada-002` |
| `ADMIN_USER` | 管理后台用户名 | `admin` |
| `ADMIN_PASS` | 管理后台密码 | `admin123` |

## 项目结构

```
├── main.go                      # 入口：路由注册与服务启动
├── config/
│   └── config.go                # 配置管理（.env + 环境变量）
├── models/
│   └── article.go               # Article / ArticleChunk 数据模型
├── handlers/
│   ├── home.go                  # 主页处理器
│   ├── article.go               # 文章列表/详情 + CRUD API
│   ├── admin.go                 # 管理后台页面
│   ├── chat.go                  # 问答 API + 重建索引
│   ├── auth.go                  # Basic Auth 中间件
│   └── seo.go                   # SEO 工具函数
├── services/
│   ├── article_service.go       # 文章持久化（SQLite）
│   ├── rag_service.go           # RAG：分块 → 向量化 → 检索
│   └── llm_service.go           # LLM API 客户端
├── middleware/
│   └── middleware.go            # 日志 + CORS
├── static/
│   ├── css/style.css
│   └── js/
│       ├── chat.js              # 聊天组件
│       └── admin.js             # 管理后台交互
├── templates/
│   ├── base.html                # 基础布局 + 聊天挂件
│   ├── home.html                # 主页
│   ├── article_list.html        # 文章列表
│   ├── article_detail.html      # 文章详情
│   └── admin.html               # 管理后台
└── data/
    └── site.db                  # SQLite 数据库（运行时生成）
```

## API 文档

### 公开接口

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/` | 主页 |
| `GET` | `/articles` | 文章列表页 |
| `GET` | `/articles/{id}` | 文章详情页 |
| `POST` | `/api/chat` | 智能问答（JSON: `{"question": "..."}`） |

### 管理接口（需 Basic Auth）

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/admin` | 管理后台页面 |
| `GET` | `/api/articles/{id}` | 获取单篇文章 JSON |
| `POST` | `/api/articles` | 创建文章 |
| `PUT` | `/api/articles/{id}` | 更新文章 |
| `DELETE` | `/api/articles/{id}` | 删除文章 |
| `POST` | `/api/reindex` | 重建 RAG 向量索引 |

## RAG 架构

```
用户提问 → 问题向量化 → 向量相似度检索 → 取 Top-K 文本块
                                              ↓
              流式返回答案 ← 大模型 ← System Prompt + 上下文 + 用户问题
```

- **分块策略**：按段落切分，长段落按句子再拆分（最大 500 字符/块）
- **检索策略**：优先向量检索（余弦相似度），Embedding 不可用时自动降级为关键词检索
- **关键词检索**：混合分词（中文 unigram + bigram + 英文单词）+ 最长公共子串加分

## 快捷操作

| 快捷键 | 功能 |
|--------|------|
| `Ctrl+K` 或 `/` | 打开聊天窗口 |
| `Escape` | 关闭聊天窗口 |

## License

MIT
