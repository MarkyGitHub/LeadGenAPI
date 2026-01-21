"""
Unit tests for customer API client.
"""
import pytest
from unittest.mock import patch, Mock
import httpx

from leads.services.customer_client import send_to_customer


class TestSendToCustomer:
    """Tests for send_to_customer function."""
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_successful_2xx_response(self, mock_settings, mock_post):
        """Test successful 2xx response from Customer API."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.text = '{"success": true, "lead_id": "12345"}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 200
        assert response.text == '{"success": true, "lead_id": "12345"}'
        
        # Verify correct headers were sent
        mock_post.assert_called_once()
        call_kwargs = mock_post.call_args.kwargs
        assert call_kwargs['headers']['Authorization'] == 'Bearer test-token'
        assert call_kwargs['headers']['Content-Type'] == 'application/json'
        assert call_kwargs['json'] == payload
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_201_created_response(self, mock_settings, mock_post):
        """Test 201 Created response from Customer API."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 201
        mock_response.text = '{"created": true}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 201
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_4xx_client_error_response(self, mock_settings, mock_post):
        """Test 4xx client error response (should not retry)."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 400
        mock_response.text = '{"error": "Invalid payload"}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 400
        assert 'Invalid payload' in response.text
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_401_unauthorized_response(self, mock_settings, mock_post):
        """Test 401 Unauthorized response."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'invalid-token'
        
        mock_response = Mock()
        mock_response.status_code = 401
        mock_response.text = '{"error": "Unauthorized"}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 401
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_404_not_found_response(self, mock_settings, mock_post):
        """Test 404 Not Found response."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/wrong-endpoint'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 404
        mock_response.text = '{"error": "Not found"}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 404
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_5xx_server_error_response(self, mock_settings, mock_post):
        """Test 5xx server error response (should trigger retry)."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 500
        mock_response.text = '{"error": "Internal server error"}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 500
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_503_service_unavailable_response(self, mock_settings, mock_post):
        """Test 503 Service Unavailable response (should trigger retry)."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 503
        mock_response.text = '{"error": "Service unavailable"}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        response = send_to_customer(payload)
        
        assert response.status_code == 503
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_timeout_exception(self, mock_settings, mock_post):
        """Test network timeout scenario (should trigger retry)."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_post.side_effect = httpx.TimeoutException('Request timeout')
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        with pytest.raises(httpx.TimeoutException):
            send_to_customer(payload)
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_connection_error(self, mock_settings, mock_post):
        """Test connection error (should trigger retry)."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_post.side_effect = httpx.ConnectError('Connection refused')
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        with pytest.raises(httpx.ConnectError):
            send_to_customer(payload)
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_bearer_token_in_header(self, mock_settings, mock_post):
        """Test that Bearer token is correctly included in Authorization header."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'my-secret-token-123'
        
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.text = '{"success": true}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        send_to_customer(payload)
        
        # Verify Bearer token format
        call_kwargs = mock_post.call_args.kwargs
        assert call_kwargs['headers']['Authorization'] == 'Bearer my-secret-token-123'
    
    @patch('leads.services.customer_client.httpx.post')
    @patch('leads.services.customer_client.settings')
    def test_content_type_header(self, mock_settings, mock_post):
        """Test that Content-Type header is set to application/json."""
        mock_settings.CUSTOMER_API_URL = 'https://api.customer.com/leads'
        mock_settings.CUSTOMER_TOKEN = 'test-token'
        
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.text = '{"success": true}'
        mock_post.return_value = mock_response
        
        payload = {'phone': '+49123456789', 'product': {'name': 'Solar'}}
        
        send_to_customer(payload)
        
        # Verify Content-Type header
        call_kwargs = mock_post.call_args.kwargs
        assert call_kwargs['headers']['Content-Type'] == 'application/json'
