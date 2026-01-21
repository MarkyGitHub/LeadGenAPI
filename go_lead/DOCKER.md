# Docker Deployment Guide

This guide explains how to deploy the Go Lead Gateway Service using Docker and Docker Compose.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Services](#services)
- [Configuration](#configuration)
- [Health Checks](#health-checks)
- [Testing the Service](#testing-the-service)
- [Troubleshooting](#troubleshooting)
- [Production Considerations](#production-considerations)
- [Multi-Stage Build](#multi-stage-build)
- [Network Architecture](#network-architecture)
- [Volumes and Backups](#volumes-and-backups)

## Prerequisites

- Docker 20.10 or later
- Docker Compose 2.0 or later
- Basic understanding of Docker and containerization

## Quick Start

1. **Navigate to the project directory:**

   ```bash
   cd go_lead
   ```

2. **Build and start all services:**

   ```bash
   docker-compose up --build
   ```

   This will start:
   - PostgreSQL database on port 5433
   - Redis queue on port 6380
   - API server on port 8080
   - Background worker

3. **Verify services are running:**

   ```bash
   docker-compose ps
   ```

   All services should show status "Up" with healthy health checks.

4. **View logs:**

   ```bash
   # All services
   docker-compose logs -f

   # Specific service
   docker-compose logs -f api
   docker-compose logs -f worker
   ```

5. **Test the API:**

   ```bash
   # Health check
   curl http://localhost:8080/health

   # Send test webhook
   curl -X POST http://localhost:8080/webhooks/leads \
     -H "Content-Type: application/json" \
     -d '{
       "email": "test@example.com",
       "phone": "+49123456789",
       "zipcode": "66123",
       "house": {"is_owner": true}
     }'

   # Check statistics
   curl http://localhost:8080/stats/leads/counts
   ```

6. **Stop services:**

   ```bash
   # Stop and remove containers
   docker-compose down

   # Stop and remove containers + volumes (deletes data)
   docker-compose down -v
   ```

## Common Commands

```bash
# Start services in detached mode
docker-compose up -d

# Start services with rebuild
docker-compose up --build

# Stop services
docker-compose down

# View logs
docker-compose logs -f [service_name]

# Restart a service
docker-compose restart api

# Execute command in container
docker-compose exec api sh
docker-compose exec postgres psql -U postgres -d lead_gateway

# View container status
docker-compose ps

# View resource usage
docker stats
```

## Services

The docker-compose configuration includes the following services:

### PostgreSQL Database

- **Container Name:** `go_lead_postgres`
- **Image:** `postgres:14-alpine`
- **Port Mapping:** 5433 (host) → 5432 (container)
- **Environment:**
  - `POSTGRES_USER=postgres`
  - `POSTGRES_PASSWORD=postgres`
  - `POSTGRES_DB=lead_gateway`
- **Health Check:** Checks PostgreSQL readiness every 5 seconds
- **Volume:** `postgres_data` for data persistence
- **Purpose:** Stores leads, delivery attempts, and audit trail

### Redis Queue

- **Container Name:** `go_lead_redis`
- **Image:** `redis:7-alpine`
- **Port Mapping:** 6380 (host) → 6379 (container)
- **Health Check:** Pings Redis every 5 seconds
- **Purpose:** Job queue for asynchronous lead processing

### API Server

- **Container Name:** `go_lead_api`
- **Build:** Multi-stage Dockerfile (API target)
- **Port Mapping:** 8080 (host) → 8080 (container)
- **Health Check:** HTTP GET to `/health` endpoint every 30 seconds
- **Depends On:** PostgreSQL (healthy), Redis (healthy)
- **Purpose:** Receives webhook requests, stores leads, enqueues jobs
- **Endpoints:**
  - `POST /webhooks/leads` - Receive lead webhooks
  - `GET /health` - Health check endpoint
  - `GET /stats/leads/counts` - Lead statistics by status
  - `GET /stats/leads/recent` - Recent leads
  - `GET /stats/leads/{id}/history` - Lead history with delivery attempts

### Background Worker

- **Container Name:** `go_lead_worker`
- **Build:** Multi-stage Dockerfile (Worker target)
- **No Port Mapping:** Internal service only
- **Depends On:** PostgreSQL (healthy), Redis (healthy)
- **Purpose:** Processes leads asynchronously
  - Validates leads against business rules
  - Transforms leads to customer format
  - Delivers leads to Customer API with retry logic
- **Concurrency:** Configurable via `WORKER_CONCURRENCY` (default: 5)

## Configuration

Environment variables can be customized in the `docker-compose.yml` file or by creating a `.env` file in the `go_lead` directory.

### Environment Variables Reference

#### Database Configuration

```bash
DB_HOST=postgres               # Database hostname (use service name in Docker)
DB_PORT=5432                   # Database port (internal container port)
DB_USER=postgres               # Database user
DB_PASSWORD=postgres           # Database password
DB_NAME=lead_gateway           # Database name
DB_SSLMODE=disable             # SSL mode (disable, require, verify-full)
```

**Note:** When running in Docker, use the service name (`postgres`) as the hostname. The internal port is always 5432, even though it's mapped to 5433 on the host.

#### API Configuration

```bash
API_PORT=8080                  # API server port
API_HOST=0.0.0.0               # API server host (0.0.0.0 for all interfaces)
```

#### Worker Configuration

```bash
WORKER_POLL_INTERVAL=5s        # Job polling interval (e.g., 1s, 5s, 10s)
WORKER_CONCURRENCY=5           # Number of concurrent workers (1-20 recommended)
```

**Tuning Guidelines:**

- Lower `WORKER_POLL_INTERVAL` for faster processing (higher CPU usage)
- Higher `WORKER_CONCURRENCY` for parallel processing (more database connections)
- Monitor queue depth and adjust accordingly

#### Queue Configuration

```bash
QUEUE_TYPE=redis               # Queue type (redis or database)
REDIS_URL=redis://redis:6379/0 # Redis connection URL (use service name in Docker)
```

#### Customer API Configuration

```bash
CUSTOMER_API_URL=https://api.customer.example.com  # Customer API endpoint URL
CUSTOMER_API_TOKEN=your_bearer_token_here          # Bearer token for authentication
CUSTOMER_API_TIMEOUT=30s                           # Request timeout (e.g., 10s, 30s, 60s)
CUSTOMER_PRODUCT_NAME=solar_panel_installation     # Product name to send to Customer API
```

**Important:** Replace `CUSTOMER_API_TOKEN` with your actual token before deployment.

#### Retry Configuration

```bash
MAX_RETRY_ATTEMPTS=5           # Maximum delivery retry attempts (1-10 recommended)
RETRY_BACKOFF_BASE=30s         # Base delay for exponential backoff (e.g., 10s, 30s, 60s)
```

**Retry Schedule (with default settings):**

- Attempt 1: Immediate
- Attempt 2: 30s delay
- Attempt 3: 60s delay (2 × base)
- Attempt 4: 120s delay (4 × base)
- Attempt 5: 240s delay (8 × base)
- After 5 attempts: Mark as PERMANENTLY_FAILED

#### Authentication (Optional)

```bash
ENABLE_AUTH=false              # Enable shared secret authentication (true/false)
SHARED_SECRET=your_shared_secret_here  # Shared secret for webhook authentication
```

When `ENABLE_AUTH=true`, webhook requests must include:

```
X-Shared-Secret: your_shared_secret_here
```

#### Logging

```bash
LOG_LEVEL=info                 # Logging level (debug, info, warn, error)
LOG_FORMAT=json                # Log format (json or text)
```

**Log Levels:**

- `debug`: Verbose logging for development
- `info`: Standard operational logging (recommended for production)
- `warn`: Warning messages only
- `error`: Error messages only

#### Attribute Mapping Configuration

```bash
ATTRIBUTE_MAPPING_FILE=./config/customer_attribute_mapping.json
```

This file defines validation rules for lead attributes. See `config/customer_attribute_mapping.json` for the schema.

### Customizing Configuration

#### Option 1: Edit docker-compose.yml

Edit the `environment` section for each service in `docker-compose.yml`:

```yaml
services:
  api:
    environment:
      - LOG_LEVEL=debug
      - WORKER_CONCURRENCY=10
```

#### Option 2: Create .env file

Create a `.env` file in the `go_lead` directory:

```bash
# .env
LOG_LEVEL=debug
WORKER_CONCURRENCY=10
CUSTOMER_API_TOKEN=your_actual_token
```

Then reference it in `docker-compose.yml`:

```yaml
services:
  api:
    env_file:
      - .env
```

#### Option 3: Environment Variables

Set environment variables before running docker-compose:

```bash
export LOG_LEVEL=debug
export WORKER_CONCURRENCY=10
docker-compose up
```

## Health Checks

All services include health checks to ensure they're running correctly.

### API Server Health Check

**Check endpoint:**

```bash
curl http://localhost:8080/health
```

**Expected response:**

```
OK
```

**Health check configuration:**

- Interval: 30 seconds
- Timeout: 5 seconds
- Retries: 3
- Start period: 10 seconds

### Database Health Check

**Check from host:**

```bash
docker-compose exec postgres pg_isready -U postgres
```

**Expected response:**

```
/var/run/postgresql:5432 - accepting connections
```

**Check from container:**

```bash
docker-compose exec api sh -c 'pg_isready -h postgres -U postgres'
```

**Health check configuration:**

- Interval: 5 seconds
- Timeout: 3 seconds
- Retries: 5

### Redis Health Check

**Check from host:**

```bash
docker-compose exec redis redis-cli ping
```

**Expected response:**

```
PONG
```

**Check from container:**

```bash
docker-compose exec api sh -c 'redis-cli -h redis ping'
```

**Health check configuration:**

- Interval: 5 seconds
- Timeout: 3 seconds
- Retries: 5

### View Health Status

```bash
# View all container health status
docker-compose ps

# View detailed health check logs
docker inspect go_lead_api | grep -A 10 Health

# View health check history
docker inspect go_lead_api --format='{{json .State.Health}}' | jq
```

## Testing the Service

### 1. Send Test Webhooks

#### Valid Lead (Should be accepted and delivered)

```bash
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {
      "is_owner": true
    },
    "solar_energy_consumption": "5000",
    "solar_offer_type": "Kaufen"
  }'
```

**Expected response:**

```json
{
  "lead_id": 1,
  "status": "RECEIVED",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

#### Invalid Zipcode (Should be rejected)

```bash
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "phone": "+49123456789",
    "zipcode": "12345",
    "house": {
      "is_owner": true
    }
  }'
```

**Expected:** Lead accepted (200 OK), then rejected during processing with reason `ZIP_NOT_66XXX`

#### Not Homeowner (Should be rejected)

```bash
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {
      "is_owner": false
    }
  }'
```

**Expected:** Lead accepted (200 OK), then rejected during processing with reason `NOT_HOMEOWNER`

#### Malformed JSON (Should be rejected immediately)

```bash
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{invalid json}'
```

**Expected response (400 Bad Request):**

```json
{
  "error": "malformed JSON payload",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### 2. Check Lead Statistics

```bash
# Get lead counts by status
curl http://localhost:8080/stats/leads/counts
```

**Expected response:**

```json
{
  "received": 0,
  "rejected": 2,
  "ready": 0,
  "delivered": 1,
  "failed": 0,
  "permanently_failed": 0,
  "total": 3
}
```

### 3. View Recent Leads

```bash
# Get 50 most recent leads
curl http://localhost:8080/stats/leads/recent
```

**Expected response:**

```json
[
  {
    "id": 3,
    "received_at": "2026-01-21T10:35:00Z",
    "status": "REJECTED",
    "rejection_reason": "NOT_HOMEOWNER"
  },
  {
    "id": 2,
    "received_at": "2026-01-21T10:34:00Z",
    "status": "REJECTED",
    "rejection_reason": "ZIP_NOT_66XXX"
  },
  {
    "id": 1,
    "received_at": "2026-01-21T10:33:00Z",
    "status": "DELIVERED"
  }
]
```

### 4. View Lead History

```bash
# Get full history for lead ID 1
curl http://localhost:8080/stats/leads/1/history
```

**Expected response:**

```json
{
  "id": 1,
  "received_at": "2026-01-21T10:33:00Z",
  "status": "DELIVERED",
  "rejection_reason": null,
  "raw_payload": {
    "email": "customer@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {
      "is_owner": true
    }
  },
  "normalized_payload": {
    "email": "customer@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {
      "is_owner": true
    }
  },
  "customer_payload": {
    "phone": "+49123456789",
    "product": {
      "name": "solar_panel_installation"
    },
    "attributes": []
  },
  "delivery_attempts": [
    {
      "attempt_no": 1,
      "attempted_at": "2026-01-21T10:33:05Z",
      "success": true,
      "status_code": 200,
      "error_message": null
    }
  ]
}
```

### 5. Monitor Logs

```bash
# Watch all logs
docker-compose logs -f

# Watch API logs only
docker-compose logs -f api

# Watch worker logs only
docker-compose logs -f worker

# Filter logs by lead ID
docker-compose logs -f | grep "lead_id=1"

# Filter error logs
docker-compose logs -f | grep ERROR
```

### 6. Query Database Directly

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U postgres -d lead_gateway

# Query leads
SELECT id, status, received_at, rejection_reason
FROM inbound_lead
ORDER BY received_at DESC
LIMIT 10;

# Query delivery attempts
SELECT lead_id, attempt_no, success, response_status, requested_at
FROM delivery_attempt
ORDER BY requested_at DESC
LIMIT 10;

# Count leads by status
SELECT status, COUNT(*)
FROM inbound_lead
GROUP BY status;

# Exit psql
\q
```

### 7. Test with Authentication

If authentication is enabled (`ENABLE_AUTH=true`):

```bash
# Valid request with shared secret
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -H "X-Shared-Secret: your_shared_secret_here" \
  -d '{
    "email": "test@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {"is_owner": true}
  }'

# Invalid request without shared secret
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {"is_owner": true}
  }'
```

**Expected response (401 Unauthorized):**

```json
{
  "error": "unauthorized",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Containers Won't Start

**Symptom:** `docker-compose up` fails or containers exit immediately

**Solutions:**

1. **Check Docker is running:**

   ```bash
   docker ps
   ```

2. **Check for port conflicts:**

   ```bash
   # Check if ports are already in use
   netstat -an | grep 8080   # API port
   netstat -an | grep 5433   # PostgreSQL port
   netstat -an | grep 6380   # Redis port
   ```

3. **View container logs:**

   ```bash
   docker-compose logs api
   docker-compose logs postgres
   docker-compose logs redis
   ```

4. **Remove old containers and volumes:**
   ```bash
   docker-compose down -v
   docker-compose up --build
   ```

#### 2. Database Connection Errors

**Symptom:** API or worker logs show database connection errors

**Solutions:**

1. **Check PostgreSQL is healthy:**

   ```bash
   docker-compose ps postgres
   docker-compose exec postgres pg_isready -U postgres
   ```

2. **Check database credentials:**
   - Verify `DB_USER`, `DB_PASSWORD`, `DB_NAME` in docker-compose.yml
   - Ensure they match between services

3. **Check network connectivity:**

   ```bash
   docker-compose exec api ping postgres
   ```

4. **View PostgreSQL logs:**

   ```bash
   docker-compose logs postgres
   ```

5. **Restart PostgreSQL:**
   ```bash
   docker-compose restart postgres
   ```

#### 3. Worker Not Processing Leads

**Symptom:** Leads stuck in RECEIVED status

**Solutions:**

1. **Check worker is running:**

   ```bash
   docker-compose ps worker
   ```

2. **Check worker logs:**

   ```bash
   docker-compose logs -f worker
   ```

3. **Check Redis connectivity:**

   ```bash
   docker-compose exec worker sh -c 'redis-cli -h redis ping'
   ```

4. **Check queue depth:**

   ```bash
   docker-compose exec redis redis-cli LLEN queue:process_lead
   ```

5. **Restart worker:**
   ```bash
   docker-compose restart worker
   ```

#### 4. Leads Stuck in FAILED Status

**Symptom:** Leads not progressing to DELIVERED

**Solutions:**

1. **Check Customer API configuration:**
   - Verify `CUSTOMER_API_URL` is correct
   - Verify `CUSTOMER_API_TOKEN` is valid
   - Test Customer API manually:
     ```bash
     curl -H "Authorization: Bearer $CUSTOMER_API_TOKEN" $CUSTOMER_API_URL
     ```

2. **Check delivery attempts:**

   ```bash
   curl http://localhost:8080/stats/leads/{id}/history
   ```

3. **Check worker logs for errors:**

   ```bash
   docker-compose logs worker | grep ERROR
   ```

4. **Verify retry configuration:**
   - Check `MAX_RETRY_ATTEMPTS` and `RETRY_BACKOFF_BASE`

#### 5. Slow Performance

**Symptom:** Webhook responses are slow or leads take long to process

**Solutions:**

1. **Check resource usage:**

   ```bash
   docker stats
   ```

2. **Increase worker concurrency:**

   ```yaml
   # In docker-compose.yml
   environment:
     - WORKER_CONCURRENCY=10
   ```

3. **Decrease poll interval:**

   ```yaml
   environment:
     - WORKER_POLL_INTERVAL=1s
   ```

4. **Check database performance:**

   ```bash
   docker-compose exec postgres psql -U postgres -d lead_gateway
   # Run EXPLAIN ANALYZE on slow queries
   ```

5. **Check queue depth:**
   ```bash
   docker-compose exec redis redis-cli LLEN queue:process_lead
   ```

#### 6. Out of Memory Errors

**Symptom:** Containers crash with OOM errors

**Solutions:**

1. **Increase Docker memory limit:**
   - Docker Desktop: Settings → Resources → Memory

2. **Add memory limits to docker-compose.yml:**

   ```yaml
   services:
     api:
       deploy:
         resources:
           limits:
             memory: 512M
   ```

3. **Reduce worker concurrency:**
   ```yaml
   environment:
     - WORKER_CONCURRENCY=3
   ```

#### 7. Permission Denied Errors

**Symptom:** Permission errors when accessing volumes

**Solutions:**

1. **Check volume permissions:**

   ```bash
   docker-compose exec postgres ls -la /var/lib/postgresql/data
   ```

2. **Remove and recreate volumes:**

   ```bash
   docker-compose down -v
   docker-compose up
   ```

3. **Run with correct user (if needed):**
   ```yaml
   services:
     postgres:
       user: "1000:1000"
   ```

### Debugging Commands

```bash
# View all container logs
docker-compose logs -f

# View specific service logs
docker-compose logs -f api
docker-compose logs -f worker

# Follow logs with grep
docker-compose logs -f | grep ERROR
docker-compose logs -f | grep "lead_id=123"

# Execute shell in container
docker-compose exec api sh
docker-compose exec worker sh

# View container processes
docker-compose top

# View resource usage
docker stats

# Inspect container
docker inspect go_lead_api

# View network
docker network ls
docker network inspect go_lead_go_lead_network

# View volumes
docker volume ls
docker volume inspect go_lead_postgres_data
```

### Reset Everything

If all else fails, completely reset the environment:

```bash
# Stop and remove everything
docker-compose down -v

# Remove images
docker-compose down --rmi all

# Rebuild from scratch
docker-compose build --no-cache
docker-compose up
```

## Production Considerations

For production deployments, consider the following best practices:

### 1. Security

**Secrets Management:**

- Use Docker secrets or external secrets management (AWS Secrets Manager, HashiCorp Vault)
- Never commit `.env` files or secrets to version control
- Rotate Customer API tokens regularly

```yaml
# Example using Docker secrets
secrets:
  db_password:
    external: true
  customer_api_token:
    external: true

services:
  api:
    secrets:
      - db_password
      - customer_api_token
```

**Authentication:**

- Enable webhook authentication: `ENABLE_AUTH=true`
- Use strong, randomly generated shared secrets
- Consider implementing API key rotation

**Network Security:**

- Use SSL/TLS for database connections: `DB_SSLMODE=require`
- Use HTTPS for all external communication
- Set up firewall rules to restrict access
- Use private networks for internal communication

**Container Security:**

- Run containers as non-root user
- Use minimal base images (alpine)
- Scan images for vulnerabilities
- Keep base images updated

### 2. Database

**Connection Pooling:**

```yaml
environment:
  - DB_MAX_OPEN_CONNS=50
  - DB_MAX_IDLE_CONNS=10
  - DB_CONN_MAX_LIFETIME=1h
```

**Backups:**

```bash
# Automated backup script
docker-compose exec postgres pg_dump -U postgres lead_gateway > backup_$(date +%Y%m%d_%H%M%S).sql

# Restore from backup
docker-compose exec -T postgres psql -U postgres lead_gateway < backup.sql
```

**Replication:**

- Set up PostgreSQL streaming replication for high availability
- Use read replicas for statistics queries
- Configure automatic failover

**Monitoring:**

- Monitor connection pool usage
- Track slow queries
- Set up alerts for connection errors
- Monitor disk space usage

### 3. Monitoring and Observability

**Log Aggregation:**

- Use centralized logging (ELK Stack, Splunk, CloudWatch Logs)
- Configure log shipping from containers
- Set up log retention policies

```yaml
# Example with Fluentd
services:
  api:
    logging:
      driver: fluentd
      options:
        fluentd-address: localhost:24224
        tag: api
```

**Metrics:**

- Expose Prometheus metrics endpoint
- Track key metrics:
  - Webhook response time
  - Lead processing time
  - Delivery success rate
  - Queue depth
  - Database connection pool usage
  - Error rates

**Alerting:**

- Set up alerts for:
  - High error rates
  - Slow operations (>1s)
  - Queue backlog
  - Database connection failures
  - Customer API delivery failures
  - Container health check failures

**Distributed Tracing:**

- Implement distributed tracing (Jaeger, Zipkin)
- Use correlation IDs for request tracing
- Track request flow across services

### 4. Scaling

**Horizontal Scaling:**

```yaml
# Scale API servers
docker-compose up --scale api=3

# Scale workers
docker-compose up --scale worker=5
```

**Load Balancing:**

- Use nginx or Traefik as reverse proxy
- Distribute webhook requests across API instances
- Implement health check-based routing

```yaml
# Example nginx configuration
upstream api_backend {
server api1:8080;
server api2:8080;
server api3:8080;
}
```

**Queue Scaling:**

- Use Redis Cluster for high-throughput queues
- Monitor queue depth and scale workers accordingly
- Consider using dedicated queue services (RabbitMQ, AWS SQS)

**Database Scaling:**

- Use connection pooling
- Implement read replicas for queries
- Consider database sharding for very high volumes
- Use caching (Redis) for frequently accessed data

### 5. High Availability

**Service Redundancy:**

- Run multiple instances of each service
- Use container orchestration (Kubernetes, Docker Swarm)
- Implement automatic restart on failure

**Database HA:**

- Set up PostgreSQL streaming replication
- Configure automatic failover (Patroni, Stolon)
- Use managed database services (AWS RDS, Google Cloud SQL)

**Queue HA:**

- Use Redis Sentinel for automatic failover
- Use Redis Cluster for distributed queues
- Consider managed queue services

**Health Checks:**

- Configure proper health check intervals
- Implement graceful shutdown
- Use readiness and liveness probes

### 6. Performance Optimization

**Database:**

- Add indexes for frequently queried columns
- Optimize slow queries
- Use connection pooling
- Consider partitioning for large tables

**Worker:**

- Tune `WORKER_CONCURRENCY` based on load
- Adjust `WORKER_POLL_INTERVAL` for responsiveness
- Monitor and optimize processing time

**API:**

- Implement request rate limiting
- Use caching for statistics endpoints
- Optimize JSON serialization
- Consider using CDN for static content

**Network:**

- Use HTTP/2 for Customer API requests
- Implement connection keep-alive
- Optimize payload sizes

### 7. Resource Limits

Set resource limits to prevent resource exhaustion:

```yaml
services:
  api:
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: 512M
        reservations:
          cpus: "0.5"
          memory: 256M

  worker:
    deploy:
      resources:
        limits:
          cpus: "2.0"
          memory: 1G
        reservations:
          cpus: "1.0"
          memory: 512M
```

### 8. Disaster Recovery

**Backup Strategy:**

- Automated daily database backups
- Store backups in multiple locations
- Test restore procedures regularly
- Document recovery procedures

**Data Retention:**

- Define retention policies for leads and logs
- Implement automated cleanup
- Archive old data to cold storage

**Incident Response:**

- Document incident response procedures
- Set up on-call rotation
- Create runbooks for common issues
- Conduct post-incident reviews

### 9. Compliance and Audit

**Data Privacy:**

- Implement data encryption at rest and in transit
- Define data retention and deletion policies
- Ensure GDPR/CCPA compliance if applicable
- Implement audit logging

**Audit Trail:**

- Log all lead processing activities
- Track all status transitions
- Store all delivery attempts
- Implement tamper-proof logging

### 10. Deployment Strategy

**Blue-Green Deployment:**

- Maintain two identical environments
- Switch traffic between environments
- Enable quick rollback

**Rolling Updates:**

- Update services one at a time
- Monitor health during updates
- Implement automatic rollback on failure

**Canary Deployment:**

- Route small percentage of traffic to new version
- Monitor metrics and errors
- Gradually increase traffic if successful

### Production Checklist

- [ ] Secrets stored in secrets management system
- [ ] Authentication enabled (`ENABLE_AUTH=true`)
- [ ] SSL/TLS enabled for database (`DB_SSLMODE=require`)
- [ ] HTTPS configured for API endpoint
- [ ] Database backups automated and tested
- [ ] Log aggregation configured
- [ ] Monitoring and alerting set up
- [ ] Resource limits configured
- [ ] Health checks configured
- [ ] Multiple instances of each service running
- [ ] Load balancer configured
- [ ] Disaster recovery plan documented
- [ ] Incident response procedures documented
- [ ] Security scanning automated
- [ ] Performance testing completed
- [ ] Compliance requirements met

## Multi-Stage Build

The Dockerfile uses a multi-stage build approach for optimal image size and security:

### Build Stages

**Stage 1: Builder**

```dockerfile
FROM golang:1.21-alpine AS builder
# Compiles Go binaries with optimizations
# Includes build dependencies
# Results in ~500MB image (not used in final images)
```

**Stage 2: API Runtime**

```dockerfile
FROM alpine:3.18 AS api
# Minimal runtime image (~20MB)
# Copies only API binary from builder
# Includes CA certificates for HTTPS
# Runs as non-root user
```

**Stage 3: Worker Runtime**

```dockerfile
FROM alpine:3.18 AS worker
# Minimal runtime image (~20MB)
# Copies only worker binary from builder
# Includes CA certificates for HTTPS
# Runs as non-root user
```

### Benefits

1. **Small Image Size:** Final images are ~20MB (vs ~500MB with full Go toolchain)
2. **Security:** No build tools or source code in runtime images
3. **Fast Deployment:** Smaller images deploy faster
4. **Separation:** API and worker have separate optimized images

### Building Specific Targets

```bash
# Build API image only
docker build --target api -t go_lead_api .

# Build worker image only
docker build --target worker -t go_lead_worker .

# Build both (default)
docker-compose build
```

### Dockerfile Structure

```dockerfile
# Stage 1: Build
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api cmd/api/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker cmd/worker/main.go

# Stage 2: API Runtime
FROM alpine:3.18 AS api
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/api .
COPY --from=builder /app/config ./config
COPY --from=builder /app/migrations ./migrations
EXPOSE 8080
CMD ["./api"]

# Stage 3: Worker Runtime
FROM alpine:3.18 AS worker
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/worker .
COPY --from=builder /app/config ./config
CMD ["./worker"]
```

## Network Architecture

All services communicate through a dedicated Docker network (`go_lead_network`):

```
┌─────────────────────────────────────────────────────────┐
│                    External Clients                      │
│                  (Webhook Senders)                       │
└────────────────────┬────────────────────────────────────┘
                     │ HTTP
                     │ Port 8080
                     ▼
┌─────────────────────────────────────────────────────────┐
│                   API Server (api)                       │
│  - Receives webhooks                                     │
│  - Stores leads to database                              │
│  - Enqueues jobs to Redis                                │
│  - Serves statistics endpoints                           │
└────────┬──────────────────────────┬─────────────────────┘
         │                          │
         │ PostgreSQL               │ Redis
         │ Port 5432                │ Port 6379
         ▼                          ▼
┌─────────────────┐        ┌─────────────────┐
│   PostgreSQL    │        │      Redis      │
│   (postgres)    │        │     (redis)     │
│                 │        │                 │
│  - Stores leads │        │  - Job queue    │
│  - Audit trail  │        │  - Async jobs   │
└────────┬────────┘        └────────┬────────┘
         │                          │
         │ PostgreSQL               │ Redis
         │ Port 5432                │ Port 6379
         ▼                          ▼
┌─────────────────────────────────────────────────────────┐
│              Background Worker (worker)                  │
│  - Dequeues jobs from Redis                              │
│  - Validates leads                                       │
│  - Transforms leads                                      │
│  - Delivers to Customer API                              │
│  - Updates lead status in database                       │
└────────────────────┬────────────────────────────────────┘
                     │ HTTPS
                     │ Bearer Token Auth
                     ▼
┌─────────────────────────────────────────────────────────┐
│                    Customer API                          │
│                  (External Service)                      │
└─────────────────────────────────────────────────────────┘
```

### Network Configuration

**Network Name:** `go_lead_network`
**Driver:** bridge
**Subnet:** Auto-assigned by Docker

### Service Communication

**Internal DNS:**

- Services communicate using service names as hostnames
- `postgres` resolves to PostgreSQL container IP
- `redis` resolves to Redis container IP
- `api` resolves to API server container IP

**Port Mapping:**

- **API Server:** 8080 (host) → 8080 (container)
- **PostgreSQL:** 5433 (host) → 5432 (container)
- **Redis:** 6380 (host) → 6379 (container)
- **Worker:** No external ports (internal only)

**Connection Strings:**

```bash
# From host machine
DATABASE_URL=postgresql://postgres:postgres@localhost:5433/lead_gateway
REDIS_URL=redis://localhost:6380/0

# From within Docker network
DATABASE_URL=postgresql://postgres:postgres@postgres:5432/lead_gateway
REDIS_URL=redis://redis:6379/0
```

### Network Isolation

**Benefits:**

- Services are isolated from host network
- Internal communication doesn't expose ports to host
- Easy service discovery via DNS
- Network policies can be applied

**Security:**

- Only API server exposes port to host (8080)
- Database and Redis only accessible within Docker network
- Worker has no external ports (internal only)

### Inspecting the Network

```bash
# List networks
docker network ls

# Inspect network
docker network inspect go_lead_go_lead_network

# View connected containers
docker network inspect go_lead_go_lead_network --format='{{range .Containers}}{{.Name}} {{.IPv4Address}}{{"\n"}}{{end}}'

# Test connectivity between containers
docker-compose exec api ping postgres
docker-compose exec worker ping redis
```

## Volumes and Backups

### Volumes

**postgres_data Volume:**

- **Purpose:** Persists PostgreSQL database data
- **Type:** Named volume
- **Location:** Managed by Docker (typically `/var/lib/docker/volumes/`)
- **Persistence:** Data survives container restarts and recreations

```yaml
volumes:
  postgres_data:
    driver: local
```

### Volume Management

**List volumes:**

```bash
docker volume ls
```

**Inspect volume:**

```bash
docker volume inspect go_lead_postgres_data
```

**View volume location:**

```bash
docker volume inspect go_lead_postgres_data --format='{{.Mountpoint}}'
```

**Remove volume (WARNING: Deletes all data):**

```bash
docker-compose down -v
```

### Database Backups

#### Manual Backup

**Create backup:**

```bash
# Backup to file with timestamp
docker-compose exec postgres pg_dump -U postgres lead_gateway > backup_$(date +%Y%m%d_%H%M%S).sql

# Backup with compression
docker-compose exec postgres pg_dump -U postgres lead_gateway | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz

# Backup specific tables
docker-compose exec postgres pg_dump -U postgres -t inbound_lead -t delivery_attempt lead_gateway > backup_tables.sql
```

**Restore from backup:**

```bash
# Restore from SQL file
docker-compose exec -T postgres psql -U postgres lead_gateway < backup.sql

# Restore from compressed file
gunzip -c backup.sql.gz | docker-compose exec -T postgres psql -U postgres lead_gateway

# Restore to new database
docker-compose exec postgres createdb -U postgres lead_gateway_restore
docker-compose exec -T postgres psql -U postgres lead_gateway_restore < backup.sql
```

#### Automated Backup Script

Create a backup script (`backup.sh`):

```bash
#!/bin/bash
# backup.sh - Automated database backup script

BACKUP_DIR="./backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/lead_gateway_$TIMESTAMP.sql.gz"
RETENTION_DAYS=30

# Create backup directory
mkdir -p $BACKUP_DIR

# Create backup
echo "Creating backup: $BACKUP_FILE"
docker-compose exec -T postgres pg_dump -U postgres lead_gateway | gzip > $BACKUP_FILE

# Check if backup was successful
if [ $? -eq 0 ]; then
    echo "Backup created successfully"

    # Remove old backups
    find $BACKUP_DIR -name "lead_gateway_*.sql.gz" -mtime +$RETENTION_DAYS -delete
    echo "Old backups removed (older than $RETENTION_DAYS days)"
else
    echo "Backup failed"
    exit 1
fi
```

**Make script executable:**

```bash
chmod +x backup.sh
```

**Run backup:**

```bash
./backup.sh
```

**Schedule with cron:**

```bash
# Edit crontab
crontab -e

# Add daily backup at 2 AM
0 2 * * * /path/to/go_lead/backup.sh >> /path/to/go_lead/backup.log 2>&1
```

#### Backup to Cloud Storage

**AWS S3:**

```bash
# Backup and upload to S3
docker-compose exec -T postgres pg_dump -U postgres lead_gateway | gzip | aws s3 cp - s3://my-bucket/backups/lead_gateway_$(date +%Y%m%d_%H%M%S).sql.gz
```

**Google Cloud Storage:**

```bash
# Backup and upload to GCS
docker-compose exec -T postgres pg_dump -U postgres lead_gateway | gzip | gsutil cp - gs://my-bucket/backups/lead_gateway_$(date +%Y%m%d_%H%M%S).sql.gz
```

### Data Migration

**Export data from old system:**

```bash
# Export from old database
pg_dump -h old-host -U postgres -d old_database > old_data.sql
```

**Import to Docker container:**

```bash
# Import to new database
docker-compose exec -T postgres psql -U postgres lead_gateway < old_data.sql
```

**Verify migration:**

```bash
# Check record counts
docker-compose exec postgres psql -U postgres -d lead_gateway -c "SELECT COUNT(*) FROM inbound_lead;"
docker-compose exec postgres psql -U postgres -d lead_gateway -c "SELECT COUNT(*) FROM delivery_attempt;"
```

### Disaster Recovery

**Complete system backup:**

```bash
# Backup database
docker-compose exec postgres pg_dump -U postgres lead_gateway | gzip > backup_db.sql.gz

# Backup configuration
tar -czf backup_config.tar.gz .env docker-compose.yml config/

# Backup volumes (if needed)
docker run --rm -v go_lead_postgres_data:/data -v $(pwd):/backup alpine tar -czf /backup/backup_volume.tar.gz /data
```

**Complete system restore:**

```bash
# Restore configuration
tar -xzf backup_config.tar.gz

# Start services
docker-compose up -d postgres

# Wait for PostgreSQL to be ready
sleep 10

# Restore database
gunzip -c backup_db.sql.gz | docker-compose exec -T postgres psql -U postgres lead_gateway

# Start remaining services
docker-compose up -d
```

### Volume Backup Best Practices

1. **Regular Backups:** Schedule automated backups daily or more frequently
2. **Multiple Locations:** Store backups in multiple locations (local + cloud)
3. **Test Restores:** Regularly test backup restoration procedures
4. **Retention Policy:** Define and implement backup retention policies
5. **Encryption:** Encrypt backups containing sensitive data
6. **Monitoring:** Monitor backup success/failure and set up alerts
7. **Documentation:** Document backup and restore procedures
8. **Access Control:** Restrict access to backup files

### Monitoring Volume Usage

```bash
# Check volume size
docker system df -v

# Check database size
docker-compose exec postgres psql -U postgres -d lead_gateway -c "SELECT pg_size_pretty(pg_database_size('lead_gateway'));"

# Check table sizes
docker-compose exec postgres psql -U postgres -d lead_gateway -c "
SELECT
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
"
```

---

## Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
- [PostgreSQL Docker Image](https://hub.docker.com/_/postgres)
- [Redis Docker Image](https://hub.docker.com/_/redis)
- [Go Docker Best Practices](https://docs.docker.com/language/golang/)

## Support

For issues or questions:

1. Check the [Troubleshooting](#troubleshooting) section
2. Review container logs: `docker-compose logs -f`
3. Check service health: `docker-compose ps`
4. Consult the main [README.md](README.md) for additional documentation
