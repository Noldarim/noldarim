// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateContainerLabels(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid labels",
			labels: map[string]string{
				"noldarim.task.id":    "task123",
				"noldarim.project.id": "proj456",
				"app.version":     "1.0.0",
			},
			expectError: false,
		},
		{
			name: "invalid label key with uppercase",
			labels: map[string]string{
				"INVALID.KEY": "value",
			},
			expectError:   true,
			errorContains: "must be a valid DNS subdomain",
		},
		{
			name: "invalid label key with spaces",
			labels: map[string]string{
				"invalid key": "value",
			},
			expectError:   true,
			errorContains: "must be a valid DNS subdomain",
		},
		{
			name: "label value with null byte",
			labels: map[string]string{
				"valid.key": "value\x00with\x00nulls",
			},
			expectError:   true,
			errorContains: "contains null bytes",
		},
		{
			name: "label value with control characters",
			labels: map[string]string{
				"valid.key": "value\x01with\x02control",
			},
			expectError:   true,
			errorContains: "contains control characters",
		},
		{
			name: "label value too long",
			labels: map[string]string{
				"valid.key": strings.Repeat("a", 4097),
			},
			expectError:   true,
			errorContains: "exceeds maximum length",
		},
		{
			name: "label key segment too long",
			labels: map[string]string{
				strings.Repeat("a", 64) + ".key": "value",
			},
			expectError:   true,
			errorContains: "segment exceeds 63 character limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerLabels(tt.labels)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name          string
		env           map[string]string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid environment variables",
			env: map[string]string{
				"TASK_ID":     "task123",
				"PROJECT_ID":  "proj456",
				"MY_VAR":      "value",
				"DEBUG_MODE":  "true",
				"PORT_NUMBER": "8080",
			},
			expectError: false,
		},
		{
			name: "invalid env var name with lowercase",
			env: map[string]string{
				"invalid_name": "value",
			},
			expectError:   true,
			errorContains: "must start with a letter or underscore",
		},
		{
			name: "invalid env var name with spaces",
			env: map[string]string{
				"INVALID NAME": "value",
			},
			expectError:   true,
			errorContains: "must start with a letter or underscore",
		},
		{
			name: "invalid env var name starting with number",
			env: map[string]string{
				"123INVALID": "value",
			},
			expectError:   true,
			errorContains: "must start with a letter or underscore",
		},
		{
			name: "reserved environment variable",
			env: map[string]string{
				"PATH": "/usr/bin",
			},
			expectError:   true,
			errorContains: "is a reserved environment variable name",
		},
		{
			name: "env var value with null byte",
			env: map[string]string{
				"VALID_NAME": "value\x00with\x00nulls",
			},
			expectError:   true,
			errorContains: "contains null bytes",
		},
		{
			name: "env var value with control characters",
			env: map[string]string{
				"VALID_NAME": "value\x01with\x02control",
			},
			expectError:   true,
			errorContains: "contains control characters",
		},
		{
			name: "env var value too long",
			env: map[string]string{
				"VALID_NAME": strings.Repeat("a", 4097),
			},
			expectError:   true,
			errorContains: "exceeds maximum length",
		},
		{
			name: "env var value with allowed whitespace",
			env: map[string]string{
				"VALID_NAME": "value\twith\nallowed\rwhitespace",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnvironmentVariables(tt.env)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationErrors(t *testing.T) {
	t.Run("single validation error", func(t *testing.T) {
		err := ValidationError{Field: "test", Message: "test message"}
		assert.Equal(t, "validation error for test: test message", err.Error())
	})

	t.Run("multiple validation errors", func(t *testing.T) {
		errors := ValidationErrors{
			{Field: "field1", Message: "message1"},
			{Field: "field2", Message: "message2"},
		}
		errStr := errors.Error()
		assert.Contains(t, errStr, "multiple validation errors")
		assert.Contains(t, errStr, "field1: message1")
		assert.Contains(t, errStr, "field2: message2")
	})

	t.Run("empty validation errors", func(t *testing.T) {
		errors := ValidationErrors{}
		assert.Equal(t, "no validation errors", errors.Error())
	})
}
