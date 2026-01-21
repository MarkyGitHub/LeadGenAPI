"""
Normalization service for lead data.
"""
import logging
from typing import Any

logger = logging.getLogger(__name__)


def normalize_value(value: Any) -> Any:
    """
    Normalize a single value.
    
    - Strings: trim whitespace
    - Boolean strings: convert to actual booleans
    """
    if isinstance(value, str):
        trimmed = value.strip()
        # Convert boolean strings to actual booleans
        if trimmed.lower() == 'true':
            return True
        elif trimmed.lower() == 'false':
            return False
        return trimmed
    return value


def normalize_dict(data: dict) -> dict:
    """
    Recursively normalize all values in a dictionary.
    """
    result = {}
    for key, value in data.items():
        if isinstance(value, dict):
            result[key] = normalize_dict(value)
        elif isinstance(value, list):
            result[key] = [normalize_value(item) if not isinstance(item, dict) 
                          else normalize_dict(item) for item in value]
        else:
            result[key] = normalize_value(value)
    return result


def normalize(payload: dict) -> dict:
    """
    Normalizes lead data.
    
    Operations:
    - Lowercase and trim email addresses
    - Trim whitespace from all string fields
    - Convert boolean strings ('true', 'false') to actual booleans
    
    Args:
        payload: Raw lead data
    
    Returns:
        Normalized payload with cleaned data
    """
    if not payload:
        return {}
    
    # First, normalize all values recursively
    normalized = normalize_dict(payload)
    
    # Special handling for email: lowercase
    if 'email' in normalized and isinstance(normalized['email'], str):
        normalized['email'] = normalized['email'].lower()
    
    logger.debug(f"Normalized payload: {normalized}")
    return normalized
