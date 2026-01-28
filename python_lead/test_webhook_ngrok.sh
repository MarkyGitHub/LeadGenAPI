#!/bin/bash
# test_webhook_ngrok.sh - Test the webhook through ngrok
# 
# Usage: ./test_webhook_ngrok.sh [NGROK_URL]
# Example: ./test_webhook_ngrok.sh https://abcd1234.ngrok.io

set -e

# Color codes for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Get ngrok URL from argument or prompt
if [ -z "$1" ]; then
    echo -e "${YELLOW}Enter your ngrok URL (e.g., https://abcd1234.ngrok.io):${NC}"
    read -r NGROK_URL
else
    NGROK_URL=$1
fi

# Remove trailing slash if present
NGROK_URL=${NGROK_URL%/}

echo ""
echo "🚀 Testing Lead Gateway via ngrok"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Target: ${NGROK_URL}/webhooks/leads/"
echo ""

# Test 1: Valid lead
echo -e "${YELLOW}Test 1: Valid Lead (should succeed)${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${NGROK_URL}/webhooks/leads/" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "address": {
      "zip": "66123",
      "street": "Main Street 123"
    },
    "house": {
      "is_owner": true
    }
  }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" == "200" ]; then
    echo -e "${GREEN}✓ Test 1 PASSED${NC}"
    echo "Response: $BODY"
else
    echo -e "${RED}✗ Test 1 FAILED (HTTP $HTTP_CODE)${NC}"
    echo "Response: $BODY"
fi

echo ""
sleep 2

# Test 2: Invalid ZIP code
echo -e "${YELLOW}Test 2: Invalid ZIP (should be rejected)${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${NGROK_URL}/webhooks/leads/" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "invalid@example.com",
    "address": {
      "zip": "12345",
      "street": "Somewhere"
    },
    "house": {
      "is_owner": true
    }
  }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" == "200" ]; then
    echo -e "${GREEN}✓ Test 2 PASSED (accepted, will fail validation)${NC}"
    echo "Response: $BODY"
else
    echo -e "${YELLOW}⚠ Test 2 returned HTTP $HTTP_CODE${NC}"
    echo "Response: $BODY"
fi

echo ""
sleep 2

# Test 3: Not a homeowner
echo -e "${YELLOW}Test 3: Not a Homeowner (should be rejected)${NC}"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${NGROK_URL}/webhooks/leads/" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "renter@example.com",
    "address": {
      "zip": "66123",
      "street": "Rental Street 1"
    },
    "house": {
      "is_owner": false
    }
  }')

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" == "200" ]; then
    echo -e "${GREEN}✓ Test 3 PASSED (accepted, will fail validation)${NC}"
    echo "Response: $BODY"
else
    echo -e "${YELLOW}⚠ Test 3 returned HTTP $HTTP_CODE${NC}"
    echo "Response: $BODY"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}Testing Complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Check ngrok dashboard: http://localhost:4040"
echo "2. View Django logs: docker-compose logs web"
echo "3. View Celery logs: docker-compose logs celery"
echo ""
