"""
End-to-end tests for webhook REST endpoint and customer API delivery.
"""
from unittest.mock import Mock, patch

import pytest
from django.conf import settings
from rest_framework.test import APIClient

from leads.models import InboundLead
from leads.tasks import process_lead


@pytest.mark.django_db
class TestWebhookE2E:
    """Live-ish e2e tests for webhook → task → customer API flow."""

    def _run_task_sync(self, lead_id: int) -> None:
        """Run the Celery task synchronously for tests."""
        process_lead(lead_id)

    @patch("leads.services.customer_client.httpx.post")
    @patch("leads.views.process_lead.delay")
    def test_accepts_lead_and_delivers_to_customer_api(
        self,
        mock_delay,
        mock_httpx_post,
        valid_lead_payload,
    ):
        """Valid lead is accepted and delivered to CUSTOMER_API_URL."""
        mock_delay.side_effect = self._run_task_sync

        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.text = "{\"success\": true}"
        mock_httpx_post.return_value = mock_response

        client = APIClient()
        response = client.post("/webhooks/leads/", valid_lead_payload, format="json")

        assert response.status_code == 200
        assert response.data["status"] == "accepted"

        lead = InboundLead.objects.get(id=response.data["lead_id"])
        assert lead.status == InboundLead.Status.DELIVERED
        assert lead.delivery_attempts.count() == 1

        mock_httpx_post.assert_called_once()
        call_args, call_kwargs = mock_httpx_post.call_args
        url = call_kwargs.get("url") or (call_args[0] if call_args else None)
        assert url == settings.CUSTOMER_API_URL
        assert call_kwargs["headers"]["Authorization"] == f"Bearer {settings.CUSTOMER_TOKEN}"
        assert call_kwargs["headers"]["Content-Type"] == "application/json"
        assert call_kwargs["json"]["phone"] == valid_lead_payload["phone"]
        assert call_kwargs["json"]["product"]["name"] == settings.CUSTOMER_PRODUCT_NAME

    @patch("leads.services.customer_client.httpx.post")
    @patch("leads.views.process_lead.delay")
    def test_rejects_lead_and_does_not_call_customer_api(
        self,
        mock_delay,
        mock_httpx_post,
        invalid_zipcode_payload,
    ):
        """Invalid lead is rejected and not sent to CUSTOMER_API_URL."""
        mock_delay.side_effect = self._run_task_sync

        client = APIClient()
        response = client.post("/webhooks/leads/", invalid_zipcode_payload, format="json")

        assert response.status_code == 200
        assert response.data["status"] == "accepted"

        lead = InboundLead.objects.get(id=response.data["lead_id"])
        assert lead.status == InboundLead.Status.REJECTED
        assert lead.delivery_attempts.count() == 0

        mock_httpx_post.assert_not_called()