```
   ___   _________  _______  ______  ____  __
  / _ | / ___/ __ \/ _ \ _ \/ __/\ \/ /\ \/ /
 / __ |/ (_ / /_/ / ___/   / _/   \  /  \  / 
/_/ |_|\___/\____/_/  /_/|_\___/  /_/   /_/  
```
> **Ultra-lightweight, single-binary Go reverse proxy for Google Antigravity with stealth anti-detection, multi-account rotation, Google AI Pro quota monitoring, Laya System-1 dynamic steering, Nano Banana 2 image generation, and a cyberpunk Web Mission Control.**

[![NixOS Flake](https://img.shields.io/badge/NixOS-Flake-blue?logo=nixos)](https://nixos.org)
[![Go Report](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

---

## 🖥️ Web Mission Control (`http://localhost:8080/dashboard`)

`agproxy` features an embedded, high-density, cyberpunk Web Mission Control dashboard with:
- **Zero AI Slop / Zero External Bloat**: Glassmorphism dark theme, JetBrains Mono typography, pure embedded HTML/CSS/JS.
- **Live 4-Bucket Quota Monitors**: Real-time progress gauges and reset timers matching the official Antigravity UI:
  - `Gemini (Flash / Pro)` (1,000 units shared bucket)
  - `Claude (Sonnet / Opus)` (1,000 units)
  - `GPT-OSS 120B (Medium)` (1,000 units)
  - `Gemini 3.1 Flash Image` (Shared Gemini bucket)
- **Active Primary & Auto-Failover Indicator**: Visual cues showing which account is active and cooldown timers for rate-limited accounts.
- **Interactive Playground**: Test prompt routing, inspect latency, model assignments, and token usage on the fly.
- **Telemetry Stream**: Live SSE logs tracking requests and Laya System-1 routing choices.

---

## ✨ Features

- **Blazing Fast & Low Memory**: Written in 100% pure Go (zero CGO). Memory footprint < 20MB. Single static executable binary.
- **Stealth & Anti-Detection (Anti-Ban)**:
  - **No `requestType: "agent"`**: Omits `requestType` on the chat path to prevent Google from returning fake `429 RESOURCE_EXHAUSTED` errors.
  - **System Prompt Sanitization**: Regex-cleans competitor branding (`Claude Code`, `OpenCode`, `x-anthropic-billing-header`).
  - **Tool Cloaking & Native Decoys**: Appends `_ide` to custom client tools and injects Antigravity's native decoy tools (`run_command`, `view_file`, `replace_file_content`, etc.) so requests match the official IDE.
  - **JSON Schema Sanitization**: Strips incompatible JSON Schema draft constraints (`$schema`, `additionalProperties`, etc.) that crash Gemini's protobuf parser with 400.
  - **Thought Signature Handling**: Caches and backfills `thoughtSignature` across turns for Gemini 3+ tool calling.
- **Bleeding-Edge Raw Research Model (`gemini-3.8-flash-raw`)**:
  - Maps to `gemini-3.8-flash-low(low)`.
  - Sets all 5 harm categories (`HATE_SPEECH`, `DANGEROUS_CONTENT`, `SEXUALLY_EXPLICIT`, `HARASSMENT`, `CIVIC_INTEGRITY`) to `BLOCK_NONE` for unrestricted research and advanced coding.
- **Multi-Account Auto-Rotation**:
  - Store multiple Google accounts in `~/.config/agproxy/accounts.json`.
  - Automatically fails over to the next account if a 429 rate limit is hit, with temporary backoff blocks.
- **Autonomous System-1 Routing (`laya`)**:
  - Dynamically routes incoming prompts using the local Laya System-1 decision engine (`http://127.0.0.1:8089/v1/systemone`).
  - Zero manual tier selection: automatically dispatches to the optimal Google model based on domain classification and difficulty scoring with sub-200ms response time and graceful fallback.
- **Live Quota Inspector**:
  - `agproxy quota` prints a terminal ASCII table with 5-hour and weekly remaining quota percentages and progress bars for all accounts.
- **Diagnostic Doctor (`agproxy doctor`)**:
  - One-click health check verifying OAuth tokens, Google Cloud upstream reachability, Laya System-1 daemon status, and local port bindings.
- **Cyberpunk Web Dashboard (`/dashboard`)**:
  - Real-time web UI showing account pools, Google AI Pro tier status, live 4-bucket quotas, and an interactive prompt playground.

---

## 📦 Supported Models

| Model ID | Upstream Model | Description |
| :--- | :--- | :--- |
| `laya` | *Dynamic Google Router* | **Autonomous System-1 Router (Multi-tier Google-only)** |
| `gemini-3.8-flash` | `gemini-3.8-flash-medium(medium)` | Gemini 3.8 Flash (Default) |
| `gemini-3.8-flash-high` | `gemini-3.8-flash-high(high)` | Gemini 3.8 Flash High reasoning |
| `gemini-3.8-flash-medium` | `gemini-3.8-flash-medium(medium)` | Gemini 3.8 Flash Medium |
| `gemini-3.8-flash-low` | `gemini-3.8-flash-low(low)` | Gemini 3.8 Flash Low |
| `gemini-3.8-flash-raw` | `gemini-3.8-flash-low(low)` | **Research profile (Safety `BLOCK_NONE`)** |
| `gemini-3.1-pro` | `gemini-3.1-pro` | Gemini 3.1 Pro (Deep Math & High Complexity) |
| `gemini-3.7-flash` | `gemini-3.7-flash-tiered(medium)` | Gemini 3.7 Flash |
| `gemini-3.7-flash-high` | `gemini-3.7-flash-tiered(high)` | Gemini 3.7 Flash High |
| `claude-sonnet-4-6` | `claude-sonnet-4-6` | Claude Sonnet 4.6 (Thinking) |
| `claude-opus-4-6-thinking`| `claude-opus-4-6-thinking` | Claude Opus 4.6 (Thinking) |
| `gpt-oss-120b-medium` | `gpt-oss-120b-medium` | GPT-OSS 120B |

---

## 🧠 Laya Autonomous System-1 Router

When setting `"model": "laya"`, `agproxy` performs real-time System-1 evaluation via the local Laya daemon (`http://127.0.0.1:8089`) before proxying to upstream Google Antigravity endpoints:

| Domain & Difficulty Criteria | Target Google Model | Rationale |
| :--- | :--- | :--- |
| `diff >= 2.5` OR `domain == "math_or_logic"` | `gemini-3.1-pro` | Deep mathematical reasoning, formal logic, and complex theorem verification |
| `(domain == "code" && diff >= 1.6)` OR `diff >= 2.0` | `gemini-3.8-flash-high` | Multi-step architecture, heavy coding, complex debugging |
| `diff < 0.9` AND (`domain == "chitchat"` OR `domain == "factual_lookup"`) | `gemini-3.8-flash-low` | Trivial lookups, greetings, quick conversational replies |
| *All other standard / balanced queries* | `gemini-3.8-flash-medium` | Balanced general tasks (also serves as instant fallback if Laya daemon is offline) |

- **Sub-200ms Decision Overhead**: Queries Laya with a tight 200ms timeout budget.
- **Fail-Safe Fallback**: If the Laya service is offline or times out, seamlessly falls back to `gemini-3.8-flash-medium` without dropping the user request.

---

## 🛠️ Getting Started

### 1. Build & Run with Nix
```bash
# Enter devShell
nix develop

# Or build the binary directly
CGO_ENABLED=0 go build -o agproxy ./cmd/agproxy
```

### 2. Log in with Google Antigravity
```bash
./agproxy login
```
Follow the URL in your terminal to authenticate. The CLI will automatically:
1. Complete OAuth token exchange.
2. Call `loadCodeAssist` to discover your Companion Project ID and Tier.
3. Call `onboardUser` to provision the project.
4. Save the account to `~/.config/agproxy/accounts.json`.

*(Repeat `./agproxy login` to add multiple Google accounts for rotation).*

### 3. Check Live Quota
```bash
./agproxy quota
```

### 4. Start the Server
```bash
./agproxy serve --port 8080 --api-key sk-my-secret-key
```
Or set `AGPROXY_API_KEY`:
```bash
export AGPROXY_API_KEY="sk-my-secret-key"
./agproxy serve --port 8080
```
Server starts on `http://127.0.0.1:8080`. When `--api-key` or `AGPROXY_API_KEY` is provided, all requests must supply `Authorization: Bearer <key>`. If not set, requests are accepted without key validation.

### Flake Run
```bash
nix run github:surtr85/agproxy -- serve --port 8080
```

---

## 🔌 Editor & Client Configuration

Point any OpenAI-compatible client to `agproxy`:

### Cursor / Continue / Cline / Zed
- **Base URL**: `http://localhost:8080/v1`
- **API Key**: `sk-antigravity` (any string)
- **Model**: `laya` (for auto-routing), `gemini-3.8-flash-raw`, or `claude-sonnet-4-6`

### cURL Test
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "laya",
    "messages": [{"role": "user", "content": "Explain theoretical wormholes in detail"}],
    "stream": true
  }'
```
