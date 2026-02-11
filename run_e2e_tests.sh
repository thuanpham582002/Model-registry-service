#!/bin/bash
#
# Run all E2E tests for Model Registry Service
#
# Usage:
#   ./run_e2e_tests.sh          # Run all tests
#   ./run_e2e_tests.sh sdk      # Run SDK contract tests only
#   ./run_e2e_tests.sh serving  # Run serving/traffic/metrics tests only
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}Model Registry E2E Tests${NC}"
echo -e "${YELLOW}========================================${NC}"

# Check if service is running
echo -e "\n${YELLOW}Checking service health...${NC}"
if ! curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
    echo -e "${RED}Service is not running. Start it with:${NC}"
    echo "  docker compose up -d"
    echo "  # or"
    echo "  go run cmd/server/main.go"
    exit 1
fi
echo -e "${GREEN}Service is healthy${NC}"

# Check Python
if ! command -v python3 &> /dev/null; then
    echo -e "${RED}python3 is required${NC}"
    exit 1
fi

# Install requests if needed
python3 -c "import requests" 2>/dev/null || pip3 install requests

FAILED=0

run_test() {
    local name=$1
    local script=$2

    echo -e "\n${YELLOW}========================================${NC}"
    echo -e "${YELLOW}Running: $name${NC}"
    echo -e "${YELLOW}========================================${NC}"

    if python3 "$script"; then
        echo -e "${GREEN}✓ $name PASSED${NC}"
    else
        echo -e "${RED}✗ $name FAILED${NC}"
        FAILED=1
    fi
}

case "${1:-all}" in
    sdk)
        run_test "SDK Contract Tests" "test_e2e_sdk.py"
        ;;
    serving)
        run_test "Serving/Traffic/Metrics Tests" "test_e2e_serving.py"
        ;;
    all|*)
        run_test "SDK Contract Tests" "test_e2e_sdk.py"
        run_test "Serving/Traffic/Metrics Tests" "test_e2e_serving.py"
        ;;
esac

echo -e "\n${YELLOW}========================================${NC}"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
else
    echo -e "${RED}Some tests failed!${NC}"
fi
echo -e "${YELLOW}========================================${NC}"

exit $FAILED
