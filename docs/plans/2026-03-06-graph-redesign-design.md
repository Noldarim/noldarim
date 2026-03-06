# Graph View Redesign: Vertical Spine + Semantic Zoom

**Date**: 2026-03-06
**Status**: Approved

## Problem

The current graph view becomes unreadable as complexity grows:
- Y-axis doesn't respect temporal ordering (commits at row 0, pipelines above).
- All runs always expanded into step chains creates massive horizontal spread.
- Zoomed-out view lacks clarity — no visual hierarchy between overview and detail.
- Commit timeline has no clear spine structure.

## Decision

Evolve the existing custom layout engine (Approach A) rather than adopting elkjs or other layout libraries. The custom layout is domain-aware, zero-dependency, and already handles all required layout patterns. The redesign changes orientation and adds semantic zoom.

## Design

### 1. Orientation: Vertical Spine + Horizontal Branches

**Current**: Time flows left-to-right (X = depth via Kahn's algorithm), pipelines stack vertically (Y = row).

**New**: Time flows top-to-bottom (Y = temporal position). Pipelines branch rightward (X = horizontal lane).

```
Current:                          New:
c1 -- c2 -- c3                   main (lane 0)
|           |                      |
[pipeA]   [pipeB]                  c1 -- [pipeline A steps] -->
|           |                      |
*           *                      c2 -- [pipeline B steps] -->
                                   |
                                   c3
```

Coordinate mapping:
- `y = ORIGIN_Y + timeSlot * ROW_GAP` (time axis, top-to-bottom)
- `x = ORIGIN_X + lane * COLUMN_GAP` (pipeline axis, left-to-right)

**Vertical spine (lane 0)**: All main-branch commits sit in lane 0, ordered top-to-bottom by commit time. Vertical timeline edges connect consecutive commits.

**Pipeline lanes (lane 1+)**: Each pipeline run branches rightward from its source commit. Step chains extend horizontally. Pipeline Y is anchored to its source commit's Y on the spine.

**Bidirectional pipelines**: Execution pipelines extend rightward. Merge/promote pipelines extend leftward (steps flow right-to-left, bringing changes back to the spine).

**Fork handling**: Forks branch from their specific divergence step node, extending further right. The forked pipeline occupies the next available Y slot below the parent pipeline. Forks use the same layout logic as new pipelines.

**Merge-back**: When a pipeline's head commit is promoted to main, the promote pipeline (leftward) runs from the branch head back to the spine.

### 2. Semantic Zoom (Level of Detail)

Three zoom levels with automatic transitions:

**LOD 1 — Overview (zoom < 0.4)**:
- Spine commits: small dots with abbreviated SHA.
- Pipelines: collapsed to a mini progress bar (colored segments for completed/running/failed, step count like `4/6`).
- Only spine edges and source-to-progress-bar connections visible.
- No patch/outcome nodes, no step edges.

**LOD 2 — Standard (0.4 <= zoom < 0.8)**:
- Spine commits: full nodes with SHA + message.
- Pipelines: still collapsed progress bars, slightly larger, showing current step name.
- Spine edges + pipeline connection edges.

**LOD 3 — Detail (zoom >= 0.8)**:
- Full expansion: pipeline step chains with patch + outcome nodes.
- All edges visible: step edges, connector edges, fork edges.
- Current behavior — the fully expanded view.

**Mechanism**:
- `onViewportChange` callback tracks zoom level via ref (no re-render per tick).
- LOD threshold crossing triggers `buildProjectGraph()` with different `expandedRunIds`:
  - LOD 1-2: empty (all collapsed)
  - LOD 3: all run IDs (all expanded)
- Viewport center maintained across LOD transitions.

### 3. Layout Algorithm Changes

**`computeTimeSlots()`** (replaces `computeDepths()` for spine):
- Sort main commits by commit timestamp (git log order as fallback).
- Assign sequential time slots: slot 0 = oldest, slot N = newest.
- Pipeline source commits inherit time slot from spine.
- Insert vertical space between spine commits where pipelines are attached.

**`assignLanes()`** (replaces `assignPipelineRows()`):
- Lane 0 = main spine (all main commits).
- Lane 1+ = pipeline runs, assigned in creation order.
- Forked pipelines get next available lane.
- Step chain sub-positions: each step pair occupies incrementing horizontal positions.

**Dynamic vertical spacing**:
- Second pass after base time slots: insert additional Y space between spine commits where pipelines branch.
- Collapsed: 1 row of space (progress bar beside commit).
- Expanded: space proportional to pipeline lanes attached.

### 4. Edge Routing

Three principles: **right angles only** for direction changes, **no collisions** with nodes, **simplest possible path**.

- **Spine edges**: Vertical straight lines downward between consecutive commits.
- **Pipeline start edges**: Straight horizontal right from source node to first step.
- **Step edges**: Straight horizontal in pipeline's direction (right for execution, left for merge).
- **Fork edges**: From fork point, go up then right (right-angle bend).
- **Leftward pipelines (merge/promote)**: Steps flow right-to-left. Same routing rules, reversed direction.
- **Collision avoidance**: When path crosses a node, add extra right-angle bend to route around.

### 5. New & Updated Components

**`PipelineSummaryNode` (new)**:
- Rendered at LOD 1-2 (zoomed out).
- Shows: pipeline name, colored progress segments, step count, status icon.

**`CommitNode` (updated)**:
- Main-spine variant for lane 0 commits: vertical handles (top/bottom) + right handle for pipeline branching.
- LOD 1: small dot with label.
- LOD 2-3: full detail (existing behavior).

**`PipelineEdge` (updated)**:
- `getSmoothStepPath` with `borderRadius` for right-angle bends.
- Support leftward direction for merge pipelines.

**`PipelineBgNode` (updated)**:
- Reoriented: horizontal background spanning step chain.
- Y-anchored to source commit's time slot, X spans rightward.

### 6. Data Flow & State

- New state: `lodLevel: 1 | 2 | 3` derived from `viewport.zoom`.
- LOD change triggers graph rebuild with different `expandedRunIds`.
- Pipeline direction inferred from pipeline template type (execution=right, merge=left).
- All changes frontend-only — no backend API changes needed.

### 7. Testing Strategy

**Unit tests (graph-layout.ts)**:
- `computeTimeSlots()`: temporal ordering, pipeline source inheritance, spacing.
- `assignLanes()`: spine at lane 0, pipelines sequential, fork lane assignment.
- Edge routing: right-angle paths, collision avoidance, leftward merge pipelines.
- LOD switching: correct collapsed vs expanded node sets.

**Component tests**:
- `PipelineSummaryNode`: progress segments, step count, status rendering.
- `CommitNode` LOD variants: dot vs full modes.
- Existing tests updated for coordinate swap.

**Integration tests**:
- Zoom in/out triggers correct LOD transitions.
- Click collapsed pipeline expands.
- Fork branches from correct step.

## Files Affected

- `desktop/src/lib/graph-layout.ts` — major rewrite (orientation + layout algorithm)
- `desktop/src/components/NoldarimGraphView.tsx` — zoom listener + LOD state
- `desktop/src/components/nodes/CommitNode.tsx` — spine variant + LOD modes
- `desktop/src/components/nodes/PipelineBgNode.tsx` — reorientation
- `desktop/src/components/edges/PipelineEdge.tsx` — bidirectional + routing
- `desktop/src/components/nodes/PipelineSummaryNode.tsx` — new component
- `desktop/src/lib/graph-layout.test.ts` — updated + new tests
- `desktop/src/styles/_components.css` — updated styles
