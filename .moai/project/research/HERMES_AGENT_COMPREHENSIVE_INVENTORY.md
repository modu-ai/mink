# Hermes Agent - Comprehensive Repository Inventory

**Repository**: NousResearch/hermes-agent  
**Latest Version**: v0.8.0  
**License**: MIT  
**Language**: Python (core), JavaScript/Node.js (frontend/gateway)

---

## 1. DIRECTORY TREE (Key Structure)

```
hermes-agent/
├── README.md                      # Main documentation entry point
├── cli.py                        # Interactive terminal UI (9,185 lines)
├── run_agent.py                  # Core agent orchestrator (9,845 lines)
├── batch_runner.py               # Batch trajectory generation (1,287 lines)
├── mcp_serve.py                  # MCP (Model Context Protocol) server
├── mini_swe_runner.py            # Software engineering specific agent runner
├── model_tools.py                # Tool discovery & orchestration
├── toolsets.py                   # Tool grouping/categorization system
├── hermes_state.py               # SQLite state persistence (sessions, messages)
├── hermes_constants.py           # Global constants & paths
├── hermes_logging.py             # Logging configuration
├── hermes_time.py                # Time utilities
├── rl_cli.py                     # Reinforcement learning CLI
├── trajectory_compressor.py      # RL trajectory compression
├── utils.py                      # General utilities
│
├── agent/                        # Core agent module (24 files)
│   ├── __init__.py
│   ├── anthropic_adapter.py      # Anthropic API integration
│   ├── auxiliary_client.py       # OpenAI/third-party client handling
│   ├── builtin_memory_provider.py # Built-in memory system
│   ├── context_compressor.py     # Token/context optimization
│   ├── context_references.py     # Context file handling
│   ├── copilot_acp_client.py     # Copilot ACP client
│   ├── credential_pool.py        # API key management
│   ├── display.py                # Terminal UI formatting
│   ├── error_classifier.py       # API error handling
│   ├── insights.py               # Usage analytics
│   ├── memory_manager.py         # Memory orchestration
│   ├── memory_provider.py        # Memory plugin interface
│   ├── model_metadata.py         # Model info & token estimation
│   ├── models_dev.py             # Dev model setup
│   ├── prompt_builder.py         # System prompt assembly
│   ├── prompt_caching.py         # Anthropic prompt caching
│   ├── rate_limit_tracker.py     # API rate limiting
│   ├── redact.py                 # Sensitive data filtering
│   ├── retry_utils.py            # Exponential backoff logic
│   ├── skill_commands.py         # Skill execution commands
│   ├── skill_utils.py            # Skill parsing/discovery
│   ├── smart_model_routing.py    # Model selection logic
│   ├── subdirectory_hints.py     # Context file organization
│   ├── title_generator.py        # Session title generation
│   ├── trajectory.py             # RL trajectory handling
│   └── usage_pricing.py          # Cost estimation
│
├── tools/                        # Tool implementations (53+ Python files)
│   ├── __init__.py
│   ├── registry.py               # Central tool registry
│   ├── web_tools.py              # Web search, content extraction
│   ├── terminal_tool.py          # Command execution (6 backends)
│   ├── file_tools.py             # File operations (read/write/patch)
│   ├── vision_tools.py           # Image analysis
│   ├── browser_tool.py           # Browser automation (Playwright, etc.)
│   ├── browser_camofox.py        # Anti-detection browser features
│   ├── browser_providers/        # Browser provider backends (browserbase, firecrawl, etc.)
│   ├── skills_tool.py            # Skill management
│   ├── memory_tool.py            # Memory tool implementation
│   ├── session_search_tool.py    # Full-text session search
│   ├── todo_tool.py              # To-do list management
│   ├── delegate_tool.py          # Subagent spawning
│   ├── code_execution_tool.py    # Python code execution
│   ├── image_generation_tool.py  # Image generation (Fal, Replicate, etc.)
│   ├── tts_tool.py               # Text-to-speech
│   ├── cronjob_tools.py          # Cron job scheduling
│   ├── mcp_tool.py               # MCP server tool integration
│   ├── homeassistant_tool.py     # Home Assistant integration
│   ├── mixture_of_agents_tool.py # MOA ensemble calls
│   ├── send_message_tool.py      # Platform-agnostic messaging
│   ├── clarify_tool.py           # Interactive clarification
│   ├── environments/             # 8 execution environment backends
│   │   ├── base.py               # Base environment interface
│   │   ├── local.py              # Local shell execution
│   │   ├── docker.py             # Docker container execution
│   │   ├── ssh.py                # Remote SSH execution
│   │   ├── modal.py              # Modal serverless GPU
│   │   ├── managed_modal.py      # Modal with persistence
│   │   ├── daytona.py            # Daytona IDE integration
│   │   ├── singularity.py        # Singularity container
│   │   └── modal_utils.py        # Modal utilities
│   ├── browser_providers/        # Browser automation backends
│   │   ├── base.py
│   │   ├── browser_use.py
│   │   ├── browserbase.py
│   │   ├── firecrawl.py
│   │   └── __init__.py
│   └── [30+ more tools...]
│
├── hermes_cli/                   # CLI application layer (44 modules)
│   ├── __init__.py
│   ├── main.py                   # CLI entry point
│   ├── banner.py                 # ASCII art & formatting
│   ├── commands.py               # Slash command handlers
│   ├── auth.py                   # Authentication flow
│   ├── auth_commands.py          # Auth-related commands
│   ├── config.py                 # Configuration management
│   ├── model_switch.py           # Model switching logic
│   ├── models.py                 # Model provider handling
│   ├── cron.py                   # Cron job management
│   ├── doctor.py                 # System health checking
│   ├── gateway.py                # Messaging gateway setup
│   ├── curses_ui.py              # Terminal UI components
│   ├── callbacks.py              # Event callbacks
│   ├── pairing.py                # DM pairing flow
│   ├── plugins.py                # Plugin discovery/loading
│   ├── mcp_config.py             # MCP server configuration
│   └── [20+ more modules...]
│
├── gateway/                      # Messaging platform gateway (11 modules)
│   ├── __init__.py
│   ├── run.py                    # Gateway orchestrator
│   ├── session.py                # Gateway session management
│   ├── config.py                 # Gateway configuration
│   ├── delivery.py               # Message delivery system
│   ├── pairing.py                # User pairing/DM management
│   ├── status.py                 # Gateway status tracking
│   ├── hooks.py                  # Event hooks system
│   ├── builtin_hooks/            # Default hook implementations
│   ├── platforms/                # Platform handlers (telegram, discord, slack, etc.)
│   └── channel_directory.py      # Channel registry
│
├── acp_adapter/                  # Anthropic Copilot Protocol adapter (8 modules)
│   ├── __init__.py
│   ├── entry.py
│   ├── server.py
│   ├── auth.py
│   ├── session.py
│   ├── events.py
│   ├── tools.py
│   └── permissions.py
│
├── skills/                       # Procedural memory library (28+ categories)
│   ├── apple/                    # Apple ecosystem skills
│   ├── github/                   # GitHub/Git workflows
│   ├── software-development/     # SWE-specific skills
│   ├── mlops/                    # ML operations
│   ├── research/                 # Research methodology
│   ├── productivity/             # Productivity workflows
│   ├── creative/                 # Creative tasks
│   └── [20+ more categories...]
│
├── environments/                 # RL training environments (14 modules)
│   ├── ai_town_env.py
│   ├── atropos_env.py
│   └── [12+ more...]
│
├── optional-skills/              # Optional skill packs (16 directories)
├── hermes_cli/plugins            # Plugin system
├── tests/                        # Test suite (43+ test files)
├── scripts/                      # Installation & utility scripts
├── docs/                         # Documentation source
├── website/                      # Landing page/docs site
├── landingpage/                  # Alternative landing page
├── docker/                       # Docker configuration
├── nix/                          # Nix package definition
└── [config files: pyproject.toml, requirements.txt, etc.]
```

---

## 2. KEY ENTRY POINTS & EXECUTION PATHS

### 2.1 CLI Entry Point (`hermes` command)
- **File**: `cli.py` (9,185 lines)
- **Entry Function**: `main()` via `hermes_cli/main.py`
- **Purpose**: Interactive terminal UI with persistent TUI, streaming output, interrupt handling
- **Key Components**:
  - `prompt_toolkit` for fixed input area
  - `KeyBindings` for slash-command autocomplete
  - Message history via `FileHistory`
  - Streaming tool output with spinner
  - Config loading from `~/.hermes/config.yaml` or `./cli-config.yaml`

### 2.2 Agent Conversation Loop (`run_agent.py`)
- **File**: `run_agent.py` (9,845 lines)
- **Main Class**: `AIAgent`
- **Entry Method**: `AIAgent.run_conversation(user_input: str) -> str`
- **Data Flow**:
  1. Build system prompt (identity + memory + skills + context files)
  2. Add user message to conversation history
  3. Call LLM via `openai.Client` (OpenRouter, OpenAI, Anthropic, etc.)
  4. Parse response (handle tool calls, streaming, etc.)
  5. Execute tools via `model_tools.handle_function_call()`
  6. Loop until tool calls complete or max iterations reached
  7. Return final assistant response
- **Responsible For**:
  - Conversation state management
  - Tool calling orchestration
  - Token tracking & cost estimation
  - Error classification & retries
  - Memory synchronization

### 2.3 Tool Execution Flow
- **Entry Point**: `model_tools.handle_function_call(tool_name, args, task_id)`
- **Resolution**:
  1. Look up tool in `tools.registry` (pre-loaded at module import)
  2. Check availability via `ToolEntry.check_fn`
  3. Route to handler (sync or async)
  4. Catch/format result or error
- **Registry**: Centralized in `tools/registry.py` (ToolRegistry class)

### 2.4 Batch Processing (`batch_runner.py`)
- **Purpose**: Generate trajectories for RL training
- **Entry**: `batch_runner.py` with config YAML
- **Outputs**: `.jsonl` trajectory files

### 2.5 MCP Server (`mcp_serve.py`)
- **Purpose**: Expose Hermes tools via Model Context Protocol
- **Supports**: Anthropic Claude, OpenAI Codex, other MCP clients

### 2.6 Gateway (Messaging Platforms)
- **Entry**: `hermes gateway start`
- **File**: `gateway/run.py`
- **Platforms**: Telegram, Discord, Slack, WhatsApp, Signal, Email
- **Flow**: Platform → Session → `run_agent.py` → Tools → Platform

---

## 3. CORE MODULES - DETAILED SUMMARIES

### 3.1 AGENT MODULE (`agent/`)
**Purpose**: Core agent orchestration and AI reasoning

#### `memory_manager.py`
- **Class**: `MemoryManager`
- **Responsibility**: Orchestrates built-in + 1 external memory provider
- **Key Methods**:
  - `add_provider(provider)` - Register memory provider
  - `build_system_prompt()` - Inject memory into system prompt
  - `prefetch_all(user_message)` - Recall memory before API call
  - `sync_all(user_msg, assistant_response)` - Save memories post-turn
- **Data**: Fenced memory context blocks to prevent injection

#### `prompt_builder.py`
- **Constants**:
  - `DEFAULT_AGENT_IDENTITY` - Core personality
  - `MEMORY_GUIDANCE` - Memory tool instructions
  - `SKILLS_GUIDANCE` - Skill discovery hints
  - `TOOL_USE_ENFORCEMENT_GUIDANCE` - For models requiring tool enforcement
- **Functions**:
  - `build_system_prompt()` - Assemble complete system prompt
  - `_scan_context_content()` - Detect prompt injection in SOUL.md, .cursorrules
  - `load_soul_md()` - Load SOUL.md files (agent personality override)
  - `build_skills_system_prompt()` - Generate skill index
  - `build_context_files_prompt()` - Inject .hermes.md, AGENTS.md

#### `context_compressor.py`
- **Class**: `ContextCompressor`
- **Purpose**: Optimize message history to fit context window
- **Strategy**: Token estimation → drop oldest non-tool-call messages
- **Methods**:
  - `compress(messages, max_tokens, model)` - Compress to fit budget

#### `memory_provider.py`
- **Interface**: `MemoryProvider` (abstract base)
- **Methods** (to be implemented by plugins):
  - `get_tool_schemas()` - Tool definitions for memory ops
  - `prefetch(user_input)` - Retrieve memory before turn
  - `sync(user_input, assistant_response)` - Save memory after turn
  - `handle_tool_call(tool_name, args)` - Execute memory tool

#### `builtin_memory_provider.py`
- **Built-in memory** (no external dependency)
- **Tools**: Simple JSON-based storage
- **Location**: `~/.hermes/memory.json`

#### `model_metadata.py`
- **Purpose**: Model capability detection
- **Functions**:
  - `fetch_model_metadata(provider, model)` - Get context limit, costs
  - `estimate_tokens_rough(messages)` - Rough token estimation
  - `query_ollama_num_ctx()` - Detect local Ollama capabilities
  - `save_context_length()` - Cache discovered context limits

#### `error_classifier.py`
- **Class**: `FailoverReason` (enum)
- **Purpose**: Classify API errors for retry/failover logic
- **Types**: context_limit, rate_limit, auth_error, service_error, etc.
- **Function**: `classify_api_error(status, message)` → `FailoverReason`

#### `usage_pricing.py`
- **Class**: `CanonicalUsage` (dataclass)
- **Tracks**: Input/output/cache tokens, reasoning tokens
- **Functions**:
  - `estimate_usage_cost(provider, model, tokens)` - Cost calculation
  - `format_token_count_compact()` - Pretty token display
  - `format_duration_compact()` - Pretty time display

#### `trajectory.py`
- **Purpose**: RL trajectory format (converting to think tags, etc.)
- **Functions**:
  - `convert_scratchpad_to_think()` - Claude thinking format
  - `save_trajectory_to_file()` - Persist RL training data

#### `skill_utils.py`
- **Purpose**: Skill discovery and parsing
- **Functions**:
  - `get_all_skills_dirs()` - Locate skill directories
  - `iter_skill_index_files()` - Find skill metadata
  - `parse_frontmatter()` - YAML frontmatter parsing
  - `extract_skill_description()` - Skill text extraction
  - `skill_matches_platform()` - Filter skills by platform

#### `credential_pool.py`
- **Purpose**: Manage API keys from multiple sources
- **Sources**: Environment, credential files, ~/.hermes/, secret managers

#### `retry_utils.py`
- **Functions**:
  - `jittered_backoff(attempt, base_delay)` - Exponential backoff with jitter

#### `smart_model_routing.py`
- **Purpose**: Auto-select best model based on task
- **Strategy**: Cost, latency, availability, capability matching

#### `anthropic_adapter.py`
- **Purpose**: Anthropic Claude API integration
- **Handles**: Prompt caching, vision, thinking models

#### `auxiliary_client.py`
- **Purpose**: OpenAI and third-party client handling

#### Other modules:
- `display.py` - Terminal formatting, spinners, tool previews
- `title_generator.py` - Auto-generate session titles
- `insights.py` - Usage analytics
- `redact.py` - Redact sensitive info from logs
- `rate_limit_tracker.py` - Track rate limits per provider
- `subdirectory_hints.py` - Hint file context structure
- `prompt_caching.py` - Anthropic cache control
- `context_references.py` - Manage .hermes.md references
- `copilot_acp_client.py` - Copilot ACP integration

---

### 3.2 TOOLS MODULE (`tools/`)
**Purpose**: Extensible tool system with 53+ implementations

#### `registry.py`
- **Class**: `ToolRegistry` (singleton)
- **Responsibility**: Central tool registration and discovery
- **Data Structure**: `ToolEntry(name, toolset, schema, handler, check_fn, ...)`
- **Key Methods**:
  - `register()` - Register a tool at module import
  - `get_tool_definitions()` - Filtered list (by toolset, disabled)
  - `dispatch(tool_name, args)` - Execute tool handler
  - `get_tool_to_toolset_map()` - Mapping for backward compat
- **Safety**: Checks availability before execution

#### Tool Categories:

**Web Tools** (`web_tools.py`):
- `web_search` - Search engine queries
- `web_extract` - Content extraction from URLs

**Terminal/Process** (`terminal_tool.py`):
- `terminal` - Execute shell commands
- Supports 6 backends: local, docker, SSH, Modal, Daytona, Singularity
- **Environments Module** (`tools/environments/`):
  - Base interface for all backends
  - Each backend handles sandbox isolation, timeouts, resource limits
  - Persistent sessions with state

**File Operations** (`file_tools.py`):
- `read_file` - Read files
- `write_file` - Create/append files
- `patch` - Apply unified diffs
- `search_files` - Grep-like search

**Browser Automation** (`browser_tool.py`):
- `browser_navigate` - Load URL
- `browser_snapshot` - Screenshot
- `browser_click`, `browser_type`, `browser_scroll` - Interactions
- `browser_vision` - Analyze page elements
- `browser_console` - Execute JavaScript
- **Browser Providers** (`tools/browser_providers/`):
  - Playwright (local)
  - Browserbase (cloud)
  - Firecrawl (web scraping)
  - BrowserUse (agent-controlled)
  - CamoFox (anti-detection)

**Vision** (`vision_tools.py`):
- `vision_analyze` - Image understanding

**Code & Execution** (`code_execution_tool.py`):
- `execute_code` - Run Python in sandbox
- Uses execution environments (local, docker, modal, etc.)

**Skills** (`skills_tool.py`):
- `skills_list` - List available skills
- `skill_view` - View skill content
- `skill_manage` - Create/update skills

**Memory** (`memory_tool.py`):
- Delegates to `MemoryManager.handle_tool_call()`

**Session Search** (`session_search_tool.py`):
- `session_search` - FTS5 search across all sessions
- Uses `HermesState.search_sessions_fts()`

**Planning** (`todo_tool.py`):
- `todo` - Create/manage/complete tasks

**Messaging** (`send_message_tool.py`):
- `send_message` - Cross-platform message sending (via gateway)

**Delegation** (`delegate_tool.py`):
- `delegate_task` - Spawn isolated subagent
- Runs in ThreadPoolExecutor with dedicated event loop
- Returns result or error to parent agent

**Image Generation** (`image_generation_tool.py`):
- Providers: Fal, Replicate, local Stable Diffusion

**Text-to-Speech** (`tts_tool.py`):
- Providers: Google TTS, local TTS, NeuTTS, OpenAI TTS

**Cron Jobs** (`cronjob_tools.py`):
- Schedule & manage cron jobs
- Stored in SQLite, executed by daemon

**Home Assistant** (`homeassistant_tool.py`):
- `ha_list_entities`, `ha_get_state`, `ha_call_service`
- Requires `HASS_TOKEN` and `HASS_URL`

**Mixture of Agents** (`mixture_of_agents_tool.py`):
- `mixture_of_agents` - Ensemble LLM calls

**MCP Tool** (`mcp_tool.py`):
- Dynamically load tools from external MCP servers
- Discovery via `discover_mcp_tools()`

**Clarification** (`clarify_tool.py`):
- `clarify` - Interactive refinement questions

**Approval** (`approval.py`):
- Command approval for sensitive operations

**Process Registry** (`process_registry.py`):
- Track running processes, manage PIDs

---

### 3.3 CLI MODULE (`hermes_cli/`)
**Purpose**: Terminal user interface and configuration

#### Key Modules:

**`main.py`**:
- Entry point for `hermes` command
- Argument parsing via `fire.Fire()`
- Routes to subcommands (model, tools, config, gateway, etc.)

**`banner.py`**:
- ASCII art rendering
- Version/context display

**`commands.py`**:
- Slash command handlers
- Examples: `/new`, `/reset`, `/model`, `/skills`, `/retry`

**`config.py`**:
- Load config from `~/.hermes/config.yaml`
- Merge with CLI defaults
- Support for environment variable overrides

**`auth.py`**, **`auth_commands.py`**:
- OAuth flows for platforms
- Credential management

**`model_switch.py`**:
- Interactive model selection
- Provider discovery (OpenRouter, OpenAI, Anthropic, z.ai, etc.)

**`doctor.py`**:
- System health check
- Dependency validation
- Configuration diagnostics

**`gateway.py`**:
- Gateway setup wizard
- Platform credential configuration
- Daemon management

**`curses_ui.py`**:
- Terminal UI components

**`plugins.py`**:
- Discover and load plugins from:
  - `~/.hermes/plugins/` (user)
  - Project `./.hermes/plugins/`
  - Pip-installed packages

**`cron.py`**:
- Cron job management commands

---

### 3.4 GATEWAY MODULE (`gateway/`)
**Purpose**: Multi-platform messaging interface

#### Key Components:

**`run.py`**:
- Gateway orchestrator
- Manages platform connectors
- Routes messages to/from agents

**`session.py`**:
- Per-user session state in gateway
- Bridges platform user → agent task

**`platforms/`**:
- Handler for each platform:
  - `telegram.py` - Telegram Bot API
  - `discord.py` - Discord.py
  - `slack.py` - Slack Bolt
  - `whatsapp.py` - WhatsApp Business API
  - `signal.py` - Signal CLI
  - `email.py` - Email via SMTP/IMAP
  - `matrix.py` - Matrix.org

**`delivery.py`**:
- Outbound message delivery
- Handling long responses (chunking, threading)

**`pairing.py`**:
- DM pairing for command approvals
- User linking across platforms

**`hooks.py`**:
- Event-driven system
- Examples: `before_tool_call`, `after_tool_call`
- Built-in hooks: logging, analytics

---

### 3.5 STATE PERSISTENCE (`hermes_state.py`)
**SQLite Schema**:

```
sessions:
  - id (PK)
  - source (CLI, telegram, discord, etc.)
  - user_id
  - model, model_config
  - system_prompt
  - parent_session_id (chain on compression)
  - started_at, ended_at
  - message_count, tool_call_count
  - input/output/cache/reasoning tokens
  - cost tracking
  - title

messages:
  - id (PK)
  - session_id (FK)
  - role (user/assistant)
  - content, tool_calls, tool_name
  - timestamp
  - token_count, finish_reason
  - reasoning (Claude thinking)

messages_fts (virtual table):
  - Full-text search index on content
```

**Key Methods**:
- `create_session()` - Start new conversation
- `save_message()` - Persist message
- `search_sessions_fts()` - FTS5 search
- `end_session()` - Close session, store cost

---

### 3.6 TOOL DISCOVERY & ORCHESTRATION (`model_tools.py`)
**Entry Points**:

```python
get_tool_definitions(enabled_toolsets, disabled_toolsets) -> list[dict]
  # Returns OpenAI-compatible tool schemas

handle_function_call(tool_name, args, task_id, user_task) -> str
  # Execute tool, return result/error

get_toolset_for_tool(tool_name) -> str
  # Reverse lookup

check_toolset_requirements() -> dict
  # Check availability of toolset dependencies
```

**Async Bridging**:
- Persistent event loop to avoid "Event loop is closed" errors
- Per-thread loops for parallel execution
- Wrapper: `_run_async(coro)` handles all contexts

---

## 4. ARCHITECTURE & DATA FLOW

### 4.1 Conversation Loop Flow

```
CLI Input / Gateway Message
    ↓
cli.py / gateway/run.py
    ↓
run_agent.py: AIAgent.run_conversation(user_input)
    ↓
┌─────────────────────────────────────────────────────────┐
│ 1. SETUP TURN                                           │
│    - Load session from hermes_state.db                 │
│    - Retrieve memory via MemoryManager.prefetch_all() │
│    - Build system prompt (identity + memory + skills) │
│    - Apply context compression if needed              │
│    - Load .hermes.md, AGENTS.md, SOUL.md             │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 2. API CALL                                             │
│    - Assemble messages: [system, history..., user]    │
│    - Call LLM (OpenRouter, OpenAI, Anthropic, etc.)  │
│    - Stream response to terminal/platform             │
│    - Apply Anthropic prompt caching                   │
│    - Track tokens & estimated cost                    │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 3. TOOL CALLING LOOP (if LLM requests tool calls)      │
│    For each tool_call in response:                     │
│      - Look up tool in tools.registry                 │
│      - Check availability (ToolEntry.check_fn)       │
│      - Execute handler (sync or async):               │
│        * Terminal: spawn in configured environment    │
│        * Browser: interact via provider               │
│        * File: read/write from disk                   │
│        * Memory: delegate to MemoryManager            │
│        * Skills: load from ~/.hermes/skills/          │
│        * Delegate: spawn ThreadPoolExecutor subagent │
│      - Capture result (max chars, formatted)         │
│      - Add tool_result to messages                    │
│    Continue to API call (step 2) if needed          │
└─────────────────────────────────────────────────────────┘
    ↓
┌─────────────────────────────────────────────────────────┐
│ 4. COMPLETION & PERSISTENCE                             │
│    - Save conversation to hermes_state.db             │
│    - Sync memory via MemoryManager.sync_all()         │
│    - Queue prefetch for next turn                     │
│    - Cleanup resources (browser, subagents, etc.)     │
└─────────────────────────────────────────────────────────┘
    ↓
Return final assistant_response to CLI / Gateway
    ↓
Display / Send to platform
```

### 4.2 Tool Execution Path

```
model_tools.handle_function_call(tool_name, args)
    ↓
tools.registry.dispatch(tool_name, args)
    ↓
ToolEntry lookup
    ↓
Is available? (ToolEntry.check_fn)
    ├─ No → Return error
    └─ Yes ↓
        Call ToolEntry.handler(args)
        ├─ If async: await via _run_async()
        └─ If sync: direct call
        ↓
        Format result or error
        ↓
        Return result_string
```

### 4.3 Memory Flow

```
User input → MemoryManager.prefetch_all()
            ↓
    For each provider:
      - Call provider.prefetch(user_input)
      - Retrieve relevant facts
    ↓
    Wrap in <memory-context> fence
    ↓
    Inject into system prompt

                After tool execution:
                ↓
    MemoryManager.sync_all(user_msg, response)
    ↓
    For each provider:
      - Call provider.sync(user_input, response)
      - Decide what to save
      - Update persistent storage
```

### 4.4 Skill System

```
Skill discovery:
  ~/.hermes/skills/ (user)
      ↓
  .hermes/skills/ (project)
      ↓
  skills/ (bundled)

Each skill:
  - SKILL.md (required)
  - Optional frontmatter: metadata, conditions
  - Markdown body: instructions
  - Conditional visibility (platform, tool availability)

During conversation:
  - Agent can call skill_view / skill_manage
  - Or directly execute via function (rare)
  - Learned skills auto-created in ~/.hermes/skills/
```

### 4.5 Platform Gateway Flow

```
User Message (Telegram/Discord/Slack/etc.)
    ↓
gateway/platforms/[platform].py
    ↓
gateway/session.py: create or get session
    ↓
run_agent.py: AIAgent.run_conversation()
    ↓
[Tool execution, API calls, etc.]
    ↓
gateway/delivery.py: format and send response
    ↓
Back to platform
```

---

## 5. MODULE DEPENDENCIES & IMPORT GRAPH

### Top-Level Imports
```
cli.py
  ├─ run_agent.py (AIAgent)
  ├─ model_tools.py (get_tool_definitions, handle_function_call)
  ├─ hermes_state.py (HermesState)
  ├─ hermes_cli/ (config, auth, commands, etc.)
  └─ toolsets.py (resolve_toolset)

run_agent.py
  ├─ agent/ (memory, prompt, display, error_classifier, etc.)
  ├─ model_tools.py
  ├─ tools/registry.py
  ├─ hermes_state.py
  ├─ toolsets.py
  └─ openai.Client (OpenRouter, OpenAI, Anthropic, etc.)

model_tools.py
  ├─ tools/registry.py
  ├─ tools/*.py (all tool modules)
  └─ toolsets.py

tools/registry.py
  (Standalone, no tool imports - circular protection)

gateway/run.py
  ├─ run_agent.py
  ├─ hermes_state.py
  ├─ gateway/platforms/
  ├─ gateway/delivery.py
  └─ agent/

hermes_cli/main.py
  ├─ cli.py
  ├─ hermes_cli/commands.py
  ├─ hermes_cli/config.py
  └─ hermes_cli/auth.py
```

---

## 6. CONFIGURATION FILES & EXAMPLES

### 6.1 Configuration (`~/.hermes/config.yaml`)
```yaml
model: provider:model  # e.g., openrouter:nous-hermes-2
provider_config:
  openrouter:
    api_key: sk-...
    base_url: https://openrouter.ai/api/v1
  openai:
    api_key: sk-...

tools:
  enabled: [web, terminal, file, vision, browser]
  disabled: []

skills:
  enabled: true
  auto_create: true
  search: true

memory:
  provider: builtin  # or honcho, etc.
  enabled: true

gateway:
  telegram:
    token: 123:ABC...
  discord:
    token: MzM...
  slack:
    token: xoxb-...
```

### 6.2 CLI Config (`./cli-config.yaml`)
Project-specific defaults, same format as above.

### 6.3 SOUL.md
User/project personality override. Markdown file in project root.
Scanned for prompt injection before injection.

### 6.4 .hermes.md / AGENTS.md
Context files automatically discovered and injected into system prompt.
Format: Markdown, YAML frontmatter optional.

---

## 7. CONFIGURATION & ENVIRONMENT

### 7.1 Environment Variables
```
HERMES_HOME           # ~/.hermes (default)
HERMES_QUIET          # Suppress startup messages
OPENROUTER_API_KEY    # OpenRouter key
OPENAI_API_KEY        # OpenAI key
ANTHROPIC_API_KEY     # Anthropic key
HASS_TOKEN, HASS_URL  # Home Assistant
MCP_SERVERS           # MCP server config (JSON)
```

### 7.2 Paths
```
~/.hermes/
  ├─ config.yaml           # Configuration
  ├─ state.db              # SQLite sessions, messages
  ├─ memory.json           # Built-in memory
  ├─ skills/               # Learned skills
  ├─ plugins/              # User plugins
  ├─ credentials/          # Encrypted API keys
  ├─ logs/                 # Activity logs
  └─ .env                  # Environment overrides
```

---

## 8. KEY DESIGN PATTERNS

### 8.1 Tool Registry Pattern
- **Central Registry**: `tools/registry.py` (singleton)
- **Self-Registration**: Each tool calls `registry.register()` at import
- **Circular Protection**: `registry.py` imports nothing from tool modules
- **Late Discovery**: `model_tools.py` imports all tools, then queries registry
- **Lazy Availability**: Check functions determine if tool can run

### 8.2 Memory Provider Pattern
- **Abstract Base**: `agent/memory_provider.py`
- **Single External**: Only 1 non-builtin provider allowed (config)
- **Built-in Default**: Simple JSON fallback always present
- **Unified Interface**: `MemoryManager` coordinates all calls
- **Prefetch/Sync**: Memory recalled before turn, saved after

### 8.3 Execution Environment Pattern
- **Base Interface**: `tools/environments/base.py`
- **Pluggable Backends**: Local, Docker, SSH, Modal, Daytona, Singularity
- **Isolation**: Each environment sandboxes tool execution
- **Persistence**: Sessions can be long-lived across API calls

### 8.4 Async Bridging Pattern
- **Persistent Loops**: Per-thread event loops, never closed
- **Context Detection**: Detect if already in async context
- **Thread-Safe**: Worker threads get their own loops
- **Client Safety**: Cached httpx/AsyncOpenAI stay bound to live loop

### 8.5 Prompt Injection Defense
- **Context Scanning**: `_scan_context_content()` checks SOUL.md, .hermes.md
- **Pattern Detection**: Regex for common injection techniques
- **Invisible Unicode Check**: Detect zero-width characters
- **Safe Fallback**: Return placeholder if injection detected

---

## 9. NOTABLE FEATURES & CAPABILITIES

### 9.1 Multi-Provider Model Support
- **OpenRouter** (200+ models): Hermes, Llama, GPT, Claude, etc.
- **OpenAI**: GPT-4, o1, etc.
- **Anthropic**: Claude 3.5, with prompt caching & thinking
- **Custom Endpoints**: Ollama, vLLM, etc.
- **Smart Routing**: Auto-select best model per task

### 9.2 Terminal Execution Backends
1. **Local**: Direct shell on host
2. **Docker**: Containerized execution
3. **SSH**: Remote execution
4. **Modal**: Serverless GPU/CPU
5. **Daytona**: IDE environment
6. **Singularity**: HPC containers

### 9.3 Browser Automation
- **Playwright**: Local browser control
- **Browserbase**: Cloud-hosted browser
- **Firecrawl**: Web scraping with AI
- **BrowserUse**: Agentic browser control
- **CamoFox**: Anti-detection features

### 9.4 Skills System (Procedural Memory)
- **Learned Skills**: Auto-created from complex tasks
- **Self-Improvement**: Skills refine during use
- **Search**: FTS5 skill discovery
- **Platform Conditions**: Skills gated by platform/tools
- **Open Standard**: Compatible with agentskills.io

### 9.5 Memory System
- **Built-in**: Simple JSON-based storage
- **Honcho Plugin**: Dialectic user modeling
- **Recalled**: Injected as context before each turn
- **Saved**: Async update after each turn
- **Compact**: Pruned to avoid context bloat

### 9.6 RL Training Infrastructure
- **Trajectory Compression**: Reduce RL datasets
- **Atropos Environments**: Interactive training
- **Batch Runner**: Generate trajectories at scale
- **Mini-SWE**: Software engineering-specific runner

### 9.7 Multi-Platform Gateway
- **Telegram**: Full bot with command approval
- **Discord**: Slash commands, threads
- **Slack**: Slash commands, file upload
- **WhatsApp**: Business API integration
- **Signal**: End-to-end encrypted
- **Email**: SMTP/IMAP integration
- **Single Daemon**: Unified gateway process

---

## 10. ERROR HANDLING & RESILIENCE

### 10.1 API Error Classification
- **context_limit**: Compress or truncate
- **rate_limit**: Exponential backoff with jitter
- **auth_error**: Prompt for credentials
- **service_error**: Retry with backoff
- **model_unavailable**: Failover to alternative

### 10.2 Retry Strategy
- **Exponential Backoff**: Base 2^attempt, capped at 10 attempts
- **Jitter**: Randomized delay to avoid thundering herd
- **Per-Provider**: Track rate limits per API
- **Smart Failover**: Route to alternative provider if available

### 10.3 Tool Execution Safety
- **Availability Check**: `check_fn` before execution
- **Timeout**: Per-tool and per-environment limits
- **Result Truncation**: Max chars per tool result
- **Error Formatting**: User-friendly error messages
- **Cleanup**: Auto-cleanup resources (browsers, processes, etc.)

---

## 11. TESTING & DEVELOPMENT

### Test Structure (`tests/`)
- Unit tests for core modules
- Integration tests for tool execution
- Mock API clients for offline testing
- RL environment tests

### Development Tools
- **Nix**: `flake.nix` for reproducible environment
- **Docker**: Multi-stage builds for deployment
- **Scripts**: `setup-hermes.sh` installation script
- **CI/CD**: GitHub Actions workflows

---

## 12. PERFORMANCE OPTIMIZATIONS

### 12.1 Token Estimation
- **Rough Estimation**: Fast pre-flight check
- **Accurate Estimation**: Per-model token counters
- **Caching**: Cache model metadata (context limits, costs)

### 12.2 Prompt Caching
- **Anthropic Cache Control**: Mark stable context for reuse
- **Cost Savings**: 90% discount on cached tokens

### 12.3 Context Compression
- **Oldest-First Pruning**: Drop oldest messages first
- **Tool Call Preservation**: Keep recent tool interactions
- **Smart Truncation**: Drop least relevant messages

### 12.4 Async Execution
- **Parallel Tools**: Multiple tools run concurrently
- **Non-Blocking I/O**: Async file, network operations
- **Persistent Loops**: Avoid event loop creation overhead

---

## 13. SECURITY CONSIDERATIONS

### 13.1 Credential Management
- **Environment Variables**: First source of truth
- **Credential Files**: Secure file-based storage
- **OAuth Flows**: Browser-based auth for platforms
- **Secret Managers**: Integration with system keychains

### 13.2 Command Approval
- **Interactive Confirmation**: For sensitive commands
- **DM Pairing**: Cross-platform approval via gateway
- **Audit Logging**: All commands logged to state.db

### 13.3 Sandbox Isolation
- **Terminal Execution**: Containerized per environment
- **File Operations**: Respect user-defined base paths
- **Memory Injection**: Fenced context to prevent model confusion

### 13.4 Prompt Injection Defense
- **Context Scanning**: Detect malicious patterns in .hermes.md, SOUL.md
- **Invisible Character Detection**: Unicode tricks blocked
- **Safe Fallback**: Return placeholder if injection suspected

---

## 14. EXTENSION POINTS

### 14.1 Custom Tools
1. Create `~/.hermes/plugins/my_tool.py`
2. Import `tools.registry.register`
3. Define schema, handler, check_fn
4. Call `registry.register(name, toolset, schema, handler, check_fn, ...)`
5. Reload: `hermes doctor --reload`

### 14.2 Custom Memory Provider
1. Inherit `agent.memory_provider.MemoryProvider`
2. Implement `get_tool_schemas()`, `prefetch()`, `sync()`, `handle_tool_call()`
3. Install as pip package or place in `~/.hermes/plugins/`
4. Configure in `config.yaml`: `memory.provider: honcho`

### 14.3 Custom Skills
1. Create `~/.hermes/skills/my-skill/SKILL.md`
2. Optional: Add frontmatter for metadata
3. Write markdown instructions
4. Conditions (platform, tools) optional in frontmatter
5. Auto-discovered on next restart

### 14.4 Gateway Platform
1. Inherit `gateway.platforms.base.Platform`
2. Implement `async send_message()`, `async receive_messages()`
3. Register in `gateway/platforms/__init__.py`
4. Add config in `hermes_cli/gateway.py` setup wizard

### 14.5 MCP Servers
1. Define MCP server (separate process)
2. Configure in `~/.hermes/config.yaml` under `mcp_servers`
3. Hermes auto-discovers tools via MCP protocol
4. Tools integrated alongside native tools

---

## 15. SUMMARY TABLE

| Component | Location | Purpose | LOC |
|-----------|----------|---------|-----|
| **CLI UI** | `cli.py` | Interactive terminal | 9,185 |
| **Agent Core** | `run_agent.py` | Orchestration & loops | 9,845 |
| **Tool System** | `tools/registry.py`, `model_tools.py` | Tool discovery & execution | 2,000+ |
| **Agent Utils** | `agent/` (24 files) | Memory, prompts, errors, etc. | 5,000+ |
| **CLI Commands** | `hermes_cli/` (44 files) | Commands & setup | 8,000+ |
| **Gateway** | `gateway/` (11 files) | Multi-platform messaging | 3,000+ |
| **State** | `hermes_state.py` | SQLite persistence | 1,000+ |
| **Tools** | `tools/*.py` (53 files) | Web, terminal, browser, etc. | 15,000+ |
| **Skills** | `skills/` (28+ cats) | Procedural memory library | Varies |
| **Tests** | `tests/` (43+ files) | Unit & integration tests | 5,000+ |

**Total LOC**: ~70,000+ (excluding tests, docs, configs)

---

## 16. README CONTENTS SUMMARY

The README highlights:

1. **Self-improving agent** with built-in learning loop
2. **Terminal interface** with TUI, streaming, autocomplete
3. **Multi-platform** messaging (Telegram, Discord, Slack, WhatsApp, Signal)
4. **Closed learning loop**: Agent-curated memory, autonomous skill creation
5. **Scheduled automations**: Cron jobs, natural language workflows
6. **Parallelization**: Spawn subagents, delegate work
7. **Runs anywhere**: Local, Docker, SSH, Daytona, Modal, Singularity
8. **Research-ready**: Batch trajectory generation, RL environments
9. **Quick install**: Curl script for Linux, macOS, WSL2, Termux
10. **Documentation**: Comprehensive docs at hermes-agent.nousresearch.com/docs

---

## 17. ENTRY POINTS AT A GLANCE

```bash
hermes                    # CLI (cli.py → run_agent.py)
hermes model              # Switch model
hermes tools              # Configure toolsets
hermes config set         # Set config values
hermes gateway setup      # Setup messaging platforms
hermes gateway start      # Start daemon
hermes skills             # Browse skills
hermes memory             # Manage memory
hermes cron               # Manage scheduled jobs
hermes doctor             # Health check
hermes update             # Update version
```

All commands route through `hermes_cli/main.py` which delegates to appropriate modules.

