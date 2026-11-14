package validator

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalidEmailFormat        = errors.New("invalid email format")
	ErrPasswordTooShort          = errors.New("password must be at least 8 characters long")
	ErrPasswordRequiresDigit     = errors.New("password must contain at least one digit")
	ErrPasswordRequiresLetter    = errors.New("password must contain at least one latin letter")
	ErrPasswordContainsForbidden = errors.New("password contains forbidden characters")
	ErrNameEmpty                 = errors.New("name cannot be empty")
	ErrNameTooShort              = errors.New("name must be at least 2 characters long")
	ErrNameInvalid               = errors.New("name contains invalid characters")

	reStrict    = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	reHasLetter = regexp.MustCompile(`[a-zA-Z]`)
	reHasDigit  = regexp.MustCompile(`[0-9]`)
	emailRegex  = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	nameRegex   = regexp.MustCompile(`^[\p{L}a-zA-Z0-9_\s'-]+$`)
)

func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if !emailRegex.MatchString(email) {
		return ErrInvalidEmailFormat
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	if !reStrict.MatchString(password) {
		return ErrPasswordContainsForbidden
	}
	if !reHasLetter.MatchString(password) {
		return ErrPasswordRequiresLetter
	}
	if !reHasDigit.MatchString(password) {
		return ErrPasswordRequiresDigit
	}
	return nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameEmpty
	}
	if utf8.RuneCountInString(name) < 2 {
		return ErrNameTooShort
	}
	if !nameRegex.MatchString(name) {
		return ErrNameInvalid
	}
	return nil
}
