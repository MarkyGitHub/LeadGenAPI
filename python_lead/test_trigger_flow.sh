#!/bin/bash
# test_trigger_flow.sh - Helper script to run the ngrok trigger E2E test
# 
# Usage: ./test_trigger_flow.sh [NGROK_URL]
# Example: ./test_trigger_flow.sh https://abcd1234.ngrok.io

set -e

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  ngrok Trigger Flow E2E Test${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo ""

# Get ngrok URL from argument or prompt
if [ -z "$1" ]; then
    echo -e "${YELLOW}Enter your ngrok URL (e.g., https://abcd1234.ngrok.io):${NC}"
    read -r NGROK_URL
else
    NGROK_URL=$1
fi

# Remove trailing slash if present
NGROK_URL=${NGROK_URL%/}

# Validate URL format
if [[ ! $NGROK_URL =~ ^https?:// ]]; then
    echo -e "${RED}❌ Error: Invalid URL format${NC}"
    echo "URL must start with http:// or https://"
    exit 1
fi

echo -e "${GREEN}✓${NC} Using ngrok URL: ${CYAN}$NGROK_URL${NC}"
echo ""

# Check if Django is running
echo -e "${YELLOW}Checking if Django is running...${NC}"
if ! nc -z localhost 8004 2>/dev/null && ! timeout 1 bash -c 'cat < /dev/null > /dev/tcp/localhost/8004' 2>/dev/null; then
    echo -e "${RED}❌ Django is not running on port 8004${NC}"
    echo ""
    echo "Start Django first:"
    echo "  docker-compose up -d"
    echo "  OR"
    echo "  python manage.py runserver 8004"
    exit 1
fi
echo -e "${GREEN}✓${NC} Django is running"
echo ""

# Check if database is accessible
echo -e "${YELLOW}Checking database connection...${NC}"
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}

if command -v nc >/dev/null 2>&1; then
    if ! nc -z $DB_HOST $DB_PORT 2>/dev/null; then
        echo -e "${YELLOW}⚠${NC} Database may not be accessible at $DB_HOST:$DB_PORT"
        echo "Test may fail if database is not available"
    else
        echo -e "${GREEN}✓${NC} Database is accessible"
    fi
fi
echo ""

# Check if pytest is installed
if ! command -v pytest &> /dev/null; then
    echo -e "${RED}❌ pytest is not installed${NC}"
    echo ""
    echo "Install pytest:"
    echo "  pip install pytest"
    exit 1
fi

# Check if psycopg2 is installed
if ! python -c "import psycopg2" 2>/dev/null; then
    echo -e "${YELLOW}⚠${NC} psycopg2 not installed - database assertions will fail"
    echo "Install it with: pip install psycopg2-binary"
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Run the test
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Running E2E Trigger Flow Test${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo ""
echo "This test will:"
echo "  1. Call the external trigger endpoint"
echo "  2. Trigger sends lead to your ngrok URL"
echo "  3. Django receives and queues the lead"
echo "  4. Celery processes the lead asynchronously"
echo "  5. Test verifies lead in database"
echo ""
echo "This may take 10-15 seconds..."
echo ""

export NGROK_URL=$NGROK_URL

# Run the test with verbose output
pytest leads/tests/test_e2e_trigger_flow.py -v -s

TEST_EXIT_CODE=$?

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✅ Test Passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "  • Check ngrok dashboard: http://localhost:4040"
    echo "  • View Django logs: docker-compose logs web"
    echo "  • View Celery logs: docker-compose logs celery"
else
    echo -e "${RED}❌ Test Failed${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "  • Verify ngrok is running and URL is correct"
    echo "  • Check Django logs: docker-compose logs web"
    echo "  • Verify database is accessible"
    echo "  • Check ngrok dashboard: http://localhost:4040"
fi

echo -e "${CYAN}═══════════════════════════════════════════════════${NC}"
echo ""

exit $TEST_EXIT_CODE
