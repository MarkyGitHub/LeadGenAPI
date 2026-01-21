"""
Celery configuration for Lead Gateway Service.
"""
import os
from celery import Celery

os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'lead_gateway.settings')

app = Celery('lead_gateway')
app.config_from_object('django.conf:settings', namespace='CELERY')
app.autodiscover_tasks()
