# Lead Gateway Service (Go)

Ein Go-basiertes Microservice-System zum Empfangen von Lead-Webhooks, Validieren, Transformieren und Zustellen an eine externe Customer API mit Retry-Logik und vollständigem Audit-Trail.

## Inhaltsverzeichnis

- [Überblick](#überblick)
- [Architektur](#architektur)
- [Projektstruktur](#projektstruktur)
- [Schnellstart](#schnellstart)
- [Konfiguration](#konfiguration)
- [API-Dokumentation](#api-dokumentation)
- [Datenbankschema](#datenbankschema)
- [Lead-Verarbeitungsfluss](#lead-verarbeitungsfluss)
- [Entwicklung](#entwicklung)
- [Tests](#tests)
- [Deployment](#deployment)
- [Fehlerbehebung](#fehlerbehebung)

## Überblick

Der Lead Gateway Service ist ein webhookbasiertes Lead-Processing-System, das:

1. **Empfängt** Leads über einen HTTP-Webhook-Endpunkt
2. **Validiert** Leads gegen Business-Regeln (Postleitzahl, Eigentümerstatus)
3. **Transformiert** Leads in das Customer-API-Format mit permissiver Attributbehandlung
4. **Liefert** Leads an die externe Customer API mit exponentiellem Backoff
5. **Protokolliert** eine vollständige Audit-Historie aller Verarbeitungsschritte

### Wichtige Funktionen

- **Schnelle Webhook-Antwort** (<500ms) mit asynchroner Verarbeitung
- **Robuste Retry-Logik** mit exponentiellem Backoff (max. 5 Versuche)
- **Permissive Transformation** – ungültige optionale Attribute werden ausgelassen, nicht abgelehnt
- **Vollständiger Audit-Trail** – alle Payloads, Versuche und Statusänderungen werden protokolliert
- **Strukturiertes Logging** mit Korrelations-IDs zur Nachverfolgung
- **Statistik-Endpunkte** für Monitoring und Observability

### Lead-Lebenszyklus

```
RECEIVED → REJECTED (Validierung fehlgeschlagen)
RECEIVED → READY → DELIVERED (Erfolgsweg)
READY → FAILED → PERMANENTLY_FAILED (Zustellung fehlgeschlagen, Retries erschöpft)
```

**Statusdefinitionen:**

- `RECEIVED`: Lead per Webhook angenommen und zur Verarbeitung eingereiht
- `REJECTED`: Lead hat Validierungsregeln verletzt
- `READY`: Lead ist validiert und transformiert, bereit zur Zustellung
- `DELIVERED`: Lead erfolgreich an Customer API gesendet
- `FAILED`: Zustellversuch fehlgeschlagen, erneuter Versuch möglich
- `PERMANENTLY_FAILED`: Max. Retry-Versuche erschöpft oder nicht wiederholbarer Fehler

## Architektur

Der Service folgt einer mehrschichtigen Architektur mit klarer Verantwortlichkeit:

```
┌─────────────────────────────────────────────────────────┐
│                     HTTP Webhook                         │
│                  POST /webhooks/leads                    │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│                   API Server (cmd/api)                   │
│  - Webhook Handler                                       │
│  - Authentifizierungs-Middleware                         │
│  - Fehlerbehandlung                                      │
└────────────────────┬────────────────────────────────────┘
                     │
                     ├──────> PostgreSQL (Lead speichern)
                     │
                     └──────> Redis/Queue (Job einreihen)
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────┐
│              Background Worker (cmd/worker)              │
│  1. Validierungsservice                                  │
│  2. Normalisierungsservice                               │
│  3. Mapping-Service                                      │
│  4. Customer API Client (mit Retry)                      │
└────────────────────┬────────────────────────────────────┘
                     │
                     ├──────> PostgreSQL (Status aktualisieren)
                     │
                     └──────> Customer API (Lead zustellen)
```

### Komponenten

- **API Server**: Nimmt Webhook-Requests entgegen, speichert Leads, reiht Jobs ein
- **Background Worker**: Verarbeitet Leads asynchron (Validierung → Transformation → Zustellung)
- **PostgreSQL**: Speichert Leads, Zustellversuche und Audit-Trail
- **Redis**: Job-Queue für asynchrone Verarbeitung
- **Customer API Client**: Zustellung mit Retry-Logik

## Projektstruktur

```
go_lead/
├── cmd/
│   ├── api/                    # API-Server Einstiegspunkt
│   │   └── main.go
│   └── worker/                 # Background-Worker Einstiegspunkt
│       └── main.go
├── internal/
│   ├── models/                 # Domänenmodelle und Typen
│   │   ├── lead.go            # InboundLead-Modell
│   │   ├── status.go          # Status-Enums
│   │   └── errors.go          # Eigene Fehlertypen
│   ├── repository/             # Datenbankzugriffsschicht
│   │   ├── lead_repository.go
│   │   └── delivery_attempt_repository.go
│   ├── services/               # Business-Logik
│   │   ├── validation.go      # Validierungsregeln
│   │   ├── normalization.go   # Normalisierung
│   │   └── mapping.go         # Attribut-Mapping
│   ├── handlers/               # HTTP-Handler
│   │   ├── webhook.go         # Webhook-Endpunkt
│   │   ├── stats.go           # Statistik-Endpunkte
│   │   └── middleware.go      # Authentifizierungs-Middleware
│   ├── worker/                 # Worker-Orchestrierung
│   │   └── processor.go       # Job-Processor
│   ├── queue/                  # Queue-Abstraktion
│   │   ├── queue.go           # Queue-Interface
│   │   └── db_queue.go        # DB-basierte Queue
│   ├── client/                 # Externe API-Clients
│   │   └── customer_api.go    # Customer API Client
│   ├── database/               # Datenbank-Utilities
│   │   ├── database.go        # Verbindungsmanagement
│   │   └── migrations.go      # Migrations-Runner
│   ├── config/                 # Konfigurationsmanagement
│   │   └── config.go
│   └── logger/                 # Strukturiertes Logging
│       └── logger.go
├── config/                     # Konfigurationsdateien
│   └── customer_attribute_mapping.json
├── migrations/                 # Datenbank-Migrationen
│   ├── 001_create_inbound_lead.sql
│   └── 002_create_delivery_attempt.sql
├── .env.example                # Beispiel-Umgebungsvariablen
├── docker-compose.yml          # Docker-Compose-Konfiguration
├── Dockerfile                  # Multi-Stage Docker Build
├── Makefile                    # Häufige Development-Tasks
├── go.mod                      # Go-Modul-Abhängigkeiten
├── go.sum                      # Abhängigkeits-Checksummen
├── README.md                   # Diese Datei (EN)
└── DOCKER.md                   # Docker-Deployment-Guide
```

## Schnellstart

### Voraussetzungen

- Docker 20.10+ und Docker Compose 2.0+ (empfohlen)
- ODER Go 1.21+, PostgreSQL 14+, Redis 6+ (für lokale Entwicklung)

### Mit Docker (empfohlen)

1. **In das Projektverzeichnis wechseln:**

   ```bash
   cd go_lead
   ```

2. **Alle Services starten:**

   ```bash
   docker-compose up --build
   ```

3. **API verfügbar unter** `http://localhost:8080`

4. **Test-Webhook senden:**

   ```bash
   curl -X POST http://localhost:8080/webhooks/leads \
     -H "Content-Type: application/json" \
     -d '{
       "email": "test@example.com",
       "phone": "+49123456789",
       "zipcode": "66123",
       "house": {
         "is_owner": true
       }
     }'
   ```

5. **Lead-Statistiken abrufen:**
   ```bash
   curl http://localhost:8080/stats/leads/counts
   ```

Weitere Docker-Kommandos: siehe [DOCKER.md](DOCKER.md).

### Lokales Development-Setup

1. **Umgebungs-Konfiguration kopieren:**

   ```bash
   cp .env.example .env
   ```

2. **`.env` anpassen** (Datenbank, Customer-API-Zugangsdaten, etc.)

3. **Abhängigkeiten installieren:**

   ```bash
   go mod download
   ```

4. **PostgreSQL und Redis starten** (Ports gemäß Konfiguration)

5. **Datenbank-Migrationen ausführen:**

   ```bash
   go run cmd/api/main.go migrate
   ```

6. **API-Server starten:**

   ```bash
   go run cmd/api/main.go
   ```

7. **In separatem Terminal den Worker starten:**
   ```bash
   go run cmd/worker/main.go
   ```

## Konfiguration

### Umgebungsvariablen

Alle Konfigurationen erfolgen über Umgebungsvariablen. Siehe `.env.example` für die vollständige Liste.

#### Datenbank-Konfiguration

```bash
DB_HOST=localhost              # Datenbank-Hostname
DB_PORT=5432                   # Datenbank-Port
DB_USER=postgres               # Datenbank-Benutzer
DB_PASSWORD=postgres           # Datenbank-Passwort
DB_NAME=lead_gateway           # Datenbank-Name
DB_SSLMODE=disable             # SSL-Modus (disable, require, verify-full)
```

#### API-Server-Konfiguration

```bash
API_PORT=8080                  # API-Server-Port
API_HOST=0.0.0.0               # API-Server-Host (0.0.0.0 für alle Interfaces)
```

#### Worker-Konfiguration

```bash
WORKER_POLL_INTERVAL=5s        # Job-Poll-Intervall
WORKER_CONCURRENCY=5           # Anzahl paralleler Worker
```

#### Queue-Konfiguration

```bash
QUEUE_TYPE=redis               # Queue-Typ (redis oder database)
REDIS_URL=redis://localhost:6379/0  # Redis-Verbindungs-URL
```

#### Customer API Konfiguration

```bash
CUSTOMER_API_URL=https://contactapi.static.fyi/lead/receive/fake/USER_ID/
CUSTOMER_API_TOKEN=FakeCustomerToken  # Customer API Endpunkt
CUSTOMER_API_TOKEN=your_bearer_token_here          # Bearer Token für Auth
CUSTOMER_API_TIMEOUT=30s                           # Request-Timeout
CUSTOMER_PRODUCT_NAME=solar_panel_installation     # Produktname
```

#### Retry-Konfiguration

```bash
MAX_RETRY_ATTEMPTS=5           # Maximale Zustellversuche
RETRY_BACKOFF_BASE=30s         # Basis-Delay für exponentiellen Backoff
```

**Retry-Zeitplan:**

- Versuch 1: Sofort
- Versuch 2: 30s Verzögerung
- Versuch 3: 60s Verzögerung
- Versuch 4: 120s Verzögerung
- Versuch 5: 240s Verzögerung
- Nach 5 Versuchen: Status `PERMANENTLY_FAILED`

#### Authentifizierung (optional)

```bash
ENABLE_AUTH=false              # Shared-Secret-Authentifizierung aktivieren
SHARED_SECRET=your_secret      # Shared Secret für Webhook-Authentifizierung
```

Wenn `ENABLE_AUTH=true`, müssen Webhook-Requests enthalten:

```
X-Shared-Secret: your_secret
```

#### Logging

```bash
LOG_LEVEL=info                 # Log-Level (debug, info, warn, error)
LOG_FORMAT=json                # Log-Format (json oder text)
```

#### Attribut-Mapping-Konfiguration

```bash
ATTRIBUTE_MAPPING_FILE=./config/customer_attribute_mapping.json
```

### Attribut-Mapping-Konfigurationsdatei

Die Datei `customer_attribute_mapping.json` definiert Validierungsregeln für Lead-Attribute:

```json
{
  "solar_energy_consumption": {
    "attribute_type": "text",
    "is_numeric": true,
    "values": null
  },
  "solar_offer_type": {
    "attribute_type": "dropdown",
    "is_numeric": false,
    "values": ["Beides interessant", "Mieten", "Kaufen"]
  },
  "solar_area": {
    "attribute_type": "range",
    "is_numeric": true,
    "values": null
  }
}
```

**Attributtypen:**

- `text`: Freitext (optional numerisch)
- `dropdown`: Muss exakt einem der Werte entsprechen
- `range`: Numerischer Wert innerhalb eines Bereichs

**Validierungsverhalten:**

- **Pflichtfelder** (`phone`, `product.name`): Fehlende Werte führen zu FAILED
- **Optionale Attribute**: Ungültige Werte werden ausgelassen (permissive Verarbeitung)

## API-Dokumentation

### Webhook-Endpunkt

#### POST /webhooks/leads

Empfängt Lead-Webhooks zur asynchronen Verarbeitung.

**Request-Header:**

```
Content-Type: application/json
X-Shared-Secret: your_secret (falls Auth aktiviert)
```

**Request-Body:**

```json
{
  "email": "customer@example.com",
  "phone": "+49123456789",
  "zipcode": "66123",
  "house": {
    "is_owner": true
  },
  "solar_energy_consumption": "5000",
  "solar_offer_type": "Kaufen"
}
```

**Erfolgsantwort (200 OK):**

```json
{
  "lead_id": 123,
  "status": "RECEIVED",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response-Header:**

```
X-Correlation-ID: 550e8400-e29b-41d4-a716-446655440000
```

**Fehlerantworten:**

- **400 Bad Request** – Ungültiger JSON-Payload

  ```json
  {
    "error": "malformed JSON payload",
    "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
  }
  ```

- **401 Unauthorized** – Ungültiges/fehlendes Shared Secret

  ```json
  {
    "error": "unauthorized",
    "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
  }
  ```

- **503 Service Unavailable** – Datenbank oder Queue nicht verfügbar

  ```json
  {
    "error": "database error",
    "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
  }
  ```

- **500 Internal Server Error** – Unerwarteter Fehler
  ```json
  {
    "error": "internal server error",
    "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
  }
  ```

### Statistik-Endpunkte

#### GET /stats/leads/counts

Gibt Lead-Zahlen nach Status gruppiert zurück.

**Antwort (200 OK):**

```json
{
  "received": 10,
  "rejected": 2,
  "ready": 3,
  "delivered": 45,
  "failed": 1,
  "permanently_failed": 0,
  "total": 61
}
```

#### GET /stats/leads/recent

Gibt die 50 zuletzt empfangenen Leads in absteigender Reihenfolge zurück.

**Antwort (200 OK):**

```json
[
  {
    "id": 123,
    "received_at": "2026-01-21T10:30:00Z",
    "status": "DELIVERED",
    "rejection_reason": null
  },
  {
    "id": 122,
    "received_at": "2026-01-21T10:25:00Z",
    "status": "REJECTED",
    "rejection_reason": "ZIP_NOT_66XXX"
  }
]
```

#### GET /stats/leads/{id}/history

Gibt die vollständige Historie eines Leads inklusive Zustellversuchen zurück.

**Antwort (200 OK):**

```json
{
  "id": 123,
  "received_at": "2026-01-21T10:30:00Z",
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
      "attempted_at": "2026-01-21T10:30:05Z",
      "success": true,
      "status_code": 200,
      "error_message": null
    }
  ]
}
```

**Fehlerantwort (404 Not Found):**

```json
{
  "error": "lead not found"
}
```

### Health-Check-Endpunkt

#### GET /health

Gibt den Service-Status zurück.

**Antwort (200 OK):**

```
OK
```

## Datenbankschema

### Tabelle: inbound_lead

Speichert alle eingehenden Leads mit Status und Payloads.

```sql
CREATE TABLE inbound_lead (
    id SERIAL PRIMARY KEY,
    received_at TIMESTAMP NOT NULL DEFAULT NOW(),
    raw_payload JSONB NOT NULL,
    source_headers JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'RECEIVED',
    rejection_reason VARCHAR(100),
    normalized_payload JSONB,
    customer_payload JSONB,
    payload_hash VARCHAR(64) UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT check_status CHECK (
        status IN ('RECEIVED', 'REJECTED', 'READY', 'DELIVERED', 'FAILED', 'PERMANENTLY_FAILED')
    )
);

CREATE INDEX idx_inbound_lead_status ON inbound_lead(status);
CREATE INDEX idx_inbound_lead_received_at ON inbound_lead(received_at);
CREATE INDEX idx_inbound_lead_payload_hash ON inbound_lead(payload_hash);
```

**Spalten:**

- `id`: Eindeutige Lead-ID
- `received_at`: Zeitpunkt des Empfangs
- `raw_payload`: Originaler JSON-Payload
- `source_headers`: HTTP-Header des Requests
- `status`: Aktueller Verarbeitungsstatus
- `rejection_reason`: Ablehnungsgrund (z. B. `ZIP_NOT_66XXX`, `NOT_HOMEOWNER`)
- `normalized_payload`: Normalisierter Payload
- `customer_payload`: Payload für Customer API
- `payload_hash`: SHA-256 Hash zur Deduplizierung (optional)
- `created_at`: Erstellungszeitpunkt
- `updated_at`: Letzte Aktualisierung

### Tabelle: delivery_attempt

Speichert alle Zustellversuche an die Customer API.

```sql
CREATE TABLE delivery_attempt (
    id SERIAL PRIMARY KEY,
    lead_id INTEGER NOT NULL REFERENCES inbound_lead(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL,
    requested_at TIMESTAMP NOT NULL DEFAULT NOW(),
    response_status INTEGER,
    response_body TEXT,
    error_message TEXT,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT check_attempt_no CHECK (attempt_no > 0),
    CONSTRAINT check_response_status CHECK (
        response_status IS NULL OR (response_status >= 100 AND response_status < 600)
    )
);

CREATE INDEX idx_delivery_attempt_lead_id ON delivery_attempt(lead_id);
CREATE INDEX idx_delivery_attempt_success ON delivery_attempt(success);
CREATE INDEX idx_delivery_attempt_requested_at ON delivery_attempt(requested_at);
CREATE UNIQUE INDEX idx_delivery_attempt_lead_attempt ON delivery_attempt(lead_id, attempt_no);
```

**Spalten:**

- `id`: Eindeutige Attempt-ID
- `lead_id`: Foreign Key zu inbound_lead
- `attempt_no`: Laufende Versuchszahl (1-basiert)
- `requested_at`: Zeitpunkt des Zustellversuchs
- `response_status`: HTTP-Statuscode der Customer API (null bei Netzwerkfehler)
- `response_body`: Antwort-Body (bei Erfolg)
- `error_message`: Fehlermeldung bei Fehlschlag
- `success`: Ob die Zustellung erfolgreich war
- `created_at`: Erstellungszeitpunkt

### Datenbank-Migrationen

Migrationen liegen im Verzeichnis `migrations/` und werden beim Start automatisch angewendet.

**Manuelle Migration:**

```bash
go run cmd/api/main.go migrate
```

**Rollback (falls implementiert):**

```bash
go run cmd/api/main.go migrate-down
```

## Lead-Verarbeitungsfluss

### 1. Webhook-Empfang (API Server)

1. POST-Request an `/webhooks/leads` empfangen
2. Korrelations-ID erzeugen
3. JSON-Payload validieren
4. Authentifizierung prüfen (falls aktiviert)
5. Lead in DB speichern mit Status `RECEIVED`
6. Hintergrund-Job mit lead_id einreihen
7. 200 OK Antwort (<500ms)

### 2. Validierung (Background Worker)

1. Job aus der Queue holen
2. Lead aus der DB laden
3. PLZ muss `^66\d{3}$` entsprechen
4. `house.is_owner` muss exakt `true` sein
5. Wenn Validierung fehlschlägt:
   - Status auf `REJECTED`
   - Ablehnungsgrund speichern
   - Verarbeitung stoppen
6. Wenn erfolgreich:
   - Status auf `READY`
   - Transformation fortsetzen

**Ablehnungsgründe:**

- `ZIP_NOT_66XXX`: PLZ erfüllt Pattern nicht
- `NOT_HOMEOWNER`: house.is_owner ist nicht `true`
- `MISSING_REQUIRED_FIELD`: Pflichtfeld fehlt

### 3. Transformation (Background Worker)

1. **Normalisierung:**
   - E-Mail-Adressen kleinschreiben und trimmen
   - Telefonnummern standardisieren
   - Whitespace trimmen
   - Boolean-Strings zu Booleans konvertieren

2. **Mapping:**
   - Attributdefinitionen aus der Konfiguration laden
   - Pflichtfelder (`phone`, `product.name`) prüfen
   - Optionale Attribute validieren
   - **Permissive Behandlung**: Ungültige optionale Attribute werden ausgelassen
   - Customer-Payload erzeugen

3. **Payloads speichern:**
   - `normalized_payload` in DB speichern
   - `customer_payload` in DB speichern

4. **Fehlerbehandlung:**
   - Pflichtfelder fehlen: Status `FAILED`, Verarbeitung stoppen
   - Ungültige optionale Attribute: Auslassen, weiterverarbeiten

### 4. Zustellung (Background Worker)

1. POST an Customer API mit Bearer Token
2. `delivery_attempt` erstellen
3. Response-Handling:
   - **2xx**: Status `DELIVERED`, Response speichern
   - **4xx** (außer 429): `PERMANENTLY_FAILED`, kein Retry
   - **5xx oder Netzwerkfehler**: Retry mit Backoff

4. **Retry-Logik:**
   - Versuch 1: Sofort
   - Versuch 2: 30s
   - Versuch 3: 60s
   - Versuch 4: 120s
   - Versuch 5: 240s
   - Danach: Status `PERMANENTLY_FAILED`

5. **Atomare Updates:**
   - Status-Update und Attempt-Erstellung atomar
   - Verhindert Race-Conditions und Inkonsistenzen

## Entwicklung

### Abhängigkeiten

```bash
# Abhängigkeiten installieren
go mod download

# Abhängigkeiten aktualisieren
go mod tidy

# Neue Abhängigkeit hinzufügen
go get github.com/example/package
```

### Tests ausführen

```bash
# Alle Tests
go test ./...

# Tests mit Coverage
go test -cover ./...

# Tests mit verbose Output
go test -v ./...

# Tests für ein bestimmtes Paket
go test ./internal/services/...

# Property-based Tests (100+ Iterationen)
go test -v ./internal/services/ -run TestValidation

# Coverage-Report erzeugen
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Build

```bash
# API-Server bauen
go build -o bin/api cmd/api/main.go

# Worker bauen
go build -o bin/worker cmd/worker/main.go

# Beide bauen
go build -o bin/ ./cmd/...

# Build mit Optimierungen
go build -ldflags="-s -w" -o bin/api cmd/api/main.go
```

### Makefile verwenden

```bash
make help          # Alle verfügbaren Kommandos anzeigen
make build         # Docker-Images bauen
make up            # Alle Services starten
make down          # Services stoppen
make logs          # Logs ansehen
make test          # Tests ausführen
make test-webhook  # Test-Webhook senden
make health        # API-Health prüfen
make stats         # Lead-Statistiken abrufen
make clean         # Container und Volumes entfernen
```

### Code-Qualität

```bash
# Code formatieren
go fmt ./...

# Linting (erfordert golangci-lint)
golangci-lint run

# Vet
go vet ./...

# Security Checks (erfordert gosec)
gosec ./...
```

## Tests

### Unit-Tests

Unit-Tests prüfen einzelne Funktionen und Edge-Cases:

```bash
# Unit-Tests
go test ./internal/services/...
go test ./internal/repository/...
go test ./internal/handlers/...
```

### Property-Based Tests

Property-Based Tests prüfen allgemeine Eigenschaften über viele generierte Inputs:

```bash
# Property-Tests (100+ Iterationen je Test)
go test -v ./internal/services/ -run Property
go test -v ./internal/handlers/ -run Property
go test -v ./internal/worker/ -run Property
```

**Beispiele getesteter Eigenschaften:**

- PLZ-Validierung ist konsistent
- Eigentümer-Validierung ist konsistent
- Normalisierung ist idempotent (zweimal = einmal)
- Fehlende Pflichtfelder führen immer zum Fehler
- Ungültige optionale Attribute werden immer ausgelassen
- Webhook-Antwortzeit <500ms
- Max. Retries führen immer zu permanentem Fehler
- 4xx Fehler triggern keinen Retry

### Integrationstests

Integrationstests prüfen End-to-End-Flows:

```bash
# Integrationstests
go test -v ./internal/integration/...
```

**Test-Szenarien:**

- Voller Flow: Webhook → Validierung → Transformation → Zustellung
- Retry-Verhalten bei fehlschlagender Customer API
- Fehlerszenarien (DB nicht verfügbar, Queue nicht verfügbar)

### Testabdeckung

```bash
# Coverage-Report erzeugen
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Coverage-Zusammenfassung
go test -cover ./...
```

## Deployment

### Docker-Deployment

Siehe [DOCKER.md](DOCKER.md) für detaillierte Docker-Deployment-Anweisungen.

**Schnellstart:**

```bash
docker-compose up --build
```

### Hinweise für Produktion

1. **Umgebungsvariablen:**
   - Secrets-Management nutzen (AWS Secrets Manager, HashiCorp Vault)
   - `.env` niemals ins Repository committen

2. **Datenbank:**
   - SSL/TLS aktivieren (`DB_SSLMODE=require`)
   - Automatische Backups einrichten
   - Connection-Pooling konfigurieren
   - Query-Performance überwachen

3. **Sicherheit:**
   - Authentifizierung aktivieren (`ENABLE_AUTH=true`)
   - Starke Shared Secrets verwenden
   - Customer-API-Tokens regelmäßig rotieren
   - HTTPS für externe Kommunikation
   - Firewall-Regeln setzen

4. **Monitoring:**
   - Log-Aggregation (ELK, Splunk, CloudWatch)
   - Alerts für Fehler und langsame Operationen
   - Queue-Tiefe und Worker-Lag überwachen
   - Zustellerfolgsrate tracken

5. **Skalierung:**
   - API-Server horizontal skalieren
   - Worker nach Queue-Last skalieren
   - Read Replicas für Statistik-Queries
   - Redis Cluster für hohe Last erwägen

6. **Hochverfügbarkeit:**
   - Mehrere API-Server-Instanzen
   - Mehrere Worker-Instanzen
   - PostgreSQL-Replikation
   - Redis Sentinel/Cluster

7. **Performance:**
   - DB-Connection-Pool optimieren
   - Worker-Concurrency an Last anpassen
   - Langsame Queries optimieren
   - Caching für Statistik-Endpunkte erwägen

## Fehlerbehebung

### Häufige Probleme

#### API-Server startet nicht

**Symptom:** API-Server beendet sich sofort oder startet nicht

**Lösungen:**

1. Datenbankverbindung prüfen:

   ```bash
   docker-compose exec postgres pg_isready -U postgres
   ```

2. Umgebungsvariablen prüfen:

   ```bash
   cat .env
   ```

3. Logs prüfen:

   ```bash
   docker-compose logs api
   ```

4. Portverfügbarkeit prüfen:
   ```bash
   netstat -an | grep 8080
   ```

#### Worker verarbeitet keine Leads

**Symptom:** Leads bleiben im Status RECEIVED

**Lösungen:**

1. Worker-Logs prüfen:

   ```bash
   docker-compose logs worker
   ```

2. Queue-Konnektivität prüfen:

   ```bash
   docker-compose exec redis redis-cli ping
   ```

3. Worker-Status prüfen:

   ```bash
   docker-compose ps worker
   ```

4. Datenbankzugriff aus dem Worker prüfen

#### Leads bleiben in FAILED

**Symptom:** Leads gehen nicht in DELIVERED über

**Lösungen:**

1. Customer-API-Konnektivität prüfen:

   ```bash
   curl -H "Authorization: Bearer $CUSTOMER_API_TOKEN" $CUSTOMER_API_URL
   ```

2. Zustellversuche prüfen:

   ```bash
   curl http://localhost:8080/stats/leads/{id}/history
   ```

3. Customer-API-Zugangsdaten in `.env` prüfen

4. Worker-Logs auf Fehler prüfen

#### Datenbankverbindungsfehler

**Symptom:** 503 Service Unavailable

**Lösungen:**

1. PostgreSQL läuft?

   ```bash
   docker-compose ps postgres
   ```

2. DB-Credentials in `.env` prüfen

3. DB-Logs prüfen:

   ```bash
   docker-compose logs postgres
   ```

4. Netzwerkverbindung prüfen:
   ```bash
   docker-compose exec api ping postgres
   ```

### Debugging

#### Debug-Logging aktivieren

```bash
# In .env
LOG_LEVEL=debug
```

#### Strukturierte Logs ansehen

```bash
# Alle Services
docker-compose logs -f

# Einzelne Services
docker-compose logs -f api
docker-compose logs -f worker

# Logs filtern
docker-compose logs -f | grep ERROR
docker-compose logs -f | grep lead_id=123
```

#### Datenbankzugriff

```bash
# PostgreSQL verbinden
docker-compose exec postgres psql -U postgres -d lead_gateway

# Leads abfragen
SELECT id, status, received_at FROM inbound_lead ORDER BY received_at DESC LIMIT 10;

# Zustellversuche abfragen
SELECT * FROM delivery_attempt WHERE lead_id = 123;

# Status-Zählung
SELECT status, COUNT(*) FROM inbound_lead GROUP BY status;
```

#### Redis-Queue prüfen

```bash
# Redis verbinden
docker-compose exec redis redis-cli

# Queue-Länge
LLEN queue:process_lead

# Jobs anzeigen
LRANGE queue:process_lead 0 -1
```

#### Webhook manuell testen

```bash
# Gültiger Lead (soll akzeptiert werden)
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {"is_owner": true}
  }'

# Ungültige PLZ (soll abgelehnt werden)
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "phone": "+49123456789",
    "zipcode": "12345",
    "house": {"is_owner": true}
  }'

# Nicht-Eigentümer (soll abgelehnt werden)
curl -X POST http://localhost:8080/webhooks/leads \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "phone": "+49123456789",
    "zipcode": "66123",
    "house": {"is_owner": false}
  }'
```

### Performance-Tuning

#### Datenbank

```bash
# Connection-Pool erhöhen
DB_MAX_OPEN_CONNS=50
DB_MAX_IDLE_CONNS=10
```

#### Worker

```bash
# Worker-Concurrency erhöhen
WORKER_CONCURRENCY=10

# Poll-Intervall reduzieren
WORKER_POLL_INTERVAL=1s
```

#### Customer API

```bash
# Timeout erhöhen
CUSTOMER_API_TIMEOUT=60s
```
