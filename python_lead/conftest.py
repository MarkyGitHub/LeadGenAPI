import os
import sys
import pytest
import django

# Add the project root to the path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'lead_gateway.settings')


def pytest_configure(config):
    """Configure Django settings for pytest."""
    from django.conf import settings
    
    # Only configure if not already configured
    if not settings.configured:
        settings.configure(
            DEBUG=True,
            DATABASES={
                'default': {
                    'ENGINE': 'django.db.backends.sqlite3',
                    'NAME': ':memory:',
                }
            },
            INSTALLED_APPS=[
                'django.contrib.contenttypes',
                'django.contrib.auth',
                'leads',
            ],
            SECRET_KEY='test-secret-key',
            USE_TZ=True,
            # Celery settings for tests
            CELERY_BROKER_URL='memory://',
            CELERY_RESULT_BACKEND='cache+memory://',
            CELERY_TASK_ALWAYS_EAGER=True,
            CELERY_TASK_EAGER_PROPAGATES=True,
            # Customer API settings
            CUSTOMER_API_URL='https://contactapi.static.fyi/lead/receive/fake/USER_ID/',
            CUSTOMER_TOKEN='FakeCustomerToken',
            CUSTOMER_PRODUCT_NAME='Solaranlage',
            ATTRIBUTE_MAPPING_PATH='customer_attribute_mapping.json',
        )
        
        django.setup()
    else:
        # Override database settings for tests
        settings.DATABASES = {
            'default': {
                'ENGINE': 'django.db.backends.sqlite3',
                'NAME': ':memory:',
            }
        }


@pytest.fixture
def valid_lead_payload():
    """Return a valid lead payload for testing."""
    return {
        'email': 'test@example.com',
        'phone': '+49123456789',
        'address': {
            'zip': '66123',
            'street': '123 Main St'
        },
        'house': {
            'is_owner': True
        }
    }


@pytest.fixture
def invalid_zipcode_payload():
    """Return a payload with invalid zipcode."""
    return {
        'email': 'test@example.com',
        'phone': '+49123456789',
        'address': {
            'zip': '12345',
            'street': '123 Main St'
        },
        'house': {
            'is_owner': True
        }
    }


@pytest.fixture
def non_homeowner_payload():
    """Return a payload where house.is_owner is False."""
    return {
        'email': 'test@example.com',
        'phone': '+49123456789',
        'address': {
            'zip': '66123',
            'street': '123 Main St'
        },
        'house': {
            'is_owner': False
        }
    }
