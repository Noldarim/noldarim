#!/bin/bash

# Agent integration test runner for noldarim
# This script runs the agent-specific integration tests

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== noldarim Agent Integration Test Runner ===${NC}"
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to run the agent integration tests
run_agent_tests() {
    echo -e "${YELLOW}Running agent integration tests...${NC}"
    echo ""
    
    # Change to agent directory
    cd cmd/agent
    
    # Run the agent integration tests with verbose output
    go test -v -timeout 60s
    
    if [ $? -eq 0 ]; then
        echo ""
        echo -e "${GREEN}✓ All agent integration tests passed!${NC}"
    else
        echo ""
        echo -e "${RED}✗ Some agent integration tests failed${NC}"
        return 1
    fi
}

# Function to run specific test
run_specific_test() {
    local test_name=$1
    echo -e "${YELLOW}Running specific test: $test_name${NC}"
    echo ""
    
    cd cmd/agent
    go test -v -run "$test_name" -timeout 60s
    
    if [ $? -eq 0 ]; then
        echo ""
        echo -e "${GREEN}✓ Test $test_name passed!${NC}"
    else
        echo ""
        echo -e "${RED}✗ Test $test_name failed${NC}"
        return 1
    fi
}

# Help function
show_help() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  -h, --help           Show this help message"
    echo "  -t, --test <name>    Run specific test (e.g., TestOrchestratorAgentCommunication/OrchestratorToAgent)"
    echo ""
    echo "This script runs integration tests for the noldarim agent using the actual agent code."
    echo ""
    echo "Prerequisites:"
    echo "  - Go must be installed"
}

# Parse command line arguments
TEST_NAME=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -t|--test)
            TEST_NAME="$2"
            shift
            shift
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Main execution
main() {
    echo "Checking prerequisites..."
    
    # Check if required tools are installed
    if ! command_exists go; then
        echo -e "${RED}✗ Go is not installed${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Prerequisites check passed${NC}"
    echo ""
    
    # Check if we're in the right directory
    if [ ! -f "cmd/agent/main.go" ]; then
        echo -e "${RED}✗ Must be run from the noldarim project root directory${NC}"
        exit 1
    fi
    
    # Run tests
    if [ -n "$TEST_NAME" ]; then
        run_specific_test "$TEST_NAME"
    else
        run_agent_tests
    fi
}

# Run main function
main