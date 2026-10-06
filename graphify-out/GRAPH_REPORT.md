# Graph Report - agproxy  (2026-10-07)

## Corpus Check
- 22 files · ~14,598 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 174 nodes · 339 edges · 13 communities (12 shown, 1 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 12 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `c9b67ce7`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- OpenAIToAntigravity
- Store
- Server
- oauth.go
- sse.go
- Client
- types.go
- project.go
- main
- RouteWithLayaContext
- flake.nix
- github.com/surtr85/agproxy
- README.md

## God Nodes (most connected - your core abstractions)
1. `Store` - 25 edges
2. `Server` - 19 edges
3. `OpenAIToAntigravity()` - 19 edges
4. `handleLogin()` - 10 edges
5. `main()` - 9 edges
6. `EnsureValidToken()` - 9 edges
7. `Account` - 9 edges
8. `FetchAccountQuota()` - 9 edges
9. `handleImage()` - 7 edges
10. `GetClientID()` - 6 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `LoadStore()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/config/config.go
- `main()` --calls--> `NewServer()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/server/server.go
- `handleLogin()` --calls--> `GenerateState()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/oauth.go
- `handleLogin()` --calls--> `StartOAuthServer()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/oauth.go
- `handleLogin()` --calls--> `LoadProjectAndTier()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/project.go

## Import Cycles
- None detected.

## Communities (13 total, 1 thin omitted)

### Community 0 - "OpenAIToAntigravity"
Cohesion: 0.14
Nodes (22): testing.T, TestDashboardEndpoints(), BuildIDERequestID(), CleanJSONSchema(), cleanSchemaRecursive(), DeriveSessionID(), formatUUID(), GetThoughtSignature() (+14 more)

### Community 1 - "Store"
Cohesion: 0.25
Nodes (6): sync.RWMutex, time.Time, GetConfigDir(), Account, Store, LoadStore()

### Community 2 - "Server"
Cohesion: 0.22
Nodes (7): net/http.Flusher, net/http.Request, net/http.ResponseWriter, sync.Mutex, NewServer(), ParseRetryDelay(), Server

### Community 3 - "oauth.go"
Cohesion: 0.25
Nodes (14): TokenResponse, UserInfo, handleLogin(), BuildAuthURL(), ExchangeCode(), FetchUserInfo(), GenerateState(), GetClientID() (+6 more)

### Community 4 - "sse.go"
Cohesion: 0.18
Nodes (11): StoreThoughtSignature(), StoreToolCallName(), NewSSEState(), TestSSEState_ConvertChunk_Text(), TestSSEState_ConvertChunk_ThinkingAndTools(), TestSSEChunkUsageConversion(), OpenAISSEChunk, OpenAISSEDelta (+3 more)

### Community 5 - "Client"
Cohesion: 0.25
Nodes (8): context.Context, io.ReadCloser, net/http.Client, net/http.Header, Client, NewClient(), AntigravityRequestWrapper, TransformResult

### Community 6 - "types.go"
Cohesion: 0.36
Nodes (10): AntigravityPart, AntigravityBlob, AntigravityContent, AntigravityFnCall, AntigravityFnResp, AntigravityFunctionDecl, AntigravityRequest, AntigravityTool (+2 more)

### Community 7 - "project.go"
Cohesion: 0.29
Nodes (10): AllowedTier, ClientMetadata, CurrentTierInfo, LoadCodeAssistRequest, LoadCodeAssistResponse, OnboardUserRequest, OnboardUserResponse, PaidTierInfo (+2 more)

### Community 8 - "main"
Cohesion: 0.21
Nodes (15): handleAccounts(), handleDoctor(), handleImage(), handleQuota(), main(), printHelp(), time.Duration, EnsureValidToken() (+7 more)

### Community 9 - "RouteWithLayaContext"
Cohesion: 0.31
Nodes (6): RouteWithLaya(), RouteWithLayaContext(), TestRouteWithLayaContext_TrivialGreeting(), ModelInfo, ResolveModel(), LayaDecision

### Community 10 - "flake.nix"
Cohesion: 0.50
Nodes (3): agproxy.nix, pkgs.buildGoModule, pkgs.mkShell

### Community 12 - "README.md"
Cohesion: 0.14
Nodes (13): 1. Build & Run with Nix, 2. Log in with Google Antigravity, 3. Check Live Quota, 4. Start the Server, cURL Test, Cursor / Continue / Cline / Zed, 🔌 Editor & Client Configuration, ✨ Features (+5 more)

## Knowledge Gaps
- **18 isolated node(s):** `agproxy.nix`, `pkgs.buildGoModule`, `pkgs.mkShell`, `github.com/surtr85/agproxy`, `OnboardUserResponse` (+13 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `OpenAIToAntigravity()` connect `OpenAIToAntigravity` to `RouteWithLayaContext`, `Server`, `Client`?**
  _High betweenness centrality (0.245) - this node is a cross-community bridge._
- **Why does `Store` connect `Store` to `main`, `Server`, `oauth.go`?**
  _High betweenness centrality (0.210) - this node is a cross-community bridge._
- **Why does `Server` connect `Server` to `Store`, `Client`?**
  _High betweenness centrality (0.187) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `OpenAIToAntigravity()` (e.g. with `TestOpenAIToAntigravity_Basic()` and `TestOpenAIToAntigravity_LayaRouting()`) actually correct?**
  _`OpenAIToAntigravity()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `agproxy.nix`, `pkgs.buildGoModule`, `pkgs.mkShell` to the rest of the system?**
  _18 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `OpenAIToAntigravity` be split into smaller, more focused modules?**
  _Cohesion score 0.14153846153846153 - nodes in this community are weakly interconnected._
- **Should `README.md` be split into smaller, more focused modules?**
  _Cohesion score 0.14285714285714285 - nodes in this community are weakly interconnected._