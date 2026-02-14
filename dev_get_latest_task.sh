#!/bin/bash

echo "Running ProcessTask workflow for latest task..."
echo "The Go program will automatically find the latest task and generate the correct task queue name."

# Run the command without any arguments - it will auto-detect everything
exec go run dev_process_task.go