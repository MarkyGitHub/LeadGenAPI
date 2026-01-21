"""
Django admin configuration for leads app.
"""
from django.contrib import admin
from leads.models import InboundLead, DeliveryAttempt


class DeliveryAttemptInline(admin.TabularInline):
    """Inline display of delivery attempts for a lead."""
    model = DeliveryAttempt
    extra = 0
    readonly_fields = ('attempt_no', 'requested_at', 'response_status', 'response_body', 'error_message', 'success')
    can_delete = False


@admin.register(InboundLead)
class InboundLeadAdmin(admin.ModelAdmin):
    """Admin interface for InboundLead model."""
    
    list_display = ('id', 'status', 'received_at', 'rejection_reason')
    list_filter = ('status', 'received_at')
    search_fields = ('id', 'rejection_reason')
    readonly_fields = ('id', 'received_at', 'created_at', 'updated_at', 'raw_payload', 'source_headers', 
                      'normalized_payload', 'customer_payload', 'payload_hash')
    
    fieldsets = (
        ('Status', {
            'fields': ('id', 'status', 'rejection_reason')
        }),
        ('Timestamps', {
            'fields': ('received_at', 'created_at', 'updated_at')
        }),
        ('Payloads', {
            'fields': ('raw_payload', 'normalized_payload', 'customer_payload'),
            'classes': ('collapse',)
        }),
        ('Audit', {
            'fields': ('source_headers', 'payload_hash'),
            'classes': ('collapse',)
        }),
    )
    
    inlines = [DeliveryAttemptInline]
    
    def has_add_permission(self, request):
        """Disable manual lead creation through admin."""
        return False
    
    def has_delete_permission(self, request, obj=None):
        """Disable lead deletion through admin."""
        return False


@admin.register(DeliveryAttempt)
class DeliveryAttemptAdmin(admin.ModelAdmin):
    """Admin interface for DeliveryAttempt model."""
    
    list_display = ('id', 'lead', 'attempt_no', 'requested_at', 'response_status', 'success')
    list_filter = ('success', 'requested_at')
    search_fields = ('lead__id',)
    readonly_fields = ('lead', 'attempt_no', 'requested_at', 'response_status', 'response_body', 
                      'error_message', 'success', 'created_at')
    
    fieldsets = (
        ('Delivery Information', {
            'fields': ('lead', 'attempt_no', 'requested_at', 'success')
        }),
        ('Response', {
            'fields': ('response_status', 'response_body', 'error_message')
        }),
    )
    
    def has_add_permission(self, request):
        """Disable manual delivery attempt creation through admin."""
        return False
    
    def has_delete_permission(self, request, obj=None):
        """Disable delivery attempt deletion through admin."""
        return False
