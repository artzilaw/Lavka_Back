package validator

import (
	"errors"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	if err := ValidateEmail("user@mail.ru"); err != nil {
		t.Errorf("expected valid email, got %v", err)
	}
	if err := ValidateEmail("georgeFloyd@mail.ru"); err != nil {
		t.Errorf("expected uppercase email georgeFloyd@mail.ru to be valid, got %v", err)
	}
	if err := ValidateEmail("invalid-email"); !errors.Is(err, ErrInvalidEmailFormat) {
		t.Errorf("expected ErrInvalidEmailFormat, got %v", err)
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		pass string
		want error
	}{
		{"ValidPass123", nil},
		{"short1A", ErrPasswordTooShort},
		{"OnlyLetters", ErrPasswordRequiresDigit},
		{"123456789", ErrPasswordRequiresLetter},
		{"BadPass!123", ErrPasswordContainsForbidden},
	}

	for _, tt := range tests {
		if err := ValidatePassword(tt.pass); !errors.Is(err, tt.want) {
			t.Errorf("ValidatePassword(%q) = %v, want %v", tt.pass, err, tt.want)
		}
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName("Иван_123"); err != nil {
		t.Errorf("expected valid name, got %v", err)
	}
	if err := ValidateName(""); !errors.Is(err, ErrNameEmpty) {
		t.Errorf("expected ErrNameEmpty, got %v", err)
	}
	if err := ValidateName("A"); !errors.Is(err, ErrNameTooShort) {
		t.Errorf("expected ErrNameTooShort, got %v", err)
	}
	if err := ValidateName("Invalid@Name!"); !errors.Is(err, ErrNameInvalid) {
		t.Errorf("expected ErrNameInvalid, got %v", err)
	}
}
