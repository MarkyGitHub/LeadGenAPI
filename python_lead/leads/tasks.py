"""
Celery tasks for async lead processing.
"""
import logging
from celery import shared_task
import httpx
from django.conf import settings

from leads.models import InboundLead, DeliveryAttempt
from leads.services.validation import validate_lead
from leads.services.normalization import normalize
from leads.services.mapping import map_to_customer, MissingRequiredFieldError
from leads.services.customer_client import send_to_customer

logger = logging.getLogger(__name__)


@shared_task(
    bind=True,
    autoretry_for=(httpx.TimeoutException, httpx.ConnectError),
    retry_backoff=30,  # Exponential backoff starting at 30s
    retry_backoff_max=480,  # Max backoff of 480s (8 minutes)
    max_retries=5,
    retry_jitter=False  # Disable jitter for predictable backoff
)
def process_lead(self, lead_id: int):
    """
    Process a lead through the validation, transformation, and delivery pipeline.
    
    Workflow:
    1. Load lead from database
    2. Validate against business rules
    3. If invalid: mark REJECTED, stop
    4. Normalize data
    5. Map to customer format
    6. Send to customer API
    7. Record delivery attempt
    8. Update lead status (DELIVERED or FAILED)
    
    Args:
        lead_id: ID of the InboundLead to process
    """
    try:
        # 1. Load lead from database
        lead = InboundLead.objects.get(id=lead_id)
        logger.info(f"Processing lead {lead_id}, current status: {lead.status}")
        
        # 2. Validate
        is_valid, rejection_reason = validate_lead(lead.raw_payload)
        
        if not is_valid:
            # Mark as REJECTED and stop processing
            lead.status = InboundLead.Status.REJECTED
            lead.rejection_reason = rejection_reason
            lead.save()
            logger.info(f"Lead {lead_id} REJECTED: {rejection_reason}")
            return
        
        logger.info(f"Lead {lead_id} passed validation")
        
        # 3. Normalize
        normalized_payload = normalize(lead.raw_payload)
        lead.normalized_payload = normalized_payload
        logger.debug(f"Lead {lead_id} normalized")

        # Guard: CUSTOMER_PRODUCT_NAME must be configured
        if not settings.CUSTOMER_PRODUCT_NAME:
            lead.status = InboundLead.Status.FAILED
            lead.rejection_reason = "Missing required field: product.name (CUSTOMER_PRODUCT_NAME not configured)"
            lead.save()
            logger.error(f"Lead {lead_id} FAILED: CUSTOMER_PRODUCT_NAME not configured")
            return
        
        # 4. Map to customer format
        try:
            customer_payload, omitted_attributes = map_to_customer(normalized_payload)
            lead.customer_payload = customer_payload
            
            if omitted_attributes:
                logger.warning(
                    f"Lead {lead_id}: Omitted {len(omitted_attributes)} invalid attributes: "
                    f"{omitted_attributes}"
                )
            
            # Mark as READY (passed validation and transformation)
            lead.status = InboundLead.Status.READY
            lead.save()
            logger.info(f"Lead {lead_id} marked as READY for delivery")
            
        except MissingRequiredFieldError as e:
            # Missing required Core_Customer_Fields (phone, product.name)
            lead.status = InboundLead.Status.FAILED
            lead.rejection_reason = f"Missing required field: {str(e)}"
            lead.save()
            logger.error(f"Lead {lead_id} FAILED: {str(e)}")
            return
        
        # 5. Send to customer API
        attempt_no = lead.delivery_attempts.count() + 1
        logger.info(f"Lead {lead_id}: Delivery attempt #{attempt_no}")
        
        try:
            response = send_to_customer(customer_payload)
            
            # Record delivery attempt
            delivery_attempt = DeliveryAttempt.objects.create(
                lead=lead,
                attempt_no=attempt_no,
                response_status=response.status_code,
                response_body=response.text,
                success=(200 <= response.status_code < 300)
            )
            
            # Handle response based on status code
            if 200 <= response.status_code < 300:
                # 2xx: Success
                lead.status = InboundLead.Status.DELIVERED
                lead.save()
                logger.info(f"Lead {lead_id} DELIVERED successfully")
                
            elif 400 <= response.status_code < 500:
                # 4xx: Client error - mark as PERMANENTLY_FAILED (no retry)
                lead.status = InboundLead.Status.PERMANENTLY_FAILED
                lead.rejection_reason = f"Client error: {response.status_code}"
                lead.save()
                logger.error(
                    f"Lead {lead_id} PERMANENTLY_FAILED: "
                    f"Client error {response.status_code}, no retry"
                )
                
            elif 500 <= response.status_code < 600:
                # 5xx: Server error - mark as FAILED and retry
                lead.status = InboundLead.Status.FAILED
                lead.save()
                logger.warning(
                    f"Lead {lead_id} FAILED: Server error {response.status_code}, "
                    f"will retry (attempt {attempt_no}/{self.max_retries + 1})"
                )
                # Raise exception to trigger Celery retry
                raise Exception(f"Server error: {response.status_code}")
            
        except (httpx.TimeoutException, httpx.ConnectError) as e:
            # Network errors - record attempt and retry
            delivery_attempt = DeliveryAttempt.objects.create(
                lead=lead,
                attempt_no=attempt_no,
                error_message=str(e),
                success=False
            )
            
            lead.status = InboundLead.Status.FAILED
            lead.save()
            logger.warning(
                f"Lead {lead_id} FAILED: Network error, "
                f"will retry (attempt {attempt_no}/{self.max_retries + 1})"
            )
            # Re-raise to trigger Celery retry
            raise
        
    except InboundLead.DoesNotExist:
        logger.error(f"Lead {lead_id} not found in database")
        raise
    
    except Exception as e:
        # Handle max retry exhaustion
        if self.request.retries >= self.max_retries:
            try:
                lead = InboundLead.objects.get(id=lead_id)
                lead.status = InboundLead.Status.PERMANENTLY_FAILED
                lead.rejection_reason = f"Max retries exhausted: {str(e)}"
                lead.save()
                logger.error(
                    f"Lead {lead_id} PERMANENTLY_FAILED: "
                    f"Max retries ({self.max_retries}) exhausted"
                )
            except InboundLead.DoesNotExist:
                logger.error(f"Lead {lead_id} not found when marking as PERMANENTLY_FAILED")
        
        # Re-raise to let Celery handle retry logic
        raise
