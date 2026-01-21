"""
URL configuration for leads app.
"""
from django.urls import path
from leads.views import LeadWebhookView

urlpatterns = [
    path('leads/', LeadWebhookView.as_view(), name='lead-webhook'),
]
