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
- [ngrok Setup für externe Webhooks](#ngrok-setup-für-externe-webhooks)
- [API‑Referenz](#api-referenz)
- [Lead‑Pipeline](#lead-pipeline)
- [Validierungsregeln](#validierungsregeln)
- [Konfiguration](#konfiguration)
- [Tests](#tests)
- [Projektstruktur](#projektstruktur)
- [Geplante Erweiterungen](#geplante-erweiterungen)

**📖 Zusätzliche Dokumentation:**
- [NGROK.md](NGROK.md) - Detaillierte ngrok Setup-Anleitung

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
cd python_lead

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

Der Service ist unter `http://localhost:8004` erreichbar.


## ngrok Setup für externe Webhooks

ngrok erstellt einen sicheren Tunnel von einer öffentlichen URL zu deinem lokalen Server. Das ist perfekt zum Testen von Webhooks während der Entwicklung.

### Wie ngrok funktioniert

```
Lead Generator (extern)
   ↓
https://abcd1234.ngrok.io/webhooks/leads/  ← Öffentliche URL
   ↓
ngrok Tunnel (verschlüsselt)
   ↓
http://localhost:8004/webhooks/leads/      ← Deine lokale Django App
   ↓
Lead Gateway Service
```

Der Lead Generator kann jetzt deine lokale App erreichen, als wäre sie im Internet verfügbar.

### Schnellstart mit ngrok

#### 1️⃣ Django App starten

Stelle sicher, dass dein Webhook-Endpunkt läuft:

```bash
# Mit Docker
docker-compose up -d

# Oder lokal
python manage.py runserver 8004
```

Dein Webhook muss erreichbar sein unter:
```
http://localhost:8004/webhooks/leads/
```

#### 2️⃣ ngrok installieren

```bash
# macOS
brew install ngrok

# Linux
snap install ngrok

# Windows
choco install ngrok
```

Oder herunterladen von: https://ngrok.com/download

**Lokale Binary-Option (Windows/Linux/macOS):**
- Lege `ngrok.exe` (Windows) oder `ngrok` (Linux/macOS) direkt in den Ordner [python_lead](python_lead)
- Die Scripts [python_lead/start_ngrok.bat](python_lead/start_ngrok.bat) und [python_lead/start_ngrok.sh](python_lead/start_ngrok.sh) verwenden automatisch die lokale Datei

#### 3️⃣ ngrok authentifizieren (einmalig)

Erstelle einen kostenlosen Account auf https://ngrok.com, dann:

```bash
ngrok config add-authtoken DEIN_TOKEN
```

#### 4️⃣ Tunnel starten

**Option A: Mit Helper-Script (empfohlen)**

```bash
# Linux/macOS
chmod +x start_ngrok.sh
./start_ngrok.sh

# Windows
start_ngrok.bat
```

**Option B: Direkt mit ngrok**

```bash
ngrok http 8004
```

**Option C: Mit Konfigurationsdatei**

```bash
ngrok start --all --config=ngrok.yml
```

#### 4️⃣b Django `ALLOWED_HOSTS` für ngrok setzen

Damit Django die ngrok-Domain akzeptiert, setze `ALLOWED_HOSTS` (z. B. in `.env` oder `docker-compose.yml`):

```
ALLOWED_HOSTS=localhost,127.0.0.1,.ngrok-free.app,.ngrok.io
```

#### 5️⃣ Öffentliche URL verwenden

Du siehst eine Ausgabe wie:

```
Forwarding  https://abcd1234.ngrok.io -> http://localhost:8004
```

Verwende diese URL für deinen Lead Generator:
```
https://abcd1234.ngrok.io/webhooks/leads/
```

#### 6️⃣ Requests inspizieren

Öffne die ngrok Web-UI:
```
http://localhost:4040
```

Hier siehst du:
- Alle eingehenden Requests
- Request/Response Headers
- Request/Response Bodies
- Replay-Funktion zum Wiederholen von Requests

### ngrok Konfiguration anpassen

Bearbeite `ngrok.yml` für erweiterte Optionen:

```yaml
tunnels:
  django-webhook:
    proto: http
    addr: 8004
    # Benutzerdefinierte Subdomain (ngrok Pro)
    subdomain: my-lead-gateway
    # Basic Auth hinzufügen
    auth: "username:password"
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
  "city": "Niesky",
  "email": "lotharhalke@web.de",
  "phone": "0172 9317474",
  "street": "Bautzenerstrasse 9",
  "comment": "",
  "zipcode": "02906",
  "last_name": "Halke",
  "lead_type": "phone",
  "questions": {
    "Dachfläche": "60",
    "Dachgefälle": "15",
    "Dachmaterial": "Dachpappe / Bitumen",
    "Finanzierung": "Nicht sicher",
    "Dachausrichtung": "West",
    "Wallbox gewünscht": "Nein",
    "Wie alt ist Ihr Dach?": "Nach 1990",
    "Stromspeicher gewünscht": "Nein",
    "Sind Sie Eigentümer der Immobilie?": "Ja",
    "Wann soll das Projekt gestartet werden?": "6",
    "Welche Dachform haben Sie auf Ihrem Haus?": "Flachdach",
    "Wie hoch schätzen Sie ihren Stromverbrauch?": "2000",
    "Wo möchten Sie die Solaranlage installieren?": "Einfamilienhaus"
  },
  "created_at": 1751005815,
  "first_name": "Lothar"
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
curl -X POST http://localhost:8004/webhooks/leads/ \
  -H "Content-Type: application/json" \
  -d '{
  "city": "Niesky",
  "email": "lotharhalke@web.de",
  "phone": "0172 9317474",
  "street": "Bautzenerstrasse 9",
  "comment": "",
  "zipcode": "02906",
  "last_name": "Halke",
  "lead_type": "phone",
  "questions": {
    "Dachfläche": "60",
    "Dachgefälle": "15",
    "Dachmaterial": "Dachpappe / Bitumen",
    "Finanzierung": "Nicht sicher",
    "Dachausrichtung": "West",
    "Wallbox gewünscht": "Nein",
    "Wie alt ist Ihr Dach?": "Nach 1990",
    "Stromspeicher gewünscht": "Nein",
    "Sind Sie Eigentümer der Immobilie?": "Ja",
    "Wann soll das Projekt gestartet werden?": "6",
    "Welche Dachform haben Sie auf Ihrem Haus?": "Flachdach",
    "Wie hoch schätzen Sie ihren Stromverbrauch?": "2000",
    "Wo möchten Sie die Solaranlage installieren?": "Einfamilienhaus"
  },
  "created_at": 1751005815,
  "first_name": "Lothar"
}'
```

**Windows PowerShell (sicherer Aufruf):**

```powershell
$payload = @'
{
  "city": "Niesky",
  "email": "lotharhalke@web.de",
  "phone": "0172 9317474",
  "street": "Bautzenerstrasse 9",
  "comment": "",
  "zipcode": "02906",
  "last_name": "Halke",
  "lead_type": "phone",
  "questions": {
    "Dachfläche": "60",
    "Dachgefälle": "15",
    "Dachmaterial": "Dachpappe / Bitumen",
    "Finanzierung": "Nicht sicher",
    "Dachausrichtung": "West",
    "Wallbox gewünscht": "Nein",
    "Wie alt ist Ihr Dach?": "Nach 1990",
    "Stromspeicher gewünscht": "Nein",
    "Sind Sie Eigentümer der Immobilie?": "Ja",
    "Wann soll das Projekt gestartet werden?": "6",
    "Welche Dachform haben Sie auf Ihrem Haus?": "Flachdach",
    "Wie hoch schätzen Sie ihren Stromverbrauch?": "2000",
    "Wo möchten Sie die Solaranlage installieren?": "Einfamilienhaus"
  },
  "created_at": 1751005815,
  "first_name": "Lothar"
}
'@
Invoke-RestMethod -Uri "http://localhost:8004/webhooks/leads/" -Method Post -ContentType "application/json" -Body $payload
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

| Regel            | Muster/Wert                                          | Ablehnungs‑Code         |
| --------------- | ---------------------------------------------------- | ----------------------- |
| Postleitzahl     | `^53\d{3}$` (z. B. 53000–53999)                      | ZIPCODE_INVALID         |
| Eigenheimbesitz  | `questions["Sind Sie Eigentümer der Immobilie?"]` muss `"Ja"` sein | NOT_HOMEOWNER |
| Pflichtfelder    | `email`, `phone`, `zipcode`, `street`, `city`, `first_name`, `last_name`, `questions["Sind Sie Eigentümer der Immobilie?"]` | MISSING_REQUIRED_FIELD |

### Gültige PLZ‑Beispiele

- ✅ 53000, 53859, 53999
- ❌ 12345, 52999, 54000, 5385, 538599

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
| `CUSTOMER_TOKEN`        | Bearer‑Token                    | `Bearer FakeCustomerToken` |
| `CUSTOMER_PRODUCT_NAME` | Produktname                     | `Solaranlage`            |
| `ATTRIBUTE_MAPPING_PATH`| Pfad zur Mapping‑Datei          | `customer_attribute_mapping.json` |
| `WEBHOOK_SHARED_SECRET` | Optionales Shared‑Secret        | `None`                   |
| `ZIPCODE_PATTERN`       | Regex für gültige PLZ           | `^53\d{3}$`              |
| `ZIPCODE_PATTERN_ERROR` | Fehlercode für ungültige PLZ    | `ZIPCODE_INVALID`        |
| `NOT_HOMEOWNER`         | Fehlercode für Nicht-Eigentümer | `NOT_HOMEOWNER`          |
| `MISSING_REQUIRED_FIELD`| Fehlercode für fehlende Felder  | `MISSING_REQUIRED_FIELD` |

### Feld‑Mapping

Die Datei `config/customer_attribute_mapping.json` definiert das Mapping:
phone": "phone",
  "customer_zip": "zipcode",
  "customer_street": "street",
  "customer_city": "city",
  "customer_first_name": "first_name",
  "customer_last_name": "last_name",
  "is_homeowner": "questions[Sind Sie Eigentümer der Immobilie?]"
}
```

- **Key**: Ziel‑Feld in der Kunden‑API
- **Value**: Pfad im Quell‑Payload (Dot‑Notation oder Bracket‑Notation für Sonderzeichen)

**Bracket‑Notation**: Für Dictionary‑Keys mit Sonderzeichen (z. B. Fragezeichen, Leerzeichen) wird Bracket‑Notation verwendet: `questions[Sind Sie Eigentümer der Immobilie?]`
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
cd python_lead
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
cd python_lead
set LIVE_E2E=1
pytest leads\tests\test_e2e_live_async.py
```

Optional, falls Ports geändert wurden:

```bash
set LIVE_E2E_API_BASE_URL=http://localhost:8004
set LIVE_E2E_MOCK_BASE_URL=http://localhost:18080
```

### E2E‑Test mit ngrok Trigger

Dieser Test simuliert den kompletten externen Flow über ngrok:
`Trigger → Lead Generator → ngrok → Django → Celery → Customer API`

Dieser Test prüft den Endpoint-Flow und manipuliert **keine** Datenbank-Daten.
Alle Leads bleiben für Audit/Historie erhalten.

**Voraussetzungen:**

1. Django App läuft (Docker oder lokal auf Port 8004)
2. ngrok Tunnel ist aktiv
3. NGROK_URL Umgebungsvariable ist gesetzt

**Test ausführen:**

```bash
# 1. ngrok starten und URL kopieren
start_ngrok.bat  # oder ./start_ngrok.sh

# 2. ngrok URL setzen
set NGROK_URL=https://abcd1234.ngrok.io

# 3. Test ausführen
pytest leads\tests\test_e2e_trigger_flow.py -v
```

**Was wird getestet:**

- ✅ Trigger-Endpunkt akzeptiert Webhook-URL
- ✅ Lead wird über ngrok an Django gesendet
- ✅ Lead wird in Datenbank gespeichert
- ✅ Celery verarbeitet Lead asynchron
- ✅ Delivery Attempts werden erstellt
- ✅ Validierungsregeln werden angewendet

### Test‑Kategorien

| Kategorie        | Beschreibung                       | Anzahl |
| --------------- | ---------------------------------- | ------ |
| Unit‑Tests       | Beispiel‑ und Edge‑Cases           | 68     |
| Property‑Tests   | Allgemeine Eigenschaften           | 2      |
| E2E‑Tests        | Live Async + Trigger Flow          | 3      |
| **Gesamt**       |                                    | **73** |

### Abdeckung

- Validierung: PLZ, Eigenheimbesitz, Pflichtfelder
- Normalisierung: E‑Mail, Leerzeichen, Boolean‑Konvertierung
- Mapping: Verschachtelte Felder, fehlende Felder
- Kunden‑Client: Erfolg/Fehler, Netzwerkfehler
- Celery‑Tasks: End‑to‑End, Validierungsfehler, Retries
- Webhook: Submission, Header, Auth
- **E2E Trigger Flow: External trigger → ngrok → Django → Celery → Delivery**

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

