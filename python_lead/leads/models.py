"""
Data models for Lead Gateway Service.
"""
from django.db import models


class InboundLead(models.Model):
    """
    Represents an inbound lead received via webhook.
    Stores the complete lifecycle from reception to delivery.
    """
    
    class Status(models.TextChoices):
        RECEIVED = 'RECEIVED', 'Received'
        REJECTED = 'REJECTED', 'Rejected'
        READY = 'READY', 'Ready'
        DELIVERED = 'DELIVERED', 'Delivered'
        FAILED = 'FAILED', 'Failed'
        PERMANENTLY_FAILED = 'PERMANENTLY_FAILED', 'Permanently Failed'
    
    received_at = models.DateTimeField(auto_now_add=True, db_index=True)
    raw_payload = models.JSONField()
    source_headers = models.JSONField(null=True, blank=True)
    status = models.CharField(
        max_length=20,
        choices=Status.choices,
        default=Status.RECEIVED,
        db_index=True
    )
    rejection_reason = models.CharField(max_length=100, null=True, blank=True)
    normalized_payload = models.JSONField(null=True, blank=True)
    customer_payload = models.JSONField(null=True, blank=True)
    payload_hash = models.CharField(max_length=64, null=True, blank=True, db_index=True)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    
    class Meta:
        ordering = ['-received_at']
        indexes = [
            models.Index(fields=['status', 'received_at']),
        ]
    
    def __str__(self):
        return f"Lead {self.id} - {self.status}"


class DeliveryAttempt(models.Model):
    """
    Records each attempt to deliver a lead to the Customer API.
    Maintains full audit trail of delivery attempts.
    """
    
    lead = models.ForeignKey(
        InboundLead,
        on_delete=models.CASCADE,
        related_name='delivery_attempts'
    )
    attempt_no = models.PositiveIntegerField()
    requested_at = models.DateTimeField(auto_now_add=True)
    response_status = models.PositiveIntegerField(null=True, blank=True)
    response_body = models.TextField(null=True, blank=True)
    error_message = models.TextField(null=True, blank=True)
    success = models.BooleanField(default=False)
    created_at = models.DateTimeField(auto_now_add=True)
    
    class Meta:
        ordering = ['lead', 'attempt_no']
        indexes = [
            models.Index(fields=['lead', 'attempt_no']),
        ]
    
    def __str__(self):
        return f"Attempt {self.attempt_no} for Lead {self.lead_id} - {'Success' if self.success else 'Failed'}"
