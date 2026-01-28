"""
Validation service for lead business rules.
"""
import re
import logging
from typing import Tuple, Optional

from django.conf import settings

logger = logging.getLogger(__name__)

# Rejection codes (configurable in settings)
ZIPCODE_PATTERN_ERROR = getattr(settings, 'ZIPCODE_PATTERN_ERROR', 'ZIPCODE_INVALID')
NOT_HOMEOWNER = getattr(settings, 'NOT_HOMEOWNER', 'NOT_HOMEOWNER')
MISSING_REQUIRED_FIELD = getattr(settings, 'MISSING_REQUIRED_FIELD', 'MISSING_REQUIRED_FIELD')

# Zipcode pattern (configurable in settings)
ZIPCODE_PATTERN = re.compile(getattr(settings, 'ZIPCODE_PATTERN', r'^53\d{3}$'))


def get_nested_value(data: dict, path: str, default=None):
    """
    Get a value from a nested dictionary using dot notation.
    
    Args:
        data: The dictionary to search
        path: Dot-separated path (e.g., 'questions.Dachfläche') or bracket notation (e.g., 'questions[Sind Sie Eigentümer der Immobilie?]')
        default: Default value if path not found
    
    Returns:
        The value at the path or default
    """
    # Handle bracket notation for dictionary keys with special characters
    if '[' in path and ']' in path:
        # Extract key from bracket notation: questions[Sind Sie Eigentümer der Immobilie?]
        match = re.match(r'(\w+)\[(.+?)\]', path)
        if match:
            parent_key, nested_key = match.groups()
            value = data.get(parent_key, {})
            if isinstance(value, dict):
                return value.get(nested_key, default)
            return default
    
    # Handle dot notation
    keys = path.split('.')
    value = data
    for key in keys:
        if isinstance(value, dict) and key in value:
            value = value[key]
        else:
            return default
    return value


def validate_lead(payload: dict) -> Tuple[bool, Optional[str]]:
    """
    Validates a lead against business rules.
    
    Business Rules:
    1. zipcode must match pattern ^53\\d{3}$ (53 followed by 3 digits)
    2. questions["Sind Sie Eigentümer der Immobilie?"] must be "Ja", "true" (string), or True (boolean)
    3. Required fields: zipcode, questions["Sind Sie Eigentümer der Immobilie?"], email, phone, street, city, first_name, last_name
    
    Args:
        payload: Raw lead data dictionary
    
    Returns:
        Tuple of (is_valid, rejection_reason)
        - is_valid: True if lead passes all validation rules
        - rejection_reason: Rejection code if validation fails, None otherwise
    """
    logger.debug("Validating lead payload: %s", payload)
    if not payload:
        logger.debug("Validation failed: empty payload")
        return False, MISSING_REQUIRED_FIELD
    
    # Validate zipcode
    zipcode = payload.get('zipcode')
    if zipcode is None:
        logger.debug("Validation failed: missing zipcode")
        return False, MISSING_REQUIRED_FIELD
    
    # Convert to string for pattern matching
    zipcode_str = str(zipcode)
    if not ZIPCODE_PATTERN.match(zipcode_str):
        logger.debug(f"Validation failed: zipcode '{zipcode_str}' does not match pattern ^53\\d{{3}}$")
        return False, ZIPCODE_PATTERN_ERROR
    
    # Validate homeownership via questions
    questions = payload.get('questions', {})
    if not isinstance(questions, dict):
        logger.debug("Validation failed: questions is not a dictionary")
        return False, MISSING_REQUIRED_FIELD
    
    is_owner = questions.get('Sind Sie Eigentümer der Immobilie?')
    if is_owner is None:
        logger.debug("Validation failed: missing questions['Sind Sie Eigentümer der Immobilie?']")
        return False, MISSING_REQUIRED_FIELD
    
    # Accept "Ja" (string), "true" (string), or True (boolean)
    valid_values = ["Ja", "true", True]
    if is_owner not in valid_values:
        logger.debug(f"Validation failed: is_owner is '{is_owner}', expected one of {valid_values}")
        return False, NOT_HOMEOWNER
    
    # Validate required fields
    required_fields = ['email', 'phone', 'street', 'city', 'first_name', 'last_name']
    for field in required_fields:
        if not payload.get(field):
            logger.debug(f"Validation failed: missing or empty required field '{field}'")
            return False, MISSING_REQUIRED_FIELD
    
    logger.debug("Validation passed")
    return True, None
