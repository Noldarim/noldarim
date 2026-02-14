#!/bin/bash

# Test script for agent communication implementation

echo "Building agent binary..."
go build -o bin/agent cmd/agent/main.go

if [ $? -eq 0 ]; then
    echo "✓ Agent binary built successfully"
else
    echo "✗ Failed to build agent binary"
    exit 1
fi

echo "Testing agent with environment variables..."
AGENT_ID=test-agent-123 TASK_ID=test-task-456 ./bin/agent &
AGENT_PID=$!

sleep 3

echo "Testing agent startup..."
# Test that the agent starts without errors

if kill -0 $AGENT_PID 2>/dev/null; then
    echo "✓ Agent started successfully"
    kill $AGENT_PID
    wait $AGENT_PID 2>/dev/null
else
    echo "✗ Agent failed to start"
    exit 1
fi

echo "✓ All tests passed!"
echo ""
echo "To test the complete flow:"
echo "1. Start the main application: go run cmd/app/main.go"
echo "2. Create a task with environment through the TUI"
echo "3. Check logs for agent communication messages"