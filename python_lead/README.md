# Lead Gateway Service

Ein REST-Webhook Anwendung, der eingehende Leads prüft, umwandelt und an eine Kunden‑API sendet. Der Dienst nutzt Celery für asynchrone Aufgaben, Redis als Message‑Broker und PostgreSQL für die Speicherung.

## Problemüberblick

Externe Lead‑Quellen sollen Leads an ein Kundensystem senden. Dafür braucht es:

1. **Schnelle Bestätigung** – Antwort in < 500 ms, damit es keine Timeouts gibt
2. **Regel‑Prüfung** – Nur qualifizierte Leads (bestimmte Postleitzahlen, Eigenheimbesitzer)
3. **Daten‑Umwandlung** – Lead‑Daten müssen zum Kunden‑Format passen
4. **Zuverlässige Zustellung** – Fehler dürfen keine Leads verlieren lassen
5. **Vollständige Nachvollziehbarkeit** – Alle Schritte müssen gespeichert werden

Dieser Service löst das so:

- Nimmt Leads sofort an und legt sie in eine Warteschlange (schnelle Antwort)
- Prüft konfigurierbare Regeln vor dem Versand (Qualitätskontrolle)
- Nutzt JSON‑Mapping für flexible Transformation (anpassbar)
- Wiederholt bei Fehlern mit exponentiellem Backoff (zuverlässig)
- Speichert Rohdaten, Header und Status‑Übergänge (Audit)

## Inhalt

- [Architektur](#architektur)
- [Funktionen](#funktionen)
- [Schnellstart](#schnellstart)
- [API‑Referenz](#api-referenz)
- [Lead‑Pipeline](#lead-pipeline)
- [Validierungsregeln](#validierungsregeln)
- [Konfiguration](#konfiguration)
- [Tests](#tests)
- [Projektstruktur](#projektstruktur)
- [Geplante Erweiterungen](#geplante-erweiterungen)

## Architektur

```
┌─────────────────┐
│  Externe Quelle │
│     (Lead)      │
└────────┬────────┘
         │ HTTP POST /webhooks/leads/
         ▼
┌─────────────────────────────────────────────────┐
│           Django REST API (Webhook)             │
│  - Nimmt Lead an                                 │
│  - Speichert Rohdaten                            │
│  - Plant Celery‑Task                              │
│  - Antwortet sofort mit 200 OK                    │
└────────┬────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────┐
│              Redis (Message Broker)             │
└────────┬────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────┐
│            Celery Worker (Async)                │
│  1. Validieren                                   │
│  2. Normalisieren                                │
│  3. Mappen                                       │
│  4. Senden                                       │
│  5. Retry                                        │
└────────┬────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────┐
│         PostgreSQL (Persistenz)                  │
│  - InboundLead Tabelle                           │
│  - DeliveryAttempt Tabelle                       │
└─────────────────────────────────────────────────┘
```

## Funktionen

- **Schnelle Webhook‑Antwort**: 200 OK innerhalb von 500 ms
- **Regelprüfung**: Validiert PLZ und Eigenheimbesitz
- **Flexible Transformation**: Ungültige optionale Felder werden ausgelassen
- **Zuverlässige Zustellung**: Retry mit exponentiellem Backoff
- **Klare End‑Status**: 4xx → PERMANENTLY_FAILED, Retry‑Limit → PERMANENTLY_FAILED
- **Audit‑Trail**: Speichert Rohdaten, Header und Status‑Übergänge
- **Konfigurierbar**: JSON‑Mapping und Regeln

## Schnellstart

### Mit Docker Compose (empfohlen)

```bash
# Repository klonen
git clone <repository-url>
cd lead-gateway-service

# Alle Services starten
docker-compose up -d

# Migrationen ausführen
docker-compose exec web python manage.py migrate

# Superuser anlegen (optional)
docker-compose exec web python manage.py createsuperuser

# Logs ansehen
docker-compose logs -f

### Datenbank stoppen

# Docker stoppen
docker-compose down

# Docker stoppen UND Daten entfernen
docker-compose down -v

---
```

Der Service ist unter `http://localhost:8000` erreichbar.

### Lokale Instellation (ohne Docker)

```bash
# Virtuelle Umgebung
python -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# Abhängigkeiten installieren
pip install -r requirements.txt

# Umgebungsvariablen setzen
export POSTGRES_HOST=localhost
export POSTGRES_DB=lead_gateway
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export CELERY_BROKER_URL=redis://localhost:6379/0

# Migrationen
python manage.py migrate

# Django starten
python manage.py runserver

# Celery Worker starten (zweites Terminal)
celery -A lead_gateway worker --loglevel=info
```

## API‑Referenz

### POST /webhooks/leads/

Nimmt einen neuen Lead an.

**Header:**

```
Content-Type: application/json
X-Shared-Secret: <optional_auth_token>  # falls Auth aktiv ist
```

**Body:**

```json
{
  "email": "user@example.com",
  "address": {
    "zip": "66123",
    "street": "123 Main St"
  },
  "house": {
    "is_owner": true
  }
}
```

**Antwort (200 OK):**

```json
{
  "status": "accepted",
  "lead_id": 123,
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Fehlerantworten:**

| Status | Beschreibung                          |
| ------ | ------------------------------------- |
| 400    | Ungültiges JSON                       |
| 401    | Falsche Authentifizierung (optional)  |
| 500    | Interner Serverfehler                 |
| 503    | Dienst nicht verfügbar                |

### Beispiel

```bash
curl -X POST http://localhost:8000/webhooks/leads/ \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "address": {
      "zip": "66123",
      "street": "123 Main St"
    },
    "house": {
      "is_owner": true
    }
  }'
```

## Lead‑Pipeline

### Status‑Ablauf

```
RECEIVED → REJECTED (Validierung fehlgeschlagen)
RECEIVED → READY → DELIVERED (Erfolg)
READY → FAILED → PERMANENTLY_FAILED (Retries erschöpft)
READY → PERMANENTLY_FAILED (4xx Fehler)
```

### Status‑Definitionen

| Status             | Beschreibung                                           |
| ------------------ | ------------------------------------------------------ |
| RECEIVED           | Lead angenommen und in Warteschlange                   |
| REJECTED           | Validierung fehlgeschlagen (Ende)                      |
| READY              | Validiert und transformiert                            |
| DELIVERED          | Erfolgreich an Kunden‑API gesendet (Ende)              |
| FAILED             | Vorübergehender Fehler, Retry folgt                    |
| PERMANENTLY_FAILED | Endgültiger Fehler (4xx oder Retries erschöpft)         |

### Schritte

1. **Empfang**: Webhook nimmt Lead an und speichert Rohdaten
2. **Validierung**: PLZ‑Muster und Eigenheimbesitz prüfen
3. **Normalisierung**: E‑Mail kleinschreiben, Leerzeichen trimmen, Booleans konvertieren
4. **Transformation**: Felder per Mapping in Kunden‑Format bringen
5. **Zustellung**: An Kunden‑API mit Bearer‑Token senden
6. **Protokoll**: Zustellversuch mit Antwort speichern

## Validierungsregeln

Leads müssen alle Regeln bestehen:

| Regel            | Muster/Wert                           | Ablehnungs‑Code         |
| --------------- | -------------------------------------- | ----------------------- |
| Postleitzahl     | `^66\d{3}$` (z. B. 66000–66999)        | ZIP_NOT_66XXX           |
| Eigenheimbesitz  | `house.is_owner` muss `true` sein       | NOT_HOMEOWNER           |
| Pflichtfelder    | `address.zip`, `house.is_owner` nötig   | MISSING_REQUIRED_FIELD  |

### Gültige PLZ‑Beispiele

- ✅ 66000, 66123, 66999
- ❌ 12345, 65999, 67000, 6612, 661234

## Konfiguration

### Umgebungsvariablen

| Variable                | Beschreibung                    | Standard                 |
| ----------------------- | ------------------------------- | ------------------------ |
| `DJANGO_SECRET_KEY`     | Django Secret Key               | `dev-secret-key-...`     |
| `DEBUG`                 | Debug‑Modus                     | `True`                   |
| `ALLOWED_HOSTS`         | Erlaubte Hosts (CSV)            | `localhost,127.0.0.1`    |
| `POSTGRES_DB`           | Datenbankname                   | `lead_gateway`           |
| `POSTGRES_USER`         | DB‑Benutzer                     | `postgres`               |
| `POSTGRES_PASSWORD`     | DB‑Passwort                     | `postgres`               |
| `POSTGRES_HOST`         | DB‑Host                         | `localhost`              |
| `POSTGRES_PORT`         | DB‑Port                         | `5432`                   |
| `CELERY_BROKER_URL`     | Redis‑URL                       | `redis://localhost:6379/0` |
| `CUSTOMER_API_URL`      | Kunden‑Endpoint                 | `https://contactapi.static.fyi/lead/receive/fake/USER_ID` |
| `CUSTOMER_TOKEN`        | Bearer‑Token                    | `FakeCustomerToken`      |
| `CUSTOMER_PRODUCT_NAME` | Produktname                     | `Solaranlage`            |
| `ATTRIBUTE_MAPPING_PATH`| Pfad zur Mapping‑Datei          | `customer_attribute_mapping.json` |
| `WEBHOOK_SHARED_SECRET` | Optionales Shared‑Secret        | `None`                   |

### Feld‑Mapping

Die Datei `config/customer_attribute_mapping.json` definiert das Mapping:

```json
{
  "customer_email": "email",
  "customer_zip": "address.zip",
  "customer_street": "address.street",
  "is_homeowner": "house.is_owner"
}
```

- **Key**: Ziel‑Feld in der Kunden‑API
- **Value**: Pfad im Quell‑Payload (Dot‑Notation)

**Permissive Transformation**: Ungültige optionale Felder werden still ausgelassen. Nur `phone` und `product.name` sind Pflichtfelder.

### Retry‑Konfiguration

| Einstellung        | Wert                                          |
| ------------------ | --------------------------------------------- |
| Max Retries        | 5                                             |
| Start‑Backoff      | 30 Sekunden                                   |
| Strategie          | Exponentiell (30s, 60s, 120s, 240s, 480s)      |
| Retry bei          | 5xx, Netzwerk‑Timeouts, Verbindungsfehler     |
| Kein Retry bei     | 4xx → PERMANENTLY_FAILED sofort               |
| Limit erreicht     | Nach 5 Retries → PERMANENTLY_FAILED           |

## Tests

### Alle Tests

```bash
pytest
pytest -v
pytest --cov=leads
pytest leads/tests/test_validation.py
```

### E2E‑Tests (Live Async)

Diese Tests laufen gegen den gestarteten Docker‑Stack (web + celery + db + redis + mock customer API)
und prüfen den kompletten asynchronen Ablauf vom Webhook bis zur Zustellung an die Kunden‑API.

1) Stack mit E2E‑Override starten:

```bash
docker-compose -f docker-compose.yml -f docker-compose.e2e.yml up -d --build
```

2) Live‑E2E‑Tests starten (Windows cmd):

```bash
set LIVE_E2E=1
pytest leads\tests\test_e2e_live_async.py
```

Optional, falls Ports geändert wurden:

```bash
set LIVE_E2E_API_BASE_URL=http://localhost:8004
set LIVE_E2E_MOCK_BASE_URL=http://localhost:18080
```

### Test‑Kategorien

| Kategorie        | Beschreibung                       | Anzahl |
| --------------- | ---------------------------------- | ------ |
| Unit‑Tests       | Beispiel‑ und Edge‑Cases           | 68     |
| Property‑Tests   | Allgemeine Eigenschaften           | 2      |
| **Gesamt**       |                                    | **70** |

### Abdeckung

- Validierung: PLZ, Eigenheimbesitz, Pflichtfelder
- Normalisierung: E‑Mail, Leerzeichen, Boolean‑Konvertierung
- Mapping: Verschachtelte Felder, fehlende Felder
- Kunden‑Client: Erfolg/Fehler, Netzwerkfehler
- Celery‑Tasks: End‑to‑End, Validierungsfehler, Retries
- Webhook: Submission, Header, Auth

## Projektstruktur

```
Checkfox/
├── __pycache__/
├── conftest.py
├── customer_attribute_mapping.json
├── customer_doc.pdf
├── docker-compose.yml
├── docker-compose.e2e.yml
├── Dockerfile
├── lead_gateway/
│   ├── __init__.py
│   ├── asgi.py
│   ├── celery.py
│   ├── settings.py
│   ├── urls.py
│   ├── wsgi.py
│   └── __pycache__/
├── leads/
│   ├── __init__.py
│   ├── admin.py
│   ├── apps.py
│   ├── migrations/
│   │   ├── __init__.py
│   │   └── 0001_initial.py
│   ├── models.py
│   ├── services/
│   │   ├── __init__.py
│   │   ├── customer_client.py
│   │   ├── mapping.py
│   │   ├── normalization.py
│   │   └── validation.py
│   ├── tasks.py
│   ├── tests/
│   │   ├── __init__.py
│   │   ├── test_e2e_live_async.py
│   │   ├── test_e2e_webhook.py
│   │   ├── test_customer_client.py
│   │   ├── test_mapping.py
│   │   ├── test_normalization.py
│   │   ├── test_tasks.py
│   │   ├── test_validation.py
│   │   ├── test_validation_properties.py
│   │   └── test_views.py
│   ├── urls.py
│   ├── views.py
│   └── __pycache__/
├── manage.py
├── Primest-Onboarding-DevTask-v2.pdf
├── pytest.ini
├── README.md
├── tools/
│   └── mock_customer_api.py
└── requirements.txt
```

## Geplante Erweiterungen

### Tier 2 – Optionale Erweiterungen

- **Deduplizierung**: Doppelte Leads innerhalb von 24 Stunden erkennen
- **Strukturiertes Logging**: JSON‑Logs mit Korrelations‑IDs
- **Django Admin**: UI zur Prüfung von Leads und Zustellungen
- **Statistik‑Endpunkte**: Lead‑Zahlen nach Status
- **Webhook‑Authentifizierung**: Shared‑Secret prüfen

### Tier 3 – Erweiterte Features

- **Rate Limiting**: Schutz vor Überlastung (100 req/min)
- **Hot‑Reload Konfiguration**: Mapping ohne Neustart
- **Erweiterte Fehlerbehandlung**: Timeout‑Erkennung, Fallback‑Queue
- **Vollständige Property‑Tests**: 47 Eigenschaften
- **Metriken & Observability**: Prometheus/StatsD

