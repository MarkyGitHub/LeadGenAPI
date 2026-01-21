"""
Unit tests for normalization service.
"""
import pytest
from leads.services.normalization import normalize, normalize_value


class TestNormalizeValue:
    """Tests for the normalize_value helper function."""
    
    def test_trim_whitespace(self):
        assert normalize_value('  hello  ') == 'hello'
        assert normalize_value('\ttest\n') == 'test'
    
    def test_boolean_string_true(self):
        assert normalize_value('true') is True
        assert normalize_value('True') is True
        assert normalize_value('TRUE') is True
        assert normalize_value('  true  ') is True
    
    def test_boolean_string_false(self):
        assert normalize_value('false') is False
        assert normalize_value('False') is False
        assert normalize_value('FALSE') is False
        assert normalize_value('  false  ') is False
    
    def test_non_string_passthrough(self):
        assert normalize_value(123) == 123
        assert normalize_value(True) is True
        assert normalize_value(False) is False
        assert normalize_value(None) is None


class TestNormalizeEmail:
    """Tests for email normalization."""
    
    def test_lowercase_email(self):
        payload = {'email': 'Test@Example.COM'}
        result = normalize(payload)
        assert result['email'] == 'test@example.com'
    
    def test_trim_email_whitespace(self):
        payload = {'email': '  test@example.com  '}
        result = normalize(payload)
        assert result['email'] == 'test@example.com'
    
    def test_lowercase_and_trim_email(self):
        payload = {'email': '  TEST@EXAMPLE.COM  '}
        result = normalize(payload)
        assert result['email'] == 'test@example.com'
    
    def test_already_normalized_email(self):
        payload = {'email': 'test@example.com'}
        result = normalize(payload)
        assert result['email'] == 'test@example.com'


class TestNormalizeWhitespace:
    """Tests for whitespace trimming."""
    
    def test_trim_string_fields(self):
        payload = {
            'name': '  John Doe  ',
            'phone': ' +49123456789 '
        }
        result = normalize(payload)
        assert result['name'] == 'John Doe'
        assert result['phone'] == '+49123456789'
    
    def test_trim_nested_string_fields(self):
        payload = {
            'address': {
                'street': '  123 Main St  ',
                'city': ' Berlin '
            }
        }
        result = normalize(payload)
        assert result['address']['street'] == '123 Main St'
        assert result['address']['city'] == 'Berlin'


class TestNormalizeBooleans:
    """Tests for boolean conversion."""
    
    def test_convert_boolean_strings(self):
        payload = {
            'house': {
                'is_owner': 'true'
            }
        }
        result = normalize(payload)
        assert result['house']['is_owner'] is True
    
    def test_convert_false_string(self):
        payload = {
            'house': {
                'is_owner': 'false'
            }
        }
        result = normalize(payload)
        assert result['house']['is_owner'] is False
    
    def test_preserve_actual_booleans(self):
        payload = {
            'house': {
                'is_owner': True
            }
        }
        result = normalize(payload)
        assert result['house']['is_owner'] is True


class TestNormalizeEdgeCases:
    """Tests for edge cases."""
    
    def test_empty_payload(self):
        result = normalize({})
        assert result == {}
    
    def test_none_payload(self):
        result = normalize(None)
        assert result == {}
    
    def test_preserve_numbers(self):
        payload = {
            'address': {
                'zip': 66123
            }
        }
        result = normalize(payload)
        assert result['address']['zip'] == 66123
    
    def test_complex_nested_structure(self):
        payload = {
            'email': '  TEST@EXAMPLE.COM  ',
            'address': {
                'zip': '  66123  ',
                'street': '  Main St  '
            },
            'house': {
                'is_owner': 'true'
            }
        }
        result = normalize(payload)
        assert result['email'] == 'test@example.com'
        assert result['address']['zip'] == '66123'
        assert result['address']['street'] == 'Main St'
        assert result['house']['is_owner'] is True
