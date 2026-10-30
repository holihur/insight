# Code Insight

> 每个代码库都是一座迷宫。Code Insight 带你走到中心。

把任意公开 Git 仓库变成一份**可阅读、可追问、有源码依据**的代码 Wiki。输入仓库地址，剩下的交给分析代理：它会阅读代码、画出结构、解释关键流程，并随时回答你的问题。

![Code Insight 首页](docs/screenshot-home.jpg)

## 它做什么

- **输入一个仓库地址**，自动完成拉取、索引和语言 / 模块统计。
- **生成 5–8 个章节的 Wiki**：概览、快速开始、架构、核心子系统、关键文件等。
- **每个结论都来自源码**：代理使用文件树、文件读取和代码搜索工具，而不是凭空猜测。
- **像问同事一样追问**：流式回答，展示它正在读哪个文件、搜什么关键词，并引用真实路径。
- **直接在浏览器里读代码**：文件树、语法高亮、行号、复制和跳转 GitHub。
- **支持 Mermaid 图**：架构和调用关系可以直接渲染成图。

![仓库 Wiki 页面](docs/screenshot-repo.jpg)

## 快速开始

需要 [mise](https://mise.jdx.dev/) 和 `git`。

```bash
git clone https://github.com/holihur/insight.git
cd insight

mise install          # 安装 Go / Node / pnpm
mise run install      # 安装前后端依赖

cp .env.example .env  # 填入模型 API Key（见下方）
mise run dev          # 后端 :8080，前端 :5173
```

打开 <http://localhost:5173>，粘贴一个公开仓库地址，例如 `gin-gonic/gin`。

### 生产方式运行

```bash
mise run build
mise run start        # http://localhost:8080
```

`mise run build` 会先构建前端，再把前端资源嵌入 Go 二进制。最终只需要一个可执行文件，不依赖 Node 或静态文件目录（`WEB_DIR` 可覆盖）。

## 配置

在项目根目录的 `.env` 中填入模型服务。默认支持 DeepSeek：

```dotenv
DEEPSEEK_API_KEY=sk-...
DEEPSEEK_MODEL=deepseek-flash
```

也支持任意 OpenAI 兼容接口（OpenAI、OpenRouter、Ollama、vLLM 等）：

```dotenv
OPENAI_API_KEY=...
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_MODEL=gpt-4o-mini
```

其他常用项：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `DATA_DIR` | `./data` | 克隆和生成结果的存放目录 |
| `GITHUB_TOKEN` | 空 | 访问私有仓库时使用 |
| `GIT_CLONE_TIMEOUT` | `180` | Git 克隆超时（秒） |
| `MAX_REPO_FILES` | `8000` | 索引文件数上限 |
| `LLM_MAX_TOKENS` | `8192` | 单次生成上限 |

**没有配置 Key 也能用。** 此时会进入本地模式：用静态分析生成基础 Wiki，用代码搜索回答问题。配置 Key 后点击 **Regenerate** 即可获得完整的代理生成版本。

## 使用方式

1. 在首页粘贴仓库地址，点击 **Analyze repo**。
2. 等待克隆、索引、规划和写作完成（页面会实时显示进度）。
3. 在左侧浏览 Wiki 章节，或切到 **Code** 查看源码。
4. 在右侧 **Ask this repo** 中追问，例如：
   - “Give me a high-level architecture overview.”
   - “How does a request flow through this codebase?”
   - “Where is authentication handled?”
5. 需要重新生成时点击 **Regenerate**；删除分析会同时删除本地克隆。

## 数据与隐私

- 仓库只会以只读方式浅克隆到 `data/<repo-id>/repo`。
- 生成的 Wiki、索引和元数据保存在 `data/<repo-id>/`，不会上传到其他地方。
- `.env`、`data/`、构建产物和依赖目录都已加入 `.gitignore`。
- 删除某个分析时，对应的本地数据会一起删除。

## 开发

```bash
mise run dev      # 前后端一起启动（Vite 代理 /api 到 :8080）
mise run check    # go vet + TypeScript 类型检查
mise run build    # 构建前端并编译 Go 二进制
```

目录结构：

```text
backend/   Go 服务：克隆 / 索引 / 分析代理 / HTTP API / 静态资源托管
frontend/  React + Tailwind 前端
docs/      截图
scripts/   开发脚本
```

后端负责拉取与理解代码、生成 Wiki、提供流式对话；前端构建完成后会嵌入 Go 二进制。生产环境只需要运行 `bin/code-insight`。

## 说明

- 目前面向公开的 GitHub / GitLab 仓库；私有仓库请配置 `GITHUB_TOKEN`。
- 大型仓库的首次分析可能需要几分钟；页面会持续显示代理正在读取的文件。
- 所有引用都基于分析时的代码快照；重新生成即可更新到最新提交。
