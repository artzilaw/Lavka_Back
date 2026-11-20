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
	ErrTitleRequired             = errors.New("title is required")
	ErrTitleTooShort             = errors.New("title must be at least 3 characters long")
	ErrTitleTooLong              = errors.New("title must be at most 120 characters long")
	ErrDescriptionRequired       = errors.New("description is required")
	ErrDescriptionTooShort       = errors.New("description must be at least 5 characters long")
	ErrDescriptionTooLong        = errors.New("description must be at most 3000 characters long")
	ErrPriceNegative             = errors.New("price cannot be negative")
	ErrInvalidCategory           = errors.New("invalid category")
	ErrLocationRequired          = errors.New("location is required")
	ErrLocationTooShort          = errors.New("location must be at least 2 characters long")
	ErrPhoneInvalid              = errors.New("invalid phone number")
	ErrAddressRequired           = errors.New("delivery address is required")

	reStrict    = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	reHasLetter = regexp.MustCompile(`[a-zA-Z]`)
	reHasDigit  = regexp.MustCompile(`[0-9]`)
	emailRegex  = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	nameRegex   = regexp.MustCompile(`^[\p{L}a-zA-Z0-9_\s'-]+$`)
	phoneRegex  = regexp.MustCompile(`^[+\d][\d\s\-()]{6,20}$`)
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

func ValidateAdTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrTitleRequired
	}
	n := utf8.RuneCountInString(title)
	if n < 3 {
		return ErrTitleTooShort
	}
	if n > 120 {
		return ErrTitleTooLong
	}
	return nil
}

func ValidateAdDescription(desc string) error {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return ErrDescriptionRequired
	}
	n := utf8.RuneCountInString(desc)
	if n < 5 {
		return ErrDescriptionTooShort
	}
	if n > 3000 {
		return ErrDescriptionTooLong
	}
	return nil
}

func ValidateAdPrice(price int64) error {
	if price < 0 {
		return ErrPriceNegative
	}
	return nil
}

func ValidateAdCategory(categoryID int64) error {
	if categoryID < 1 || categoryID > 11 {
		return ErrInvalidCategory
	}
	return nil
}

func ValidateAdLocation(location string) error {
	location = strings.TrimSpace(location)
	if location == "" {
		return ErrLocationRequired
	}
	if utf8.RuneCountInString(location) < 2 {
		return ErrLocationTooShort
	}
	return nil
}

func ValidatePhone(phone string) error {
	phone = strings.TrimSpace(phone)
	if !phoneRegex.MatchString(phone) {
		return ErrPhoneInvalid
	}
	return nil
}

func ValidateDeliveryAddress(address string) error {
	address = strings.TrimSpace(address)
	if utf8.RuneCountInString(address) < 5 {
		return ErrAddressRequired
	}
	return nil
}
