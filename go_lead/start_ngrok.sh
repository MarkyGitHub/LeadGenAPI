#!/bin/bash
# start_ngrok.sh - Helper script to start ngrok tunnel for Go Lead Gateway
# 
# Usage: ./start_ngrok.sh

set -e

echo "🚀 Starting ngrok tunnel for Go Lead Gateway..."
echo ""

# Resolve local ngrok binary (preferred)
NGROK_CMD=""

if [ -f "./ngrok" ]; then
    NGROK_CMD="./ngrok"
elif [ -f "../ngrok" ]; then
    NGROK_CMD="../ngrok"
elif command -v ngrok &> /dev/null; then
    NGROK_CMD="ngrok"
else
    echo "❌ ngrok is not installed!"
    echo ""
    echo "Place the ngrok binary in this folder or in the repo root, or install ngrok:"
    echo "  • macOS:   brew install ngrok"
    echo "  • Linux:   snap install ngrok"
    echo "  • Windows: choco install ngrok"
    echo "  • Or download from: https://ngrok.com/download"
    echo ""
    exit 1
fi

# Check if Go API is running on port 8000
if ! nc -z localhost 8000 2>/dev/null && ! timeout 1 bash -c 'cat < /dev/null > /dev/tcp/localhost/8000' 2>/dev/null; then
    echo "⚠️  Warning: Go API doesn't appear to be running on port 8000"
    echo ""
    echo "Make sure your Go API is running first:"
    echo "  make run"
    echo "  OR"
    echo "  docker-compose up -d"
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo "Starting ngrok tunnel..."
echo "Port: 8000"
echo "Webhook endpoint: /webhooks/leads/"
echo ""
echo "📊 Inspect requests at: http://localhost:4041"
echo ""
echo "When ngrok starts, copy the HTTPS URL and use it as your webhook endpoint:"
echo "  Example: https://abcd1234.ngrok.io/webhooks/leads/"
echo ""
echo "Press Ctrl+C to stop the tunnel"
echo "─────────────────────────────────────────────────────"
echo ""

# Start ngrok with config file if it exists, otherwise use simple command
if [ -f "ngrok.yml" ]; then
    "$NGROK_CMD" start --all --config=ngrok.yml
else
    "$NGROK_CMD" http 8000
fi
