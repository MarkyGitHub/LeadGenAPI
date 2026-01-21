"""
Live async e2e tests against running services.

Requires docker-compose stack running with docker-compose.e2e.yml.
Set LIVE_E2E=1 to enable.
"""
import os
import time
import logging

import httpx
import psycopg2
import pytest


LIVE_E2E = os.getenv("LIVE_E2E", "").lower() in {"1", "true", "yes"}

logger = logging.getLogger(__name__)


pytestmark = pytest.mark.skipif(
    not LIVE_E2E,
    reason="LIVE_E2E not enabled (set LIVE_E2E=1)",
)


def _ensure_reachable(url: str, name: str) -> None:
    try:
        logger.info("Checking reachability for %s at %s", name, url)
        httpx.get(url, timeout=3.0)
    except httpx.HTTPError:
        pytest.skip(f"{name} not reachable at {url}. Start docker-compose with e2e override.")


def _get_db_conn():
    db_name = os.getenv("LIVE_E2E_DB_NAME", os.getenv("DB_NAME", "lead_gateway"))
    db_user = os.getenv("LIVE_E2E_DB_USER", os.getenv("DB_USER", "postgres"))
    db_password = os.getenv("LIVE_E2E_DB_PASSWORD", os.getenv("DB_PASSWORD", "postgres"))
    db_host = os.getenv("LIVE_E2E_DB_HOST", os.getenv("DB_HOST", "localhost"))
    db_port = int(os.getenv("LIVE_E2E_DB_PORT", os.getenv("DB_PORT", "5432")))

    try:
        return psycopg2.connect(
            dbname=db_name,
            user=db_user,
            password=db_password,
            host=db_host,
            port=db_port,
        )
    except psycopg2.OperationalError:
        pytest.skip("PostgreSQL not reachable. Ensure docker-compose DB is up and ports are exposed.")


def _wait_for_status(lead_id: int, expected_status: str, timeout_seconds: int = 20):
    deadline = time.time() + timeout_seconds
    while time.time() < deadline:
        with _get_db_conn() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT status FROM leads_inboundlead WHERE id = %s",
                    (lead_id,),
                )
                row = cur.fetchone()
                if row and row[0] == expected_status:
                    return True
        time.sleep(0.5)
    return False


def _get_delivery_attempt(lead_id: int):
    with _get_db_conn() as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                SELECT response_status, success
                FROM leads_deliveryattempt
                WHERE lead_id = %s
                ORDER BY attempt_no DESC
                LIMIT 1
                """,
                (lead_id,),
            )
            return cur.fetchone()


def _format_response(response: httpx.Response) -> str:
    try:
        body = response.json()
    except ValueError:
        body = response.text
    return (
        f"status={response.status_code} "
        f"headers={dict(response.headers)} "
        f"body={body}"
    )


def test_live_async_accepts_and_delivers(valid_lead_payload):
    api_base_url = "http://localhost:8004"

    _ensure_reachable(f"{api_base_url}/admin/", "Webhook API")

    logger.info("Valid lead payload=%s", valid_lead_payload)
    logger.info("Posting valid lead payload to %s/webhooks/leads/", api_base_url)
    response = httpx.post(
        f"{api_base_url}/webhooks/leads/",
        json=valid_lead_payload,
        timeout=10.0,
    )
    logger.info("Received response %s", _format_response(response))
    assert response.status_code == 200

    lead_id = response.json().get("lead_id")
    logger.info("Received lead_id=%s", lead_id)
    assert lead_id is not None

    logger.info("Waiting for lead_id=%s to reach DELIVERED", lead_id)
    assert _wait_for_status(lead_id, "DELIVERED")

    attempt = _get_delivery_attempt(lead_id)
    logger.info("Delivery attempt for lead_id=%s: %s", lead_id, attempt)
    assert attempt is not None
    response_status, success = attempt
    assert success is True
    assert 200 <= response_status < 300


def test_live_async_rejects_and_does_not_deliver(invalid_zipcode_payload): 
    api_base_url = "http://localhost:8004"

    _ensure_reachable(f"{api_base_url}/admin/", "Webhook API")

    logger.info("Posting invalid zipcode payload to %s/webhooks/leads/", api_base_url)
    response = httpx.post(
        f"{api_base_url}/webhooks/leads/",
        json=invalid_zipcode_payload,
        timeout=10.0,
    )
    logger.info("Received response %s", _format_response(response))
    assert response.status_code == 200

    lead_id = response.json().get("lead_id")
    logger.info("Received lead_id=%s", lead_id)
    assert lead_id is not None

    logger.info("Waiting for lead_id=%s to reach REJECTED", lead_id)
    assert _wait_for_status(lead_id, "REJECTED")

    attempt = _get_delivery_attempt(lead_id)
    logger.info("Delivery attempt for lead_id=%s: %s", lead_id, attempt)
    assert attempt is None