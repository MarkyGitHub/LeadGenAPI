# ngrok Setup Guide

Quick reference for using ngrok with the Lead Gateway Service.

## Quick Start

1. **Start your Django app:**
   ```bash
   docker-compose up -d
   ```

2. **Start ngrok:**
   ```bash
   # Windows
   start_ngrok.bat
   
   # Linux/macOS
   ./start_ngrok.sh
   ```

3. **Get your public URL:**
   Look for the line:
   ```
   Forwarding https://abcd1234.ngrok.io -> http://localhost:8004
   ```

4. **Use it as your webhook:**
   ```
   https://abcd1234.ngrok.io/webhooks/leads/
   ```

## Installation

### Windows
```powershell
# Using Chocolatey
choco install ngrok

# Or download from https://ngrok.com/download
```

### macOS
```bash
brew install ngrok
```

### Linux
```bash
snap install ngrok
```

## Authentication (First Time Only)

1. Sign up at https://dashboard.ngrok.com/signup
2. Get your authtoken from https://dashboard.ngrok.com/get-started/your-authtoken
3. Configure:
   ```bash
   ngrok config add-authtoken YOUR_TOKEN_HERE
   ```

## Testing Your Webhook

### Send a test lead:
```bash
curl -X POST https://YOUR_NGROK_URL.ngrok.io/webhooks/leads/ \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "address": {
      "zip": "66123",
      "street": "Main Street 1"
    },
    "house": {
      "is_owner": true
    }
  }'
```

### View requests:
Open http://localhost:4040 in your browser to see:
- All incoming requests
- Request/response details
- Replay functionality

## End-to-End Testing with Trigger Flow

Test the complete external flow: `Trigger → Lead Generator → ngrok → Django → Celery`

### Automated E2E Test

Run the complete trigger flow test:

```bash
# Windows
test_trigger_flow.bat https://abcd1234.ngrok.io

# Linux/macOS
./test_trigger_flow.sh https://abcd1234.ngrok.io
```

This test:
- ✅ Calls external trigger endpoint with your ngrok URL
- ✅ Verifies lead is received by Django
- ✅ Checks lead is processed by Celery
- ✅ Validates delivery attempts are created
- ✅ Asserts lead status in database

### Django ALLOWED_HOSTS for ngrok

Make sure Django allows ngrok hostnames:

```
ALLOWED_HOSTS=localhost,127.0.0.1,.ngrok-free.app,.ngrok.io
```

### Manual Trigger Test

Test the trigger endpoint manually:

```bash
# Set your ngrok URL
export NGROK_URL=https://abcd1234.ngrok.io

# Call trigger endpoint
curl --location 'https://contactapi.static.fyi/lead/trigger/fake/USER_ID/' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer FakeCustomerToken' \
  --data "{
    \"url\": \"$NGROK_URL/webhooks/leads/\",
    \"headers\": {
      \"Content-Type\": \"application/json\"
    }
  }"
```

Then monitor:
1. **ngrok dashboard** (http://localhost:4040) - See incoming webhook call
2. **Django logs** - `docker-compose logs -f web`
3. **Celery logs** - `docker-compose logs -f celery`
4. **Database** - Check lead status: `docker-compose exec db psql -U postgres -d lead_gateway -c "SELECT id, status FROM leads_inboundlead ORDER BY created_at DESC LIMIT 5;"`

## Configuration Options

Edit `ngrok.yml` to customize:

```yaml
tunnels:
  django-webhook:
    proto: http
    addr: 8004
    # Custom subdomain (paid plan required)
    subdomain: my-lead-gateway
    # Add authentication
    auth: "username:password"
```

## Running ngrok in Docker

To run ngrok as a Docker service:

1. Set your authtoken as environment variable:
   ```bash
   export NGROK_AUTHTOKEN=your_token_here
   ```

2. Uncomment the ngrok service in `docker-compose.yml`

3. Start all services:
   ```bash
   docker-compose up -d
   ```

4. Check ngrok logs:
   ```bash
   docker-compose logs ngrok
   ```

## Troubleshooting

### Port 8004 not accessible
- Check if Django is running: `docker-compose ps`
- Check port mapping: `docker-compose port web 8000`

### ngrok tunnel failed
- Verify your authtoken is configured
- Check if you have too many active tunnels (free plan limit: 1)
- Try restarting ngrok

### Can't access webhook
- Ensure you're using the HTTPS URL from ngrok
- Check the full path includes `/webhooks/leads/`
- Verify Django is accessible locally first

## Free vs Paid Plans

### Free Plan Includes:
- 1 online ngrok process
- 4 tunnels/ngrok process
- 40 connections/minute
- Random URLs (changes each restart)

### Paid Plans Add:
- Custom/reserved subdomains
- Reserved domains
- IP whitelisting
- More concurrent tunnels

## Security Notes

⚠️ **Important:**
- ngrok exposes your local server to the internet
- Use authentication for sensitive endpoints
- Don't use free ngrok URLs in production
- URLs change on restart (free plan)
- Monitor the web interface at http://localhost:4040

## Alternative Tools

- **localtunnel**: `npx localtunnel --port 8004`
- **Cloudflare Tunnel**: `cloudflared tunnel`
- **VS Code Port Forwarding**: Built-in for Codespaces
- **serveo.net**: SSH-based tunneling

## Support

- ngrok documentation: https://ngrok.com/docs
- ngrok dashboard: https://dashboard.ngrok.com
- Lead Gateway issues: See main README.md
