// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/log"
)

// TemporalLogAdapter adapts zerolog to Temporal's logger interface
type TemporalLogAdapter struct {
	logger zerolog.Logger
}

// NewTemporalLogAdapter creates a new Temporal log adapter
func NewTemporalLogAdapter(logger zerolog.Logger) log.Logger {
	return &TemporalLogAdapter{
		logger: logger,
	}
}

// Debug logs at debug level
func (t *TemporalLogAdapter) Debug(msg string, keyvals ...interface{}) {
	event := t.logger.Debug()
	t.addFields(event, keyvals...).Msg(msg)
}

// Info logs at info level
func (t *TemporalLogAdapter) Info(msg string, keyvals ...interface{}) {
	event := t.logger.Info()
	t.addFields(event, keyvals...).Msg(msg)
}

// Warn logs at warn level
func (t *TemporalLogAdapter) Warn(msg string, keyvals ...interface{}) {
	event := t.logger.Warn()
	t.addFields(event, keyvals...).Msg(msg)
}

// Error logs at error level
func (t *TemporalLogAdapter) Error(msg string, keyvals ...interface{}) {
	event := t.logger.Error()
	t.addFields(event, keyvals...).Msg(msg)
}

// With returns a new logger with additional fields
func (t *TemporalLogAdapter) With(keyvals ...interface{}) log.Logger {
	newLogger := t.logger.With().Logger()

	// Add fields to the new logger
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := fmt.Sprint(keyvals[i])
			value := keyvals[i+1]
			newLogger = newLogger.With().Interface(key, value).Logger()
		}
	}

	return &TemporalLogAdapter{
		logger: newLogger,
	}
}

// addFields adds key-value pairs to a zerolog event
func (t *TemporalLogAdapter) addFields(event *zerolog.Event, keyvals ...interface{}) *zerolog.Event {
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			key := fmt.Sprint(keyvals[i])
			value := keyvals[i+1]

			// Handle different types appropriately
			switch v := value.(type) {
			case string:
				event = event.Str(key, v)
			case int:
				event = event.Int(key, v)
			case int64:
				event = event.Int64(key, v)
			case float64:
				event = event.Float64(key, v)
			case bool:
				event = event.Bool(key, v)
			case error:
				event = event.Err(v)
			case fmt.Stringer:
				event = event.Str(key, v.String())
			default:
				event = event.Interface(key, v)
			}
		}
	}
	return event
}

// GetTemporalLogAdapter returns a Temporal logger adapter for the given package
func GetTemporalLogAdapter(pkg string) log.Logger {
	zerologLogger := GetLogger(pkg)
	return NewTemporalLogAdapter(zerologLogger)
}
