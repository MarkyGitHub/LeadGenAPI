"""
URL configuration for lead_gateway project.
"""
from django.contrib import admin
from django.urls import path, include

urlpatterns = [
    path('admin/', admin.site.urls),
    path('webhooks/', include('leads.urls')),
]
