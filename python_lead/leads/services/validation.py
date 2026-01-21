"""
Validation service for lead business rules.
"""
import re
import logging
from typing import Tuple, Optional

from django.conf import settings

logger = logging.getLogger(__name__)

# Rejection codes (configurable in settings)
ZIP_NOT_66XXX = getattr(settings, 'ZIP_NOT_66XXX', 'ZIP_NOT_66XXX')
NOT_HOMEOWNER = getattr(settings, 'NOT_HOMEOWNER', 'NOT_HOMEOWNER')
MISSING_REQUIRED_FIELD = getattr(settings, 'MISSING_REQUIRED_FIELD', 'MISSING_REQUIRED_FIELD')

# Zipcode pattern (configurable in settings)
ZIPCODE_PATTERN = re.compile(getattr(settings, 'ZIPCODE_PATTERN', r'^66\d{3}$'))


def get_nested_value(data: dict, path: str, default=None):
    """
    Get a value from a nested dictionary using dot notation.
    
    Args:
        data: The dictionary to search
        path: Dot-separated path (e.g., 'address.zip')
        default: Default value if path not found
    
    Returns:
        The value at the path or default
    """
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
    1. Zipcode must match pattern ^66\\d{3}$ (66 followed by 3 digits)
    2. house.is_owner must be exactly True (boolean)
    
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
    zipcode = get_nested_value(payload, 'address.zip')
    if zipcode is None:
        logger.debug("Validation failed: missing address.zip")
        return False, MISSING_REQUIRED_FIELD
    
    # Convert to string for pattern matching
    zipcode_str = str(zipcode)
    if not ZIPCODE_PATTERN.match(zipcode_str):
        logger.debug(f"Validation failed: zipcode '{zipcode_str}' does not match pattern ^66\\d{{3}}$")
        return False, ZIP_NOT_66XXX
    
    # Validate homeownership
    is_owner = get_nested_value(payload, 'house.is_owner')
    if is_owner is None:
        logger.debug("Validation failed: missing house.is_owner")
        return False, MISSING_REQUIRED_FIELD
    
    # Must be exactly True (boolean), not truthy
    if is_owner is not True:
        logger.debug(f"Validation failed: house.is_owner is {is_owner}, expected True")
        return False, NOT_HOMEOWNER
    
    logger.debug("Validation passed")
    return True, None
