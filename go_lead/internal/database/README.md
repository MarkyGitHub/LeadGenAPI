# Database Package

This package provides database connection management and migration functionality for the Lead Gateway Service.

## Features

- **Connection Pooling**: Configurable connection pool with health checks
- **Migration Runner**: Automatic database schema migration from SQL files
- **Transaction Support**: Built-in transaction management for atomic operations

## Usage

### Initialize Database Connection

```go
import "github.com/checkfox/go_lead/internal/database"

// From application config
db, err := database.InitFromConfig(cfg)
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

### Run Migrations

```go
// Run all pending migrations
err := database.RunMigrations(db, "./migrations")
if err != nil {
    log.Fatal(err)
}
```

### Check Migration Status

```go
// Print migration status
err := database.MigrationStatus(db, "./migrations")
if err != nil {
    log.Fatal(err)
}
```

### Health Check

```go
// Verify database connection is healthy
if err := db.HealthCheck(); err != nil {
    log.Printf("Database unhealthy: %v", err)
}
```

## Migration Files

Migration files should be placed in the `migrations/` directory with the following naming convention:

```
001_create_inbound_lead.sql
002_create_delivery_attempt.sql
003_add_index.sql
```

The numeric prefix determines the execution order. Each migration runs in a transaction and is recorded in the `schema_migrations` table.

## Configuration

Database connection can be configured via environment variables:

- `DB_HOST`: Database host (default: localhost)
- `DB_PORT`: Database port (default: 5432)
- `DB_USER`: Database user (default: postgres)
- `DB_PASSWORD`: Database password (default: postgres)
- `DB_NAME`: Database name (default: lead_gateway)
- `DB_SSLMODE`: SSL mode (default: disable)

## Connection Pool Settings

The connection pool is configured with sensible defaults:

- Max Open Connections: 25
- Max Idle Connections: 5
- Connection Max Lifetime: 5 minutes
- Connection Max Idle Time: 5 minutes

These can be customized by modifying the `Config` struct when creating a new database connection.
