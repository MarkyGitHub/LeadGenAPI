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
            CUSTOMER_TOKEN='Bearer FakeCustomerToken',
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
        'city': 'Niederkassel',
        'email': 'rainer.simossek@t-online.de',
        'phone': '0160 8912308',
        'street': 'Ommerich Str 119',
        'comment': '',
        'zipcode': '53859',
        'last_name': 'Simossek',
        'lead_type': 'phone',
        'first_name': 'Rainer',
        'questions': {
            'Dachfläche': '40',
            'Dachgefälle': '45',
            'Dachmaterial': 'Dachziegel',
            'Finanzierung': 'Nicht sicher',
            'Dachausrichtung': 'Süd/West',
            'Wallbox gewünscht': 'Nein',
            'Wie alt ist Ihr Dach?': 'Vor 1990',
            'Stromspeicher gewünscht': 'Ja',
            'Sind Sie Eigentümer der Immobilie?': 'Ja',
            'Wann soll das Projekt gestartet werden?': '6',
            'Welche Dachform haben Sie auf Ihrem Haus?': 'Satteldach',
            'Wie hoch schätzen Sie ihren Stromverbrauch?': '5000',
            'Wo möchten Sie die Solaranlage installieren?': 'Einfamilienhaus'
        },
        'created_at': 1751013978
    }


@pytest.fixture
def invalid_zipcode_payload():
    """Return a payload with invalid zipcode."""
    return {
        'city': 'Niederkassel',
        'email': 'rainer.simossek@t-online.de',
        'phone': '0160 8912308',
        'street': 'Ommerich Str 119',
        'comment': '',
        'zipcode': '12345',  # Invalid: not 53XXX
        'last_name': 'Simossek',
        'lead_type': 'phone',
        'first_name': 'Rainer',
        'questions': {
            'Sind Sie Eigentümer der Immobilie?': 'Ja'
        }
    }


@pytest.fixture
def non_homeowner_payload():
    """Return a payload where homeowner answer is not 'Ja'."""
    return {
        'city': 'Niederkassel',
        'email': 'rainer.simossek@t-online.de',
        'phone': '0160 8912308',
        'street': 'Ommerich Str 119',
        'comment': '',
        'zipcode': '53859',
        'last_name': 'Simossek',
        'lead_type': 'phone',
        'first_name': 'Rainer',
        'questions': {
            'Sind Sie Eigentümer der Immobilie?': 'Nein'  # Invalid
        }
    }
