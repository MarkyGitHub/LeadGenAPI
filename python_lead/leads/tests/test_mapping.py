"""
Unit tests for mapping service.
"""
import pytest
from unittest.mock import patch, mock_open
from leads.services.mapping import (
    map_to_customer,
    get_nested_value,
    set_nested_value,
    validate_attribute,
    is_numeric,
    MissingRequiredFieldError,
)


class TestGetNestedValue:
    """Tests for get_nested_value helper."""
    
    def test_simple_key(self):
        data = {'email': 'test@example.com'}
        assert get_nested_value(data, 'email') == 'test@example.com'
    
    def test_nested_key(self):
        data = {'product': {'name': 'Solar'}}
        assert get_nested_value(data, 'product.name') == 'Solar'
    
    def test_missing_key(self):
        data = {'email': 'test@example.com'}
        assert get_nested_value(data, 'phone') is None


class TestSetNestedValue:
    """Tests for set_nested_value helper."""
    
    def test_simple_key(self):
        data = {}
        set_nested_value(data, 'email', 'test@example.com')
        assert data['email'] == 'test@example.com'
    
    def test_nested_key(self):
        data = {}
        set_nested_value(data, 'product.name', 'Solar')
        assert data['product']['name'] == 'Solar'
    
    def test_deeply_nested_key(self):
        data = {}
        set_nested_value(data, 'a.b.c', 'value')
        assert data['a']['b']['c'] == 'value'


class TestIsNumeric:
    """Tests for is_numeric helper."""
    
    def test_integer(self):
        assert is_numeric(123) is True
        assert is_numeric(0) is True
        assert is_numeric(-5) is True
    
    def test_float(self):
        assert is_numeric(123.45) is True
        assert is_numeric(0.0) is True
        assert is_numeric(-5.5) is True
    
    def test_numeric_string(self):
        assert is_numeric('123') is True
        assert is_numeric('123.45') is True
        assert is_numeric('-5.5') is True
    
    def test_non_numeric_string(self):
        assert is_numeric('abc') is False
        assert is_numeric('12abc') is False
        assert is_numeric('') is False
    
    def test_other_types(self):
        assert is_numeric(True) is False
        assert is_numeric(None) is False
        assert is_numeric([]) is False


class TestValidateAttribute:
    """Tests for validate_attribute function."""
    
    def test_text_attribute_string(self):
        rules = {'attribute_type': 'text', 'is_numeric': False}
        assert validate_attribute('some text', rules) is True
    
    def test_text_attribute_numeric_required(self):
        rules = {'attribute_type': 'text', 'is_numeric': True}
        assert validate_attribute(123, rules) is True
        assert validate_attribute('123', rules) is True
        assert validate_attribute('abc', rules) is False
    
    def test_dropdown_attribute_valid(self):
        rules = {'attribute_type': 'dropdown', 'values': ['Ja', 'Nein']}
        assert validate_attribute('Ja', rules) is True
        assert validate_attribute('Nein', rules) is True
    
    def test_dropdown_attribute_invalid(self):
        rules = {'attribute_type': 'dropdown', 'values': ['Ja', 'Nein']}
        assert validate_attribute('Maybe', rules) is False
        assert validate_attribute('yes', rules) is False
    
    def test_dropdown_attribute_no_restrictions(self):
        rules = {'attribute_type': 'dropdown', 'values': None}
        assert validate_attribute('anything', rules) is True
        assert validate_attribute(123, rules) is True
    
    def test_range_attribute_numeric(self):
        rules = {'attribute_type': 'range', 'is_numeric': True}
        assert validate_attribute(100, rules) is True
        assert validate_attribute('100', rules) is True
        assert validate_attribute(100.5, rules) is True
    
    def test_range_attribute_non_numeric(self):
        rules = {'attribute_type': 'range', 'is_numeric': True}
        assert validate_attribute('abc', rules) is False


class TestMapToCustomerRequiredFields:
    """Tests for required Core_Customer_Fields."""
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_missing_phone_raises_error(self, mock_load, mock_settings):
        """Test that missing phone raises MissingRequiredFieldError."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {}
        
        payload = {'email': 'test@example.com'}
        
        with pytest.raises(MissingRequiredFieldError, match='phone'):
            map_to_customer(payload)
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_missing_product_name_raises_error(self, mock_load, mock_settings):
        """Test that missing product.name (CUSTOMER_PRODUCT_NAME) raises error."""
        mock_settings.CUSTOMER_PRODUCT_NAME = None
        mock_load.return_value = {}
        
        payload = {'phone': '+49123456789'}
        
        with pytest.raises(MissingRequiredFieldError, match='product.name'):
            map_to_customer(payload)
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_valid_required_fields(self, mock_load, mock_settings):
        """Test that valid required fields are included."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {}
        
        payload = {'phone': '+49123456789'}
        
        result, omitted = map_to_customer(payload)
        
        assert result['phone'] == '+49123456789'
        assert result['product']['name'] == 'Solar'
        assert omitted == []


class TestMapToCustomerOptionalAttributes:
    """Tests for optional Lead_Attributes."""
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_valid_attributes_included(self, mock_load, mock_settings):
        """Test that all valid attributes are included."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {
            'solar_area': {'attribute_type': 'range', 'is_numeric': True, 'values': None},
            'solar_owner': {'attribute_type': 'dropdown', 'is_numeric': False, 'values': ['Ja', 'Nein']},
        }
        
        payload = {
            'phone': '+49123456789',
            'solar_area': 100,
            'solar_owner': 'Ja',
        }
        
        result, omitted = map_to_customer(payload)
        
        assert result['phone'] == '+49123456789'
        assert result['product']['name'] == 'Solar'
        assert result['solar_area'] == 100
        assert result['solar_owner'] == 'Ja'
        assert omitted == []
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_invalid_dropdown_omitted(self, mock_load, mock_settings):
        """Test that invalid dropdown value is omitted, lead continues."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {
            'solar_owner': {'attribute_type': 'dropdown', 'is_numeric': False, 'values': ['Ja', 'Nein']},
        }
        
        payload = {
            'phone': '+49123456789',
            'solar_owner': 'InvalidValue',
        }
        
        result, omitted = map_to_customer(payload)
        
        assert result['phone'] == '+49123456789'
        assert result['product']['name'] == 'Solar'
        assert 'solar_owner' not in result
        assert 'solar_owner' in omitted
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_invalid_numeric_omitted(self, mock_load, mock_settings):
        """Test that invalid numeric value is omitted, lead continues."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {
            'solar_area': {'attribute_type': 'range', 'is_numeric': True, 'values': None},
        }
        
        payload = {
            'phone': '+49123456789',
            'solar_area': 'not_a_number',
        }
        
        result, omitted = map_to_customer(payload)
        
        assert result['phone'] == '+49123456789'
        assert result['product']['name'] == 'Solar'
        assert 'solar_area' not in result
        assert 'solar_area' in omitted
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_mix_valid_invalid_attributes(self, mock_load, mock_settings):
        """Test lead with mix of valid and invalid attributes - only valid included."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {
            'solar_area': {'attribute_type': 'range', 'is_numeric': True, 'values': None},
            'solar_owner': {'attribute_type': 'dropdown', 'is_numeric': False, 'values': ['Ja', 'Nein']},
            'solar_usage': {'attribute_type': 'dropdown', 'is_numeric': False, 'values': ['Eigenverbrauch', 'Netzeinspeisung']},
        }
        
        payload = {
            'phone': '+49123456789',
            'solar_area': 100,  # Valid
            'solar_owner': 'InvalidValue',  # Invalid
            'solar_usage': 'Eigenverbrauch',  # Valid
        }
        
        result, omitted = map_to_customer(payload)
        
        assert result['phone'] == '+49123456789'
        assert result['product']['name'] == 'Solar'
        assert result['solar_area'] == 100
        assert result['solar_usage'] == 'Eigenverbrauch'
        assert 'solar_owner' not in result
        assert omitted == ['solar_owner']
    
    @patch('leads.services.mapping.settings')
    @patch('leads.services.mapping.load_attribute_mapping')
    def test_missing_optional_attributes_not_included(self, mock_load, mock_settings):
        """Test that missing optional attributes are simply not included."""
        mock_settings.CUSTOMER_PRODUCT_NAME = 'Solar'
        mock_load.return_value = {
            'solar_area': {'attribute_type': 'range', 'is_numeric': True, 'values': None},
            'solar_owner': {'attribute_type': 'dropdown', 'is_numeric': False, 'values': ['Ja', 'Nein']},
        }
        
        payload = {
            'phone': '+49123456789',
            # No optional attributes provided
        }
        
        result, omitted = map_to_customer(payload)
        
        assert result['phone'] == '+49123456789'
        assert result['product']['name'] == 'Solar'
        assert 'solar_area' not in result
        assert 'solar_owner' not in result
        assert omitted == []
