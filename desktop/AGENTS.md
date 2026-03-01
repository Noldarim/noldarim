# Desktop App (Tauri + React)

## OVERVIEW
Tauri-wrapped React 19 SPA for pipeline orchestration and git graph visualization. Communicates with Go backend via REST (8080) and WebSockets.

## STRUCTURE
```
desktop/
├── src/
│   ├── App.tsx            # Routing, graph view, and dialog orchestration
│   ├── components/        # ReactFlow nodes, forms, and detail drawers
│   │   ├── nodes/         # Custom ReactFlow node types (Commit, Patch, Pipeline)
│   │   └── step-detail/   # Inspection panels for AI activity and git diffs
│   ├── state/             # Zustand stores (project-graph, run-state)
│   ├── lib/               # Core logic: API client, WS parser, graph layout
│   │   ├── graph-layout.ts # Complex DAG-to-ReactFlow positioning (1.6k LOC)
│   │   └── types.ts       # Manual mirrors of Go protocol types
│   └── sandbox/           # Dev-only interactive graph testing
└── src-tauri/             # Minimal Rust window manager (no IPC logic)
```

## WHERE TO LOOK
| Task | Location |
|------|----------|
| Modify Graph UI | `src/components/NoldarimGraphView.tsx` |
| Add Node Type | `src/components/nodes/` + `NoldarimGraphView.tsx` registry |
| Change API Call | `src/lib/api.ts` (centralized fetch) |
| Update State | `src/state/project-graph-store.ts` or `run-store.ts` |
| Fix Layout | `src/lib/graph-layout.ts` (performance sensitive) |
| New Event Type | `src/lib/ws.ts` (envelope parsing) + `lib/types.ts` |
| Test Component | `__tests__/` subdirectories using Vitest + RTL |

## CONVENTIONS
- **State**: Use Zustand stores exclusively. No React Context or Redux.
- **API**: Centralize all HTTP calls in `lib/api.ts`. Use Zod schemas in `lib/schemas.ts` for validation.
- **Types**: Manually sync `lib/types.ts` with Go protocol structs. No codegen.
- **Testing**: Vitest globals enabled. Mock `lib/api` and use factory functions (`makeRun`, `makeEvent`).
- **Layout**: Keep `graph-layout.ts` logic pure. Profile before adding heavy computations.
- **Sandbox**: Use `src/sandbox/` for isolated feature development without backend dependencies.

## ANTI-PATTERNS
- **Tauri IPC**: Don't use `invoke()`. All communication must go through HTTP/WS to `localhost:8080`.
- **Direct Fetch**: Never use raw `fetch()` in components. Use the `api` client.
- **Rust Imports**: Never import from `src-tauri/` into the React frontend.
- **Prop Drilling**: Use Zustand selectors for shared state instead of deep prop passing.
- **Stale Types**: Don't let `lib/types.ts` diverge from backend protocol definitions.
