#!/bin/bash

# Containerized Agent Workflow Test Script for noldarim
# This script provides a dev-test loop for ProcessTask workflow development

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Noldarim Containerized Agent Workflow Test Runner ===${NC}"
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}Checking prerequisites...${NC}"
    
    local all_good=true
    
    # Check Go
    if ! command_exists go; then
        echo -e "${RED}✗ Go is not installed${NC}"
        all_good=false
    else
        echo -e "${GREEN}✓ Go is installed${NC}"
    fi
    
    # Check Docker
    if ! command_exists docker; then
        echo -e "${RED}✗ Docker is not installed${NC}"
        all_good=false
    elif ! docker info >/dev/null 2>&1; then
        echo -e "${RED}✗ Docker daemon is not running${NC}"
        all_good=false
    else
        echo -e "${GREEN}✓ Docker is available${NC}"
    fi
    
    # Check if we're in the right directory
    if [ ! -f "cmd/agent/main.go" ]; then
        echo -e "${RED}✗ Must be run from the noldarim project root directory${NC}"
        all_good=false
    else
        echo -e "${GREEN}✓ Running from correct directory${NC}"
    fi
    
    # Check if Temporal server is running (optional warning)
    if ! nc -z localhost 7233 >/dev/null 2>&1; then
        echo -e "${YELLOW}⚠ Temporal server doesn't appear to be running on localhost:7233${NC}"
        echo -e "${YELLOW}  You may need to start it with: temporal server start-dev${NC}"
    else
        echo -e "${GREEN}✓ Temporal server is running${NC}"
    fi
    
    if [ "$all_good" = false ]; then
        echo ""
        echo -e "${RED}Prerequisites check failed. Please fix the issues above.${NC}"
        exit 1
    fi
    
    echo ""
}

# Function to build the Docker image
build_docker_image() {
    echo -e "${YELLOW}Building Docker image with agent...${NC}"
    
    if make docker-build; then
        echo -e "${GREEN}✓ Docker image 'noldarim-agent' built successfully${NC}"
    else
        echo -e "${RED}✗ Failed to build Docker image${NC}"
        exit 1
    fi
    echo ""
}

# Function to run the containerized agent test
run_containerized_test() {
    echo -e "${YELLOW}Running containerized agent workflow test...${NC}"
    echo ""
    
    # Run the specific test
    if go test -v ./internal/orchestrator/ -run "TestProcessTaskWorkflowWithContainerizedAgent" -timeout 5m; then
        echo ""
        echo -e "${GREEN}✓ Containerized agent test passed!${NC}"
    else
        echo ""
        echo -e "${RED}✗ Containerized agent test failed${NC}"
        return 1
    fi
}

# Function to run the failure test
run_failure_test() {
    echo -e "${YELLOW}Running containerized agent failure test...${NC}"
    echo ""
    
    if go test -v ./internal/orchestrator/ -run "TestProcessTaskWorkflowWithContainerizedAgentFailure" -timeout 5m; then
        echo ""
        echo -e "${GREEN}✓ Containerized agent failure test passed!${NC}"
    else
        echo ""
        echo -e "${RED}✗ Containerized agent failure test failed${NC}"
        return 1
    fi
}

# Function to run all agent-related tests
run_all_agent_tests() {
    echo -e "${YELLOW}Running all containerized agent tests...${NC}"
    echo ""
    
    if go test -v ./internal/orchestrator/ -run "TestProcessTaskWorkflowWithContainerized.*" -timeout 10m; then
        echo ""
        echo -e "${GREEN}✓ All containerized agent tests passed!${NC}"
    else
        echo ""
        echo -e "${RED}✗ Some containerized agent tests failed${NC}"
        return 1
    fi
}

# Function to clean up containers and images
cleanup() {
    echo -e "${YELLOW}Cleaning up test containers...${NC}"
    
    # Stop and remove any running test containers
    docker ps -a --filter "label=noldarim.managed=true" --format "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}" | grep -v "CONTAINER ID" || echo "No noldarim containers found"
    
    # Remove test containers
    local containers=$(docker ps -a --filter "label=noldarim.managed=true" -q)
    if [ -n "$containers" ]; then
        echo "Removing containers: $containers"
        docker rm -f $containers || echo "Failed to remove some containers"
    fi
    
    echo -e "${GREEN}✓ Cleanup completed${NC}"
}


# Help function
show_help() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  -h, --help           Show this help message"
    echo "  -b, --build-only     Only build the Docker image"
    echo "  -t, --test-only      Only run tests (skip build)"
    echo "  -f, --failure-test   Run failure test only"
    echo "  -a, --all-tests      Run all agent-related tests"
    echo "  -c, --cleanup        Clean up test containers and images"
    echo "  -s, --skip-prereqs   Skip prerequisite checks"
    echo ""
    echo "Default behavior (no options): Check prereqs, build image, run tests"
    echo ""
    echo "Development Workflow:"
    echo "  1. Make changes to ProcessTask workflow or agent code"
    echo "  2. Run: $0                    # Test the changes"
    echo "  3. Run: $0 -c                 # Clean up containers when done"
    echo ""
    echo "Prerequisites:"
    echo "  - Go must be installed"
    echo "  - Docker must be running"
    echo "  - Temporal server should be running (temporal server start-dev)"
    echo "  - Must be run from noldarim project root directory"
}

# Parse command line arguments
BUILD_ONLY=false
TEST_ONLY=false
FAILURE_TEST=false
ALL_TESTS=false
CLEANUP=false
SKIP_PREREQS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -b|--build-only)
            BUILD_ONLY=true
            shift
            ;;
        -t|--test-only)
            TEST_ONLY=true
            shift
            ;;
        -f|--failure-test)
            FAILURE_TEST=true
            shift
            ;;
        -a|--all-tests)
            ALL_TESTS=true
            shift
            ;;
        -c|--cleanup)
            CLEANUP=true
            shift
            ;;
        -s|--skip-prereqs)
            SKIP_PREREQS=true
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
    # Cleanup if requested
    if [ "$CLEANUP" = true ]; then
        cleanup
        exit 0
    fi
    
    # Check prerequisites unless skipped
    if [ "$SKIP_PREREQS" = false ]; then
        check_prerequisites
    fi
    
    # Build Docker image unless test-only
    if [ "$TEST_ONLY" = false ]; then
        build_docker_image
    fi
    
    # Run tests unless build-only
    if [ "$BUILD_ONLY" = false ]; then
        if [ "$FAILURE_TEST" = true ]; then
            run_failure_test
        elif [ "$ALL_TESTS" = true ]; then
            run_all_agent_tests
        else
            run_containerized_test
        fi
    fi
    
    echo ""
    echo -e "${GREEN}=== All operations completed successfully ===${NC}"
}

# Set up trap to cleanup on script exit
trap cleanup EXIT

# Run main function
main