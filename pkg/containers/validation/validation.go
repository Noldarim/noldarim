// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package validation

import (
	"fmt"
	"regexp"
	"strings"
)

// validLabelKeyRegex matches valid Docker label keys
// Docker label keys should follow DNS subdomain format
var validLabelKeyRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?(\.[a-z0-9]([a-z0-9\-\.]*[a-z0-9])?)*$`)

// validEnvVarNameRegex matches valid environment variable names
// Environment variable names should be alphanumeric with underscores
var validEnvVarNameRegex = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return fmt.Sprintf("multiple validation errors: %s", strings.Join(messages, "; "))
}

// ValidateContainerLabels validates a map of container labels
func ValidateContainerLabels(labels map[string]string) error {
	var errors ValidationErrors

	for key, value := range labels {
		// Validate key format
		if !validLabelKeyRegex.MatchString(key) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("label key '%s'", key),
				Message: "must be a valid DNS subdomain (lowercase letters, numbers, dots, and hyphens only)",
			})
			continue
		}

		// Check key length (Docker limit is 63 characters per segment)
		segments := strings.Split(key, ".")
		for _, segment := range segments {
			if len(segment) > 63 {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("label key '%s'", key),
					Message: "segment exceeds 63 character limit",
				})
				break
			}
		}

		// Validate value (basic safety checks)
		if err := validateStringValue(value, fmt.Sprintf("label value for key '%s'", key)); err != nil {
			errors = append(errors, *err)
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// ValidateEnvironmentVariables validates a map of environment variables
func ValidateEnvironmentVariables(env map[string]string) error {
	var errors ValidationErrors

	for name, value := range env {
		// Validate environment variable name
		if !validEnvVarNameRegex.MatchString(name) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("environment variable '%s'", name),
				Message: "must start with a letter or underscore and contain only uppercase letters, numbers, and underscores",
			})
			continue
		}

		// Check for reserved names
		if isReservedEnvVar(name) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("environment variable '%s'", name),
				Message: "is a reserved environment variable name",
			})
			continue
		}

		// Validate value
		if err := validateStringValue(value, fmt.Sprintf("environment variable value for '%s'", name)); err != nil {
			errors = append(errors, *err)
		}
	}

	if len(errors) > 0 {
		return errors
	}
	return nil
}

// validateStringValue performs common string validation
func validateStringValue(value, fieldName string) *ValidationError {
	// Check for null bytes (security issue)
	if strings.Contains(value, "\x00") {
		return &ValidationError{
			Field:   fieldName,
			Message: "contains null bytes",
		}
	}

	// Check for control characters (except common whitespace)
	for _, r := range value {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, LF, CR
			return &ValidationError{
				Field:   fieldName,
				Message: "contains control characters",
			}
		}
	}

	// Check maximum length (reasonable limit)
	if len(value) > 4096 {
		return &ValidationError{
			Field:   fieldName,
			Message: "exceeds maximum length of 4096 characters",
		}
	}

	return nil
}

// isReservedEnvVar checks if an environment variable name is reserved
func isReservedEnvVar(name string) bool {
	reserved := map[string]bool{
		"PATH":     true,
		"HOME":     true,
		"USER":     true,
		"SHELL":    true,
		"PWD":      true,
		"OLDPWD":   true,
		"HOSTNAME": true,
		"IFS":      true,
		"PS1":      true,
		"PS2":      true,
		"TERM":     true,
		"LANG":     true,
		"LC_ALL":   true,
		"TZ":       true,
	}
	return reserved[name]
}
