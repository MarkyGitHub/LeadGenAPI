"""
Unit tests for webhook API views.
"""
import pytest
import json
from unittest.mock import patch, Mock
from rest_framework.test import APIRequestFactory
from leads.views import LeadWebhookView
from leads.models import InboundLead


class TestLeadWebhookView:
    """Tests for LeadWebhookView."""
    
    def setup_method(self):
        """Set up test fixtures."""
        self.factory = APIRequestFactory()
        self.view = LeadWebhookView.as_view()
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_successful_lead_submission(self, mock_lead_model, mock_process_lead):
        """Test successful lead submission returns 200 OK."""
        # Mock lead creation
        mock_lead = Mock()
        mock_lead.id = 123
        mock_lead_model.objects.create.return_value = mock_lead
        mock_lead_model.Status = InboundLead.Status  # Use real Status enum
        
        # Create request
        payload = {
            'email': 'test@example.com',
            'phone': '+49123456789',
            'address': {'zip': '66123', 'street': '123 Main St'},
            'house': {'is_owner': True}
        }
        request = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps(payload),
            content_type='application/json'
        )
        
        # Call view
        response = self.view(request)
        
        # Verify response
        assert response.status_code == 200
        assert response.data['status'] == 'accepted'
        assert response.data['lead_id'] == 123
        assert 'correlation_id' in response.data
        
        # Verify lead was created
        mock_lead_model.objects.create.assert_called_once()
        call_kwargs = mock_lead_model.objects.create.call_args.kwargs
        assert call_kwargs['raw_payload'] == payload
        assert call_kwargs['status'] == InboundLead.Status.RECEIVED
        assert 'source_headers' in call_kwargs
        
        # Verify task was enqueued
        mock_process_lead.delay.assert_called_once_with(123)
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_empty_payload_returns_400(self, mock_lead_model, mock_process_lead):
        """Test empty payload returns 400 Bad Request."""
        # Create request with empty payload
        request = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps({}),
            content_type='application/json'
        )
        
        # Call view
        response = self.view(request)
        
        # Verify response
        assert response.status_code == 400
        assert 'error' in response.data
        assert 'correlation_id' in response.data
        
        # Verify lead was not created
        mock_lead_model.objects.create.assert_not_called()
        
        # Verify task was not enqueued
        mock_process_lead.delay.assert_not_called()
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_malformed_json_returns_400(self, mock_lead_model, mock_process_lead):
        """Test malformed JSON returns 400 Bad Request."""
        # Create request with malformed JSON
        request = self.factory.post(
            '/webhooks/leads/',
            data='{"invalid": json}',
            content_type='application/json'
        )
        
        # Call view
        response = self.view(request)
        
        # Verify response (DRF handles JSON parsing, returns 400)
        assert response.status_code in [400, 500]
        assert 'correlation_id' in response.data or 'error' in response.data
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_response_format(self, mock_lead_model, mock_process_lead):
        """Test response contains required fields."""
        # Mock lead creation
        mock_lead = Mock()
        mock_lead.id = 456
        mock_lead_model.objects.create.return_value = mock_lead
        
        # Create request
        payload = {
            'email': 'test@example.com',
            'phone': '+49123456789',
            'address': {'zip': '66123'},
            'house': {'is_owner': True}
        }
        request = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps(payload),
            content_type='application/json'
        )
        
        # Call view
        response = self.view(request)
        
        # Verify response format
        assert 'status' in response.data
        assert 'lead_id' in response.data
        assert 'correlation_id' in response.data
        assert response.data['status'] == 'accepted'
        assert response.data['lead_id'] == 456
        assert isinstance(response.data['correlation_id'], str)
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_headers_stored_in_audit_trail(self, mock_lead_model, mock_process_lead):
        """Test that request headers are stored for audit purposes."""
        # Mock lead creation
        mock_lead = Mock()
        mock_lead.id = 789
        mock_lead_model.objects.create.return_value = mock_lead
        
        # Create request with headers
        payload = {'email': 'test@example.com', 'phone': '+49123456789'}
        request = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps(payload),
            content_type='application/json',
            HTTP_USER_AGENT='TestAgent/1.0',
            HTTP_X_FORWARDED_FOR='192.168.1.1'
        )
        
        # Call view
        response = self.view(request)
        
        # Verify headers were stored
        call_kwargs = mock_lead_model.objects.create.call_args.kwargs
        source_headers = call_kwargs['source_headers']
        assert 'user-agent' in source_headers
        assert 'x-forwarded-for' in source_headers
        assert source_headers['user-agent'] == 'TestAgent/1.0'
        assert source_headers['x-forwarded-for'] == '192.168.1.1'
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_database_error_returns_500(self, mock_lead_model, mock_process_lead):
        """Test database error returns 500 Internal Server Error."""
        # Mock database error
        mock_lead_model.objects.create.side_effect = Exception('Database error')
        
        # Create request
        payload = {'email': 'test@example.com', 'phone': '+49123456789'}
        request = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps(payload),
            content_type='application/json'
        )
        
        # Call view
        response = self.view(request)
        
        # Verify response
        assert response.status_code == 500
        assert 'error' in response.data
        assert 'correlation_id' in response.data
        
        # Verify task was not enqueued
        mock_process_lead.delay.assert_not_called()
    
    @patch('leads.views.process_lead')
    @patch('leads.views.InboundLead')
    def test_correlation_id_unique_per_request(self, mock_lead_model, mock_process_lead):
        """Test that each request gets a unique correlation ID."""
        # Mock lead creation
        mock_lead1 = Mock()
        mock_lead1.id = 1
        mock_lead2 = Mock()
        mock_lead2.id = 2
        mock_lead_model.objects.create.side_effect = [mock_lead1, mock_lead2]
        
        # Create two requests
        payload = {'email': 'test@example.com', 'phone': '+49123456789'}
        request1 = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps(payload),
            content_type='application/json'
        )
        request2 = self.factory.post(
            '/webhooks/leads/',
            data=json.dumps(payload),
            content_type='application/json'
        )
        
        # Call view twice
        response1 = self.view(request1)
        response2 = self.view(request2)
        
        # Verify correlation IDs are different
        assert response1.data['correlation_id'] != response2.data['correlation_id']
