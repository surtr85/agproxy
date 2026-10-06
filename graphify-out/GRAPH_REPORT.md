# Graph Report - agproxy  (2026-10-06)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 144 nodes · 272 edges · 12 communities (11 shown, 1 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 9 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `100f8750`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Community 0
- Community 1
- Community 2
- Community 3
- Community 4
- Community 5
- Community 6
- Community 7
- Community 8
- Community 9
- Community 10
- Community 11

## God Nodes (most connected - your core abstractions)
1. `Store` - 22 edges
2. `OpenAIToAntigravity()` - 19 edges
3. `Server` - 15 edges
4. `handleLogin()` - 10 edges
5. `Account` - 8 edges
6. `main()` - 7 edges
7. `EnsureValidToken()` - 6 edges
8. `FetchAccountQuota()` - 6 edges
9. `GetClientID()` - 6 edges
10. `AntigravityPart` - 6 edges

## Surprising Connections (you probably didn't know these)
- `handleLogin()` --calls--> `LoadProjectAndTier()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/project.go
- `handleLogin()` --calls--> `OnboardUser()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/auth/project.go
- `handleQuota()` --calls--> `PrintQuotaTable()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/quota/quota.go
- `main()` --calls--> `NewServer()`  [EXTRACTED]
  cmd/agproxy/main.go → internal/server/server.go
- `handleLogin()` --references--> `Store`  [EXTRACTED]
  cmd/agproxy/main.go → internal/config/config.go

## Import Cycles
- None detected.

## Communities (12 total, 1 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.15
Nodes (22): testing.T, BuildIDERequestID(), CleanJSONSchema(), cleanSchemaRecursive(), DeriveSessionID(), formatUUID(), GetThoughtSignature(), GetToolCallName() (+14 more)

### Community 1 - "Community 1"
Cohesion: 0.19
Nodes (12): handleAccounts(), handleQuota(), main(), printHelp(), sync.RWMutex, time.Time, EnsureValidToken(), GetConfigDir() (+4 more)

### Community 2 - "Community 2"
Cohesion: 0.21
Nodes (8): net/http.Flusher, net/http.Request, net/http.ResponseWriter, sync.Mutex, time.Duration, NewServer(), ParseRetryDelay(), Server

### Community 3 - "Community 3"
Cohesion: 0.28
Nodes (13): TokenResponse, UserInfo, handleLogin(), BuildAuthURL(), ExchangeCode(), FetchUserInfo(), GenerateState(), GetClientID() (+5 more)

### Community 4 - "Community 4"
Cohesion: 0.23
Nodes (9): StoreThoughtSignature(), StoreToolCallName(), NewSSEState(), TestSSEState_ConvertChunk_Text(), TestSSEState_ConvertChunk_ThinkingAndTools(), OpenAISSEChunk, OpenAISSEDelta, OpenAIToolCallItem (+1 more)

### Community 5 - "Community 5"
Cohesion: 0.35
Nodes (10): AntigravityPart, AntigravityBlob, AntigravityContent, AntigravityFnCall, AntigravityFnResp, AntigravityFunctionDecl, AntigravityRequest, AntigravityTool (+2 more)

### Community 6 - "Community 6"
Cohesion: 0.29
Nodes (7): context.Context, io.ReadCloser, net/http.Client, net/http.Header, Client, NewClient(), AntigravityRequestWrapper

### Community 7 - "Community 7"
Cohesion: 0.31
Nodes (8): AllowedTier, ClientMetadata, LoadCodeAssistRequest, LoadCodeAssistResponse, OnboardUserRequest, OnboardUserResponse, LoadProjectAndTier(), OnboardUser()

### Community 8 - "Community 8"
Cohesion: 0.36
Nodes (8): fmtQuotaPair(), PrintQuotaTable(), renderProgressBar(), truncate(), FetchAvailableModelsResponse, ModelQuotaItem, QuotaReport, RetrieveUserQuotaSummaryResponse

### Community 9 - "Community 9"
Cohesion: 0.40
Nodes (4): RouteWithLaya(), ModelInfo, ResolveModel(), LayaDecision

### Community 10 - "Community 10"
Cohesion: 0.50
Nodes (3): agproxy.nix, pkgs.buildGoModule, pkgs.mkShell

## Knowledge Gaps
- **7 isolated node(s):** `agproxy.nix`, `pkgs.buildGoModule`, `pkgs.mkShell`, `github.com/surtr85/agproxy`, `OnboardUserResponse` (+2 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **1 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `OpenAIToAntigravity()` connect `Community 0` to `Community 9`, `Community 2`?**
  _High betweenness centrality (0.349) - this node is a cross-community bridge._
- **Why does `Store` connect `Community 1` to `Community 2`, `Community 3`?**
  _High betweenness centrality (0.324) - this node is a cross-community bridge._
- **Why does `Server` connect `Community 2` to `Community 1`, `Community 6`?**
  _High betweenness centrality (0.282) - this node is a cross-community bridge._
- **Are the 5 inferred relationships involving `OpenAIToAntigravity()` (e.g. with `TestOpenAIToAntigravity_Basic()` and `TestOpenAIToAntigravity_LayaRouting()`) actually correct?**
  _`OpenAIToAntigravity()` has 5 INFERRED edges - model-reasoned connections that need verification._
- **What connects `agproxy.nix`, `pkgs.buildGoModule`, `pkgs.mkShell` to the rest of the system?**
  _7 weakly-connected nodes found - possible documentation gaps or missing edges._