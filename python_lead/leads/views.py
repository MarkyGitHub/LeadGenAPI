"""
API views for Lead Gateway Service.
"""
import logging
import uuid
from rest_framework.exceptions import ParseError
from rest_framework.views import APIView
from rest_framework.response import Response
from rest_framework import status
from django.views.decorators.csrf import csrf_exempt
from django.utils.decorators import method_decorator

from leads.models import InboundLead
from leads.tasks import process_lead

logger = logging.getLogger(__name__)


@method_decorator(csrf_exempt, name='dispatch')
class LeadWebhookView(APIView):
    """
    Webhook endpoint for receiving leads from external sources.
    
    POST /webhooks/leads/
    - Accepts JSON payload
    - Stores raw payload and headers
    - Enqueues async processing task
    - Returns 200 OK with lead_id and correlation_id
    """
    
    def post(self, request):
        """
        Handle incoming lead submission.
        
        Returns:
            200 OK: Lead accepted and queued for processing
            400 Bad Request: Malformed JSON or invalid payload
            500 Internal Server Error: Unexpected error
        """
        # Generate correlation ID for request tracing
        correlation_id = str(uuid.uuid4())
        
        try:
            # Get raw payload
            payload = request.data
            
            if not payload:
                logger.warning(f"Empty payload received, correlation_id={correlation_id}")
                return Response(
                    {
                        'error': 'Empty payload',
                        'correlation_id': correlation_id
                    },
                    status=status.HTTP_400_BAD_REQUEST
                )
            
            # Extract headers for audit trail
            source_headers = {
                'content-type': request.META.get('CONTENT_TYPE', ''),
                'user-agent': request.META.get('HTTP_USER_AGENT', ''),
                'x-forwarded-for': request.META.get('HTTP_X_FORWARDED_FOR', ''),
                'remote-addr': request.META.get('REMOTE_ADDR', ''),
            }
            
            # Store lead in database
            lead = InboundLead.objects.create(
                raw_payload=payload,
                source_headers=source_headers,
                status=InboundLead.Status.RECEIVED
            )
            
            logger.info(
                f"Lead {lead.id} received and stored, "
                f"correlation_id={correlation_id}"
            )
            
            # Enqueue async processing task
            process_lead.delay(lead.id)
            
            logger.info(
                f"Lead {lead.id} enqueued for processing, "
                f"correlation_id={correlation_id}"
            )
            
            # Return success response
            return Response(
                {
                    'status': 'accepted',
                    'lead_id': lead.id,
                    'correlation_id': correlation_id
                },
                status=status.HTTP_200_OK
            )
            
        except ParseError as e:
            logger.warning(
                f"Malformed JSON payload: {e}, "
                f"correlation_id={correlation_id}"
            )
            return Response(
                {
                    'error': 'Malformed JSON',
                    'correlation_id': correlation_id
                },
                status=status.HTTP_400_BAD_REQUEST
            )
        except Exception as e:
            logger.error(
                f"Error processing webhook request: {e}, "
                f"correlation_id={correlation_id}",
                exc_info=True
            )
            return Response(
                {
                    'error': 'Internal server error',
                    'correlation_id': correlation_id
                },
                status=status.HTTP_500_INTERNAL_SERVER_ERROR
            )
