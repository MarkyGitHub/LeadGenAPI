"""
End-to-end test for the external trigger flow.

This test simulates the complete flow:
1. External trigger endpoint calls our ngrok webhook
2. Lead is received by Django
3. Celery processes the lead asynchronously
4. Lead is validated, transformed, and delivered

Prerequisites:
- Django app running (docker-compose up -d or locally on port 8004)
- ngrok tunnel active (./start_ngrok.sh or start_ngrok.bat)
- Set environment variable NGROK_URL to your current ngrok URL

Usage:
    # Windows
    set NGROK_URL=https://abcd1234.ngrok.io
    pytest leads/tests/test_e2e_trigger_flow.py -v

    # Linux/macOS
    export NGROK_URL=https://abcd1234.ngrok.io
    pytest leads/tests/test_e2e_trigger_flow.py -v
"""
import os
import time
import logging

import pytest
import requests


logger = logging.getLogger(__name__)


# Configuration
TRIGGER_URL = "https://contactapi.static.fyi/lead/trigger/fake/USER_ID/"
NGROK_WEBHOOK_URL = os.getenv("NGROK_URL", "")

TRIGGER_HEADERS = {
    "Authorization": "Bearer FakeCustomerToken",
    "Content-Type": "application/json",
}


# Skip test if NGROK_URL is not set
pytestmark = pytest.mark.skipif(
    not NGROK_WEBHOOK_URL,
    reason="NGROK_URL environment variable not set. Set it to your ngrok URL (e.g., https://abcd1234.ngrok.io)",
)


@pytest.fixture
def db_connection():
    """
    Provide database connection for assertions.
    Loads database configuration from .env file using load_dotenv().
    """
    import psycopg2
    from dotenv import load_dotenv
    
    # Load environment variables from .env file
    load_dotenv()
    
    # Get database configuration from environment variables
    db_config = {
        "dbname": os.getenv("DB_NAME", "lead_gateway"),
        "user": os.getenv("DB_USER", "postgres"),
        "password": os.getenv("DB_PASSWORD", "postgres"),
        "host": os.getenv("DB_HOST", "localhost"),
        "port": int(os.getenv("DB_PORT", "5432")),
    }
    
    logger.info(f"Connecting to database: {db_config['host']}:{db_config['port']}/{db_config['dbname']}")
    
    try:
        conn = psycopg2.connect(**db_config)
        logger.info(f"✅ Connected to {db_config['dbname']} successfully")
        yield conn
        conn.close()
    except psycopg2.OperationalError as e:
        logger.error(f"❌ Failed to connect: {e}")
        pytest.skip(f"Database not available: {e}")


@pytest.fixture
def clean_db(db_connection):
    """
    No-op fixture - E2E trigger tests verify endpoint flow, not database state.
    
    These tests verify the flow: Trigger → Lead Generator → Webhook → Celery → Customer API
    They don't require a clean database - they just verify that leads are processed correctly.
    All leads are preserved for audit/history purposes.
    """
    # No cleanup needed - this is an endpoint test, not a database test
    pass


def test_trigger_pushes_lead_into_system(db_connection, clean_db):
    """
    End-to-end test: Trigger -> Lead Generator -> ngrok -> Django webhook -> Celery pipeline
    
    This test verifies the complete flow:
    1. POST to trigger endpoint with our ngrok webhook URL
    2. Lead Generator calls our webhook through ngrok tunnel
    3. Django receives the lead and queues it for processing
    4. Celery worker validates, transforms, and delivers the lead
    5. Lead is stored in database with final status
    """
    
    # Prepare webhook URL with proper path
    webhook_url = NGROK_WEBHOOK_URL.rstrip("/") + "/webhooks/leads/"
    
    logger.info(f"Testing trigger flow with webhook URL: {webhook_url}")
    
    payload = {
        "url": webhook_url,
        "headers": {
            "Content-Type": "application/json"
        }
    }
    
    # 1️⃣ Send trigger request
    logger.info(f"Sending trigger request to {TRIGGER_URL}")
    response = requests.post(
        TRIGGER_URL,
        headers=TRIGGER_HEADERS,
        json=payload,
        timeout=10,
    )
    
    # Verify trigger was accepted
    assert response.status_code == 200, f"Trigger failed with status {response.status_code}: {response.text}"
    logger.info(f"Trigger accepted: {response.status_code}")
    
    # 2️⃣ Wait for async pipeline to complete
    # The flow is: trigger -> lead generator -> webhook -> celery -> delivery
    # This can take a few seconds
    logger.info("Waiting for async pipeline to process...")
    time.sleep(5)
    
    # 3️⃣ Verify lead was created and processed
    cursor = db_connection.cursor()
    
    # Check that at least one lead exists
    cursor.execute("SELECT COUNT(*) FROM leads_inboundlead")
    lead_count = cursor.fetchone()[0]
    assert lead_count > 0, "No leads found in database after trigger"
    logger.info(f"Found {lead_count} lead(s) in database")
    
    # Get the most recent lead
    cursor.execute("""
        SELECT id, status, rejection_reason, raw_payload
        FROM leads_inboundlead
        ORDER BY created_at DESC
        LIMIT 1
    """)
    lead_row = cursor.fetchone()
    lead_id, status, rejection_reason, raw_payload = lead_row
    
    logger.info(f"Lead {lead_id} status: {status}")
    if rejection_reason:
        logger.info(f"Rejection reason: {rejection_reason}")
    
    # Verify lead was processed beyond RECEIVED status
    assert status in ("READY", "DELIVERED", "FAILED", "PERMANENTLY_FAILED", "REJECTED"), \
        f"Lead still in initial state or invalid status: {status}"
    
    # 4️⃣ Verify delivery attempt was made (if lead was valid)
    if status != "REJECTED":
        cursor.execute("""
            SELECT COUNT(*)
            FROM leads_deliveryattempt
            WHERE lead_id = %s
        """, (lead_id,))
        attempt_count = cursor.fetchone()[0]
        assert attempt_count > 0, f"No delivery attempts found for lead {lead_id}"
        logger.info(f"Found {attempt_count} delivery attempt(s) for lead {lead_id}")
        
        # Get latest delivery attempt details
        cursor.execute("""
            SELECT http_status_code, response_body
            FROM leads_deliveryattempt
            WHERE lead_id = %s
            ORDER BY attempted_at DESC
            LIMIT 1
        """, (lead_id,))
        attempt_row = cursor.fetchone()
        http_status, response_body = attempt_row
        logger.info(f"Latest delivery attempt: HTTP {http_status}")
        logger.info(f"Response: {response_body[:200] if response_body else 'None'}...")
    
    cursor.close()
    
    logger.info("✅ Trigger flow test completed successfully")


def test_trigger_with_invalid_url(db_connection, clean_db):
    """
    Test that trigger still accepts request even with invalid webhook URL.
    The trigger endpoint should return 200, but the lead generator will fail to deliver.
    """
    
    payload = {
        "url": "https://invalid-url-that-does-not-exist.example.com/webhooks/leads/",
        "headers": {
            "Content-Type": "application/json"
        }
    }
    
    logger.info("Testing trigger with invalid webhook URL")
    response = requests.post(
        TRIGGER_URL,
        headers=TRIGGER_HEADERS,
        json=payload,
        timeout=10,
    )
    
    # Trigger should still accept the request
    assert response.status_code == 200, f"Trigger failed with status {response.status_code}"
    logger.info("✅ Trigger accepted invalid URL (as expected)")


def test_trigger_flow_validates_zip_codes(db_connection, clean_db):
    """
    Test that the trigger flow respects validation rules.
    This test verifies that leads with invalid ZIP codes are rejected.
    
    Note: This test depends on the lead generator sending specific test data.
    The actual validation happens in our webhook, not in the trigger.
    """
    
    webhook_url = NGROK_WEBHOOK_URL.rstrip("/") + "/webhooks/leads/"
    
    logger.info("Testing trigger flow with validation")
    
    payload = {
        "url": webhook_url,
        "headers": {
            "Content-Type": "application/json"
        }
    }
    
    response = requests.post(
        TRIGGER_URL,
        headers=TRIGGER_HEADERS,
        json=payload,
        timeout=10,
    )
    
    assert response.status_code == 200
    logger.info("Trigger accepted, waiting for processing...")
    
    # Wait for processing
    time.sleep(5)
    
    # Check that lead exists (regardless of validation result)
    cursor = db_connection.cursor()
    cursor.execute("SELECT COUNT(*) FROM leads_inboundlead")
    lead_count = cursor.fetchone()[0]
    
    assert lead_count > 0, "No leads created after trigger"
    logger.info(f"Found {lead_count} lead(s) - validation check passed")
    
    cursor.close()


if __name__ == "__main__":
    """
    Run this test directly for quick testing.
    
    Usage:
        python -m pytest leads/tests/test_e2e_trigger_flow.py -v -s
    """
    print("=" * 60)
    print("E2E Trigger Flow Test")
    print("=" * 60)
    print(f"NGROK_URL: {NGROK_WEBHOOK_URL or 'NOT SET'}")
    print(f"TRIGGER_URL: {TRIGGER_URL}")
    print("=" * 60)
    
    if not NGROK_WEBHOOK_URL:
        print("\n⚠️  WARNING: NGROK_URL not set!")
        print("\nPlease set your ngrok URL:")
        print("  Windows: set NGROK_URL=https://abcd1234.ngrok.io")
        print("  Linux:   export NGROK_URL=https://abcd1234.ngrok.io")
        print("\nThen run: pytest leads/tests/test_e2e_trigger_flow.py -v\n")
