package common

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ValidationError represents validation failures
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s' with value '%v': %s", e.Field, e.Value, e.Message)
}

// Validator provides validation utilities
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// Field validates a field and collects errors
func (v *Validator) Field(fieldName string, value interface{}, rules ...ValidationRule) *Validator {
	for _, rule := range rules {
		if err := rule(fieldName, value); err != nil {
			v.errors = append(v.errors, *err)
		}
	}
	return v
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// Errors returns all validation errors
func (v *Validator) Errors() []ValidationError {
	return v.errors
}

// Error returns a combined error message
func (v *Validator) Error() error {
	if !v.HasErrors() {
		return nil
	}

	var messages []string
	for _, err := range v.errors {
		messages = append(messages, err.Error())
	}
	return fmt.Errorf(strings.Join(messages, "; "))
}

// ErrorMessage returns a combined error message as string
func (v *Validator) ErrorMessage() string {
	if !v.HasErrors() {
		return ""
	}

	var messages []string
	for _, err := range v.errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// ValidationRule represents a single validation rule
type ValidationRule func(fieldName string, value interface{}) *ValidationError

// Required - Common validation rules
func Required(fieldName string, value interface{}) *ValidationError {
	if value == nil {
		return &ValidationError{Field: fieldName, Value: value, Message: "is required"}
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return &ValidationError{Field: fieldName, Value: value, Message: "is required"}
		}
	case *string:
		if v == nil || strings.TrimSpace(*v) == "" {
			return &ValidationError{Field: fieldName, Value: value, Message: "is required"}
		}
	}
	return nil
}

func MinLength(fieldName string, value interface{}, min int) *ValidationError {
	str, ok := value.(string)
	if !ok {
		if strPtr, ok := value.(*string); ok && strPtr != nil {
			str = *strPtr
		} else {
			return nil
		}
	}

	if utf8.RuneCountInString(str) < min {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: fmt.Sprintf("must be at least %d characters", min),
		}
	}
	return nil
}

func MaxLength(fieldName string, value interface{}, max int) *ValidationError {
	str, ok := value.(string)
	if !ok {
		if strPtr, ok := value.(*string); ok && strPtr != nil {
			str = *strPtr
		} else {
			return nil
		}
	}

	if utf8.RuneCountInString(str) > max {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: fmt.Sprintf("must be at most %d characters", max),
		}
	}
	return nil
}

func UUID(fieldName string, value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{Field: fieldName, Value: value, Message: "must be a string"}
	}

	if _, err := uuid.Parse(str); err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: "must be a valid UUID",
		}
	}
	return nil
}

func CurrencyCode(fieldName string, value interface{}) *ValidationError {
	str, ok := value.(string)
	if !ok {
		return &ValidationError{Field: fieldName, Value: value, Message: "must be a string"}
	}

	// ISO 4217 currency codes are 3 letters
	if len(str) != 3 {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: "must be exactly 3 characters (ISO 4217)",
		}
	}

	// Check if it's all uppercase letters
	currencyRegex := regexp.MustCompile(`^[A-Z]{3}$`)
	if !currencyRegex.MatchString(str) {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Message: "must be 3 uppercase letters (ISO 4217)",
		}
	}

	return nil
}

// ValidateAndReturnError validates and returns InvalidArgumentError if validation fails
func ValidateAndReturnError(validator *Validator) error {
	if validator.HasErrors() {
		return InvalidArgumentError(validator.ErrorMessage())
	}
	return nil
}

func ValidateStruct(s interface{}) error {
	validator := NewValidator()
	// TODO: Implement struct field validation using reflection
	// This would inspect struct tags and apply validation rules
	return validator.Error()
}
