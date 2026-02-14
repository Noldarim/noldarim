# CreateTask Workflow State Machine Documentation

## Overview
The CreateTask workflow orchestrates the creation of a new task with all required resources. It ensures reliable, fault-tolerant task creation through idempotent activities and proper compensation logic.

## State Machine Diagram

```
┌─────────────────┐
│     Start       │
└────────┬────────┘
         │
         v
┌─────────────────┐
│ Create Task DB  │──────> [Failure] ──> End (Error)
│  (Idempotent)   │
└────────┬────────┘
         │ Task Created
         v
┌─────────────────┐
│Create Worktree  │──────> [Failure] ──> Compensate: Delete Task ──> End (Error)
│  (Idempotent)   │
└────────┬────────┘
         │ Worktree Created
         v
┌─────────────────┐
│Create Container │──────> [Failure] ──> Compensate: Remove Worktree + Delete Task ──> End (Error)
│  (Idempotent)   │
└────────┬────────┘
         │ Container Running
         v
┌─────────────────┐
│ Publish Event   │──────> [Failure] ──> Log Warning (Non-critical)
│  (Idempotent)   │
└────────┬────────┘
         │
         v
┌─────────────────┐
│    Success      │
└─────────────────┘
```

## States

### 1. Create Task DB
- **Purpose**: Persist task record in database
- **Idempotency**: 
  - Check if task with same project_id and title exists
  - If exists and created < 5 minutes ago, return existing task
  - If exists and older, generate unique title with timestamp
- **Failure Handling**: Workflow terminates with error
- **Retry Policy**: 3 attempts with exponential backoff

### 2. Create Worktree
- **Purpose**: Create git worktree for task isolation
- **Dependencies**: Requires task ID from previous state
- **Idempotency**:
  - Check if worktree exists at expected path
  - If exists with correct branch, return success
  - If exists with wrong branch, remove and recreate
  - Use filesystem locks to prevent race conditions
- **Failure Handling**: Triggers compensation (Delete Task)
- **Retry Policy**: 3 attempts with exponential backoff

### 3. Create Container
- **Purpose**: Provision Docker container for task execution
- **Dependencies**: Requires task ID and worktree path
- **Idempotency**:
  - Check if container named "task-{taskID}" exists
  - If exists and running, return existing container
  - If exists but stopped, remove and recreate
- **Failure Handling**: Triggers compensation (Remove Worktree + Delete Task)
- **Retry Policy**: 3 attempts with exponential backoff
- **Special Timeout**: Extended timeout (5 min) for image pulls

### 4. Publish Event
- **Purpose**: Notify system of task creation
- **Dependencies**: Requires task data
- **Idempotency**:
  - Use idempotency key: "task-created-{projectID}-{taskID}"
  - Track published events in memory
  - Skip if already published
- **Failure Handling**: Log warning but continue (non-critical)
- **Retry Policy**: 2 attempts only

## Compensation Activities

### Delete Task (Compensation)
- **Triggered By**: Worktree or Container creation failure
- **Idempotency**: Returns success if task doesn't exist
- **Implementation**: Hard delete from database

### Remove Worktree (Compensation)
- **Triggered By**: Container creation failure
- **Idempotency**: Returns success if worktree doesn't exist
- **Implementation**: Remove git worktree and clean references

### Stop Container (Compensation)
- **Triggered By**: Not used in main workflow (available for saga pattern)
- **Idempotency**: Check container state before stopping
- **Implementation**: Graceful shutdown (10s), then force kill

## Workflow Configuration

### Timeouts
- Workflow Execution: 24 hours
- Workflow Run: 24 hours
- Workflow Task: 10 seconds
- Activity Start-to-Close: 30 seconds (default)
- Container Creation: 5 minutes (extended for image pulls)
- Activity Heartbeat: 5 seconds

### Retry Policy
- Initial Interval: 1 second
- Backoff Coefficient: 2.0
- Maximum Interval: 1 minute
- Maximum Attempts: 3 (default), 2 (event publishing)

## Error Types

### Retryable Errors
- Database connection failures
- Network timeouts
- Docker daemon temporary unavailability
- Filesystem permission issues

### Non-Retryable Errors
- Repository not found
- Invalid input data
- Resource constraints exceeded
- Path traversal attempts

## Idempotency Guarantees

1. **Database Operations**: Use unique constraint checks and time-based deduplication
2. **Git Operations**: Filesystem locks and path existence checks
3. **Container Operations**: Name-based deduplication and state verification
4. **Event Publishing**: In-memory idempotency key tracking

## Monitoring Points

1. **Metrics to Track**:
   - Task creation success rate
   - Average time per state
   - Compensation trigger frequency
   - Retry attempt distribution

2. **Key Log Points**:
   - Workflow start/end with correlation ID
   - Each state transition
   - Compensation triggers
   - Idempotency hits

## Security Considerations

1. **Path Validation**: Validate worktree paths to prevent directory traversal
2. **Container Isolation**: Run containers with minimal privileges
3. **Resource Limits**: Enforce memory/CPU limits from configuration
4. **Input Sanitization**: Validate task titles and descriptions