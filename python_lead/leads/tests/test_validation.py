"""
Unit tests for validation service.
"""
import pytest
from leads.services.validation import (
    validate_lead,
    get_nested_value,
    ZIPCODE_PATTERN_ERROR,
    NOT_HOMEOWNER,
    MISSING_REQUIRED_FIELD,
)


class TestGetNestedValue:
    """Tests for the get_nested_value helper function."""
    
    def test_simple_key(self):
        data = {'email': 'test@example.com'}
        assert get_nested_value(data, 'email') == 'test@example.com'
    
    def test_nested_key_with_dots(self):
        data = {'address': {'zip': '53859'}}
        assert get_nested_value(data, 'address.zip') == '53859'
    
    def test_bracket_notation_for_dict_keys(self):
        """Test bracket notation for dictionary keys with special characters."""
        data = {'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}}
        assert get_nested_value(data, 'questions[Sind Sie Eigentümer der Immobilie?]') == 'Ja'
    
    def test_missing_key_returns_default(self):
        data = {'email': 'test@example.com'}
        assert get_nested_value(data, 'phone') is None
        assert get_nested_value(data, 'phone', 'default') == 'default'


class TestValidateLeadZipcode:
    """Tests for zipcode validation."""
    
    def test_valid_zipcode_53859(self):
        """Test valid zipcode starting with 53."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_valid_zipcode_53000(self):
        """Test boundary: lowest valid zipcode."""
        payload = {
            'zipcode': '53000',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_valid_zipcode_53999(self):
        """Test boundary: highest valid zipcode."""
        payload = {
            'zipcode': '53999',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    
    def test_invalid_zipcode_12345(self):
        """Test invalid zipcode not starting with 53."""
        payload = {
            'zipcode': '12345',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIPCODE_PATTERN_ERROR
    
    def test_invalid_zipcode_66123(self):
        """Test old format zipcode starting with 66."""
        payload = {
            'zipcode': '66123',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == ZIPCODE_PATTERN_ERROR
    
    def test_missing_zipcode(self):
        """Test missing zipcode."""
        payload = {
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD


class TestValidateLeadHomeownership:
    """Tests for homeownership validation via questions."""
    
    def test_homeowner_ja(self):
        """Test valid homeowner with 'Ja' answer."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None
    def test_homeowner_true_string(self):
        """Test that homeowner check passes with 'true' (string)."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'true'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None

    def test_homeowner_true_boolean(self):
        """Test that homeowner check passes with True (boolean)."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': True}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is True
        assert reason is None    
    def test_homeowner_nein(self):
        """Test invalid: homeowner answers 'Nein'."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Nein'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == NOT_HOMEOWNER
    
    def test_homeowner_wrong_answer(self):
        """Test invalid: homeowner gives wrong answer."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Vielleicht'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == NOT_HOMEOWNER
    
    def test_missing_homeowner_question(self):
        """Test missing homeowner question."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD


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
    
    def test_missing_email(self):
        """Test missing email."""
        payload = {
            'zipcode': '53859',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_phone(self):
        """Test missing phone."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_first_name(self):
        """Test missing first_name."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_last_name(self):
        """Test missing last_name."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'city': 'Niederkassel',
            'first_name': 'Rainer',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
    
    def test_missing_city(self):
        """Test missing city."""
        payload = {
            'zipcode': '53859',
            'email': 'test@example.com',
            'phone': '0160 8912308',
            'street': 'Ommerich Str 119',
            'first_name': 'Rainer',
            'last_name': 'Simossek',
            'questions': {'Sind Sie Eigentümer der Immobilie?': 'Ja'}
        }
        is_valid, reason = validate_lead(payload)
        assert is_valid is False
        assert reason == MISSING_REQUIRED_FIELD
