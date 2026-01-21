"""
Unit tests for validation service.
"""
import pytest
from leads.services.validation import (
    validate_lead,
    get_nested_value,
    ZIP_NOT_66XXX,
    NOT_HOMEOWNER,
    MISSING_REQUIRED_FIELD,
)


class TestGetNestedValue:
    """Tests for the get_nested_value helper function."""
    
    def test_simple_key(self):
        data = {'email': 'test@example.com'}
        assert get_nested_value(data, 'email') == 'test@example.com'
    
    def test_nested_key(self):
        data = {'address': {'zip': '66123'}}
        assert get_nested_value(data, 'address.zip') == '66123'
    
    def test_deeply_nested_key(self):
        data = {'a': {'b': {'c': 'value'}}}
        assert get_nested_value(data, 'a.b.c') == 'value'
    
    def test_missing_key_returns_default(self):
        data = {'email': 'test@example.com'}
        assert get_nested_value(data, 'phone') is None
        assert get_nested_value(data, 'phone', 'default') == 'default'
    
    def test_missing_nested_key_returns_default(self):
        data = {'address': {'street': '123 Main St'}}
        assert get_nested_value(data, 'address.zip') is None


class TestValidateLeadZipcode:
    """Tests for zipcode validation."""
    
    def test_valid_zipcode_66123(self):
        """Test valid zipcode starting with 66."""
        payload = {
            'address': {'zip': '66123'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_valid_zipcode_66000(self):
        """Test boundary: lowest valid zipcode."""
        payload = {
            'address': {'zip': '66000'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_valid_zipcode_66999(self):
        """Test boundary: highest valid zipcode."""
        payload = {
            'address': {'zip': '66999'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_invalid_zipcode_12345(self):
        """Test invalid zipcode not starting with 66."""
        payload = {
            'address': {'zip': '12345'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIP_NOT_66XXX
    
    def test_invalid_zipcode_65999(self):
        """Test boundary: just below valid range."""
        payload = {
            'address': {'zip': '65999'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIP_NOT_66XXX
    
    def test_invalid_zipcode_67000(self):
        """Test boundary: just above valid range."""
        payload = {
            'address': {'zip': '67000'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIP_NOT_66XXX
    
    def test_invalid_zipcode_too_short(self):
        """Test zipcode with too few digits."""
        payload = {
            'address': {'zip': '6612'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIP_NOT_66XXX
    
    def test_invalid_zipcode_too_long(self):
        """Test zipcode with too many digits."""
        payload = {
            'address': {'zip': '661234'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIP_NOT_66XXX
    
    def test_zipcode_as_integer(self):
        """Test that integer zipcodes are handled correctly."""
        payload = {
            'address': {'zip': 66123},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None


class TestValidateLeadHomeownership:
    """Tests for homeownership validation."""
    
    def test_homeowner_true(self):
        """Test valid homeowner with True boolean."""
        payload = {
            'address': {'zip': '66123'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_homeowner_false(self):
        """Test invalid: homeowner is False."""
        payload = {
            'address': {'zip': '66123'},
            'house': {'is_owner': False}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == NOT_HOMEOWNER
    
    def test_homeowner_string_true(self):
        """Test invalid: homeowner is string 'true' not boolean."""
        payload = {
            'address': {'zip': '66123'},
            'house': {'is_owner': 'true'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == NOT_HOMEOWNER
    
    def test_homeowner_integer_one(self):
        """Test invalid: homeowner is integer 1 not boolean True."""
        payload = {
            'address': {'zip': '66123'},
            'house': {'is_owner': 1}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == NOT_HOMEOWNER


class TestValidateLeadMissingFields:
    """Tests for missing required fields."""
    
    def test_empty_payload(self):
        """Test empty payload."""
        is_valid, reason = validate_lead({})
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_none_payload(self):
        """Test None payload."""
        is_valid, reason = validate_lead(None)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_address(self):
        """Test missing address object."""
        payload = {
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_zipcode(self):
        """Test missing zipcode in address."""
        payload = {
            'address': {'street': '123 Main St'},
            'house': {'is_owner': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_house(self):
        """Test missing house object."""
        payload = {
            'address': {'zip': '66123'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_is_owner(self):
        """Test missing is_owner in house."""
        payload = {
            'address': {'zip': '66123'},
            'house': {'type': 'single_family'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
