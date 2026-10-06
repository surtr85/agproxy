# Graph Report - agproxy  (2026-10-07)

## Corpus Check
- 21 files · ~12,404 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 169 nodes · 326 edges · 13 communities (12 shown, 1 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 11 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `a804f238`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- OpenAIToAntigravity
- Store
- Server
- oauth.go
- testing.T
- types.go
- sse.go
- project.go
- quota.go
- RouteWithLayaContext
- flake.nix
- github.com/surtr85/agproxy
- agproxy 🚀

## God Nodes (most connected - your core abstractions)
1. `Store` - 25 edges
2. `OpenAIToAntigravity()` - 19 edges
3. `Server` - 17 edges
4. `handleLogin()` - 10 edges
5. `main()` - 9 edges
6. `EnsureValidToken()` - 9 edges
7. `Account` - 9 edges
8. `FetchAccountQuota()` - 8 edges
9. `handleImage()` - 7 edges
10. `GetClientID()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `NewServer()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/server/server.go
- `handleLogin()` --calls--> `GenerateState()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/oauth.go
- `handleLogin()` --calls--> `StartOAuthServer()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/oauth.go
- `handleLogin()` --calls--> `LoadProjectAndTier()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/project.go
- `handleLogin()` --calls--> `OnboardUser()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/project.go

## Import Cycles
- None detected.

## Communities (13 total, 1 thin omitted)

### Community 0 - "OpenAIToAntigravity"
Cohesion: 0.16
Nodes (18): BuildIDERequestID(), CleanJSONSchema(), cleanSchemaRecursive(), DeriveSessionID(), formatUUID(), GetThoughtSignature(), GetToolCallName(), SanitizePromptText() (+10 more)

### Community 1 - "Store"
Cohesion: 0.18
Nodes (14): handleAccounts(), handleDoctor(), handleImage(), handleQuota(), main(), printHelp(), sync.RWMutex, time.Time (+6 more)

### Community 2 - "Server"
Cohesion: 0.23
Nodes (7): net/http.Flusher, net/http.Request, net/http.ResponseWriter, sync.Mutex, NewServer(), ParseRetryDelay(), Server

### Community 3 - "oauth.go"
Cohesion: 0.25
Nodes (14): TokenResponse, UserInfo, handleLogin(), BuildAuthURL(), ExchangeCode(), FetchUserInfo(), GenerateState(), GetClientID() (+6 more)

### Community 4 - "testing.T"
Cohesion: 0.24
Nodes (10): testing.T, TestOpenAIToAntigravity_Basic(), TestOpenAIToAntigravity_LayaRouting(), TestOpenAIToAntigravity_StealthSanitization(), TestOpenAIToAntigravity_ToolCloaking(), TestOpenAIToAntigravity_ToolResponseResolution(), NewSSEState(), TestSSEState_ConvertChunk_Text() (+2 more)

### Community 5 - "types.go"
Cohesion: 0.16
Nodes (18): context.Context, io.ReadCloser, net/http.Client, net/http.Header, Client, NewClient(), AntigravityPart, AntigravityRequestWrapper (+10 more)

### Community 6 - "sse.go"
Cohesion: 0.70
Nodes (4): OpenAISSEChunk, OpenAISSEDelta, OpenAIToolCallItem, OpenAIUsage

### Community 7 - "project.go"
Cohesion: 0.31
Nodes (9): AllowedTier, ClientMetadata, CurrentTierInfo, LoadCodeAssistRequest, LoadCodeAssistResponse, OnboardUserRequest, OnboardUserResponse, FormatTierName() (+1 more)

### Community 8 - "quota.go"
Cohesion: 0.36
Nodes (7): time.Duration, formatTimeRemaining(), PrintQuotaTable(), renderBar(), QuotaReport, RetrieveUserQuotaSummaryResponse, SingleQuotaItem

### Community 9 - "RouteWithLayaContext"
Cohesion: 0.31
Nodes (6): RouteWithLaya(), RouteWithLayaContext(), TestRouteWithLayaContext_TrivialGreeting(), ModelInfo, ResolveModel(), LayaDecision

### Community 10 - "flake.nix"
Cohesion: 0.50
Nodes (3): agproxy.nix, pkgs.buildGoModule, pkgs.mkShell

### Community 12 - "agproxy 🚀"
Cohesion: 0.14
Nodes (13): 1. Build & Run with Nix, 2. Log in with Google Antigravity, 3. Check Live Quota, 4. Start the Server, agproxy 🚀, cURL Test, Cursor / Continue / Cline / Zed, 🔌 Editor & Client Configuration (+5 more)

## Knowledge Gaps
- **17 isolated node(s):** `agproxy.nix`, `pkgs.buildGoModule`, `pkgs.mkShell`, `github.com/surtr85/agproxy`, `OnboardUserResponse` (+12 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `OpenAIToAntigravity()` connect `OpenAIToAntigravity` to `RouteWithLayaContext`, `Server`, `testing.T`, `types.go`?**
  _High betweenness centrality (0.260) - this node is a cross-community bridge._
- **Why does `Store` connect `Store` to `Server`, `oauth.go`?**
  _High betweenness centrality (0.219) - this node is a cross-community bridge._
- **Why does `Server` connect `Server` to `Store`, `types.go`?**
  _High betweenness centrality (0.194) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `OpenAIToAntigravity()` (e.g. with `TestOpenAIToAntigravity_Basic()` and `TestOpenAIToAntigravity_LayaRouting()`) actually correct?**
  _`OpenAIToAntigravity()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `agproxy.nix`, `pkgs.buildGoModule`, `pkgs.mkShell` to the rest of the system?**
  _17 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `agproxy 🚀` be split into smaller, more focused modules?**
  _Cohesion score 0.14285714285714285 - nodes in this community are weakly interconnected._