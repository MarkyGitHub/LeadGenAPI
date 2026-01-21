"""
Property-based tests for validation service.

Feature: lead-gateway-service
Property 7: Zipcode Validation
Validates: Requirements 2.1
"""
import pytest
from hypothesis import given, settings, assume
import hypothesis.strategies as st

from leads.services.validation import validate_lead, ZIP_NOT_66XXX


class TestZipcodeValidationProperty:
    """
    Property 7: Zipcode Validation
    
    For any lead with a zipcode, validation SHALL pass if and only if
    the zipcode matches the pattern ^66\\d{3}$.
    
    **Validates: Requirements 2.1**
    """
    
    @settings(max_examples=100)
    @given(
        digits=st.integers(min_value=0, max_value=999)
    )
    def test_valid_zipcodes_pass_validation(self, digits):
        """
        Feature: lead-gateway-service, Property 7: Zipcode Validation
        
        For any zipcode matching ^66\\d{3}$, validation SHALL pass.
        """
        # Generate valid zipcode: 66 + 3 digits (padded with zeros)
        zipcode = f"66{digits:03d}"
        
        payload = {
            'address': {'zip': zipcode},
            'house': {'is_owner': True}
        }
        
        is_valid, reason = validate_lead(payload)
        
        assert is_valid is True, f"Valid zipcode {zipcode} should pass validation"
        assert reason is None
    
    @settings(max_examples=100)
    @given(
        prefix=st.integers(min_value=0, max_value=99).filter(lambda x: x != 66),
        suffix=st.integers(min_value=0, max_value=999)
    )
    def test_invalid_prefix_zipcodes_fail_validation(self, prefix, suffix):
        """
        Feature: lead-gateway-service, Property 7: Zipcode Validation
        
        For any zipcode NOT starting with 66, validation SHALL fail with ZIP_NOT_66XXX.
        """
        # Generate invalid zipcode: not starting with 66
        zipcode = f"{prefix:02d}{suffix:03d}"
        
        # Ensure it doesn't accidentally start with 66
        assume(not zipcode.startswith('66'))
        
        payload = {
            'address': {'zip': zipcode},
            'house': {'is_owner': True}
        }
        
        is_valid, reason = validate_lead(payload)
        
        assert is_valid is False, f"Invalid zipcode {zipcode} should fail validation"
        assert reason == ZIP_NOT_66XXX
    
    @settings(max_examples=50)
    @given(
        length=st.integers(min_value=1, max_value=4).filter(lambda x: x != 5)
    )
    def test_wrong_length_zipcodes_fail_validation(self, length):
        """
        Feature: lead-gateway-service, Property 7: Zipcode Validation
        
        For any zipcode with wrong length (not 5 digits), validation SHALL fail.
        """
        # Generate zipcode starting with 66 but wrong length
        if length <= 2:
            zipcode = "66"[:length]
        else:
            zipcode = "66" + "1" * (length - 2)
        
        payload = {
            'address': {'zip': zipcode},
            'house': {'is_owner': True}
        }
        
        is_valid, reason = validate_lead(payload)
        
        assert is_valid is False, f"Zipcode {zipcode} with length {length} should fail validation"
        assert reason == ZIP_NOT_66XXX
