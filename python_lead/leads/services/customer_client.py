"""
Customer API client for sending leads to external customer system.
"""
import logging
import json
import httpx
from django.conf import settings

logger = logging.getLogger(__name__)


def _format_response(response: httpx.Response) -> str:
    """Return a readable response string (pretty JSON if possible)."""
    try:
        data = response.json()
        return json.dumps(data, indent=2, ensure_ascii=False)
    except Exception:
        return response.text


def send_to_customer(payload: dict) -> httpx.Response:
    """
    Sends lead to customer API with Bearer token authentication.
    
    Args:
        payload: Customer-formatted lead data
    
    Returns:
        HTTP response from customer API
    
    Raises:
        httpx.HTTPError: On network/timeout errors
    """
    url = settings.CUSTOMER_API_URL
    token = settings.CUSTOMER_TOKEN
    logger.debug("Customer API token=%s", token)
    
    headers = {
        'Authorization': f'{token}',
        'Content-Type': 'application/json',
    }

    if settings.CUSTOMER_REFERER:
        headers['Referer'] = settings.CUSTOMER_REFERER
    
    logger.info(f"Sending lead to customer API: {url}")
    logger.debug(f"Payload: {payload}")
    
    try:
        response = httpx.post(
            url,
            json=payload,
            headers=headers,
            timeout=30.0  # 30 second timeout
        )
        
        logger.info(f"Customer API response: {response.status_code}")
        logger.info("Customer API response body:\n%s", _format_response(response))
        
        return response
        
    except httpx.TimeoutException as e:
        logger.error(f"Timeout sending to customer API: {e}")
        raise
    except httpx.ConnectError as e:
        logger.error(f"Connection error sending to customer API: {e}")
        raise
    except httpx.HTTPError as e:
        logger.error(f"HTTP error sending to customer API: {e}")
        raise
