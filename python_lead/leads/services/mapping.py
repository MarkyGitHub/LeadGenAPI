"""
Mapping service for transforming leads to customer API format.

The Customer API is permissive by design: invalid optional attributes are omitted
while valid core lead data is still delivered.
"""
import json
import logging
from typing import Tuple, List, Any, Optional
from pathlib import Path
from django.conf import settings

logger = logging.getLogger(__name__)


class MissingRequiredFieldError(Exception):
    """Raised when a required Core_Customer_Field is missing."""
    pass


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


def set_nested_value(data: dict, path: str, value: Any) -> None:
    """
    Set a value in a nested dictionary using dot notation.
    Creates intermediate dictionaries as needed.
    
    Args:
        data: The dictionary to modify
        path: Dot-separated path (e.g., 'product.name')
        value: Value to set
    """
    keys = path.split('.')
    current = data
    for key in keys[:-1]:
        if key not in current:
            current[key] = {}
        current = current[key]
    current[keys[-1]] = value


def load_attribute_mapping() -> dict:
    """
    Load the customer attribute mapping configuration.
    
    Returns:
        Dictionary containing attribute validation rules
    """
    mapping_path = Path(settings.ATTRIBUTE_MAPPING_PATH)
    
    if not mapping_path.exists():
        logger.warning(f"Attribute mapping file not found: {mapping_path}")
        return {}
    
    try:
        with open(mapping_path, 'r', encoding='utf-8') as f:
            mapping = json.load(f)
        logger.debug(f"Loaded {len(mapping)} attribute mappings")
        return mapping
    except Exception as e:
        logger.error(f"Error loading attribute mapping: {e}")
        return {}


def is_numeric(value: Any) -> bool:
    """
    Check if a value is numeric (int, float, or numeric string).
    
    Args:
        value: Value to check
    
    Returns:
        True if value is numeric
    """
    # Exclude booleans (bool is subclass of int in Python)
    if isinstance(value, bool):
        return False
    if isinstance(value, (int, float)):
        return True
    if isinstance(value, str):
        try:
            float(value)
            return True
        except ValueError:
            return False
    return False


def validate_attribute(value: Any, rules: dict) -> bool:
    """
    Validate an attribute value against its rules.
    
    Args:
        value: The attribute value to validate
        rules: Validation rules from customer_attribute_mapping.json
    
    Returns:
        True if valid, False if invalid (should be omitted)
    """
    attribute_type = rules.get('attribute_type')
    is_numeric_required = rules.get('is_numeric', False)
    allowed_values = rules.get('values')
    
    # Type: text
    if attribute_type == 'text':
        if is_numeric_required:
            # Must be numeric (int, float, or numeric string)
            if not is_numeric(value):
                logger.debug(f"Text attribute failed: expected numeric, got {type(value).__name__}")
                return False
        else:
            # Must be string
            if not isinstance(value, str):
                logger.debug(f"Text attribute failed: expected string, got {type(value).__name__}")
                return False
        return True
    
    # Type: dropdown
    elif attribute_type == 'dropdown':
        if allowed_values is None or len(allowed_values) == 0:
            # No restrictions, accept any value
            return True
        # Must match one of the allowed values exactly
        if value not in allowed_values:
            logger.debug(f"Dropdown attribute failed: '{value}' not in {allowed_values}")
            return False
        return True
    
    # Type: range
    elif attribute_type == 'range':
        # Must be numeric
        if not is_numeric(value):
            logger.debug(f"Range attribute failed: expected numeric, got {type(value).__name__}")
            return False
        # Note: bounds checking would go here if defined in rules
        # For now, we just validate numeric-only
        return True
    
    # Unknown type - accept by default
    logger.warning(f"Unknown attribute_type: {attribute_type}")
    return True


def map_to_customer(payload: dict) -> Tuple[dict, List[str]]:
    """
    Maps normalized lead data to customer API format.
    
    The Customer API is permissive by design: invalid attributes are omitted
    while valid core lead data is still delivered.
    
    Core Customer Fields (REQUIRED):
    - phone: Lead's phone number
    - product.name: Product name (from CUSTOMER_PRODUCT_NAME setting)
    
    Args:
        payload: Normalized lead data
    
    Returns:
        Tuple of (customer_payload, omitted_attributes)
        - customer_payload: Customer-formatted payload with valid attributes only
        - omitted_attributes: List of attribute names that were omitted due to validation failure
    
    Raises:
        MissingRequiredFieldError: If phone or product.name is missing
    """
    customer_payload = {}
    omitted_attributes = []
    
    # Validate and add Core_Customer_Fields (REQUIRED)
    phone = payload.get('phone')
    if not phone:
        raise MissingRequiredFieldError("Missing required field: phone")
    customer_payload['phone'] = phone
    
    # Set product.name from configuration (REQUIRED)
    product_name = settings.CUSTOMER_PRODUCT_NAME
    if not product_name:
        raise MissingRequiredFieldError("Missing required field: product.name (CUSTOMER_PRODUCT_NAME not configured)")
    set_nested_value(customer_payload, 'product.name', product_name)
    
    # Load attribute mapping configuration
    attribute_mapping = load_attribute_mapping()
    
    # Process optional Lead_Attributes
    for attr_name, rules in attribute_mapping.items():
        # Get value from payload (attributes are at root level in payload)
        value = payload.get(attr_name)
        
        if value is None:
            # Missing optional attributes are simply not included
            continue
        
        # Validate attribute against rules
        if not validate_attribute(value, rules):
            # Invalid attributes are omitted
            omitted_attributes.append(attr_name)
            logger.info(f"Omitted invalid attribute: {attr_name} = {value}")
            continue
        
        # Valid attribute - include in customer payload
        customer_payload[attr_name] = value
    
    # Log summary
    if omitted_attributes:
        logger.info(f"Omitted {len(omitted_attributes)} invalid attributes: {omitted_attributes}")
    
    logger.debug(f"Mapped payload with {len(customer_payload)} fields")
    return customer_payload, omitted_attributes
