// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
	"github.com/noldarim/noldarim/internal/config"
)

// Manager manages multiple loggers for different packages
type Manager struct {
	config         *config.LogConfig
	globalLogger   zerolog.Logger
	packageLoggers map[string]zerolog.Logger
	mu             sync.RWMutex
	writers        []io.Writer
}

// NewManager creates a new logger manager
func NewManager(cfg *config.LogConfig) (*Manager, error) {
	m := &Manager{
		config:         cfg,
		packageLoggers: make(map[string]zerolog.Logger),
		writers:        make([]io.Writer, 0),
	}

	// Set global log level
	globalLevel := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(globalLevel)

	// Configure time format
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Create writers based on output configuration
	writers, err := m.createWriters(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create log writers: %w", err)
	}
	m.writers = writers

	// Create multi-writer
	var multiWriter io.Writer
	if len(writers) == 1 {
		multiWriter = writers[0]
	} else if len(writers) > 1 {
		multiWriter = io.MultiWriter(writers...)
	} else {
		// If no writers configured, fall back to a default file writer to ensure logs aren't lost
		defaultPath := "./logs/noldarim-fallback.log"
		if err := os.MkdirAll(filepath.Dir(defaultPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create fallback log directory: %w", err)
		}
		file, err := os.OpenFile(defaultPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to create fallback log file: %w", err)
		}
		m.writers = append(m.writers, file)
		multiWriter = file
	}

	// Configure the global logger
	m.globalLogger = m.createLogger(multiWriter, globalLevel)

	// Do not override the default logger to avoid affecting other libraries
	// Each package should explicitly get its logger via GetLogger()

	return m, nil
}

// createWriters creates all configured output writers
func (m *Manager) createWriters(cfg *config.LogConfig) ([]io.Writer, error) {
	var writers []io.Writer

	for _, output := range cfg.Output {
		if !output.Enabled {
			continue
		}

		switch output.Type {
		case "console":
			var w io.Writer
			if cfg.Format == "console" {
				// Colored console output
				w = zerolog.ConsoleWriter{
					Out:        os.Stderr,
					TimeFormat: "15:04:05.000",
					FormatLevel: func(i interface{}) string {
						return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
					},
					FormatFieldName: func(i interface{}) string {
						return fmt.Sprintf("%s:", i)
					},
					FormatFieldValue: func(i interface{}) string {
						return fmt.Sprintf("%s", i)
					},
					NoColor: false,
				}
			} else {
				w = os.Stderr
			}
			writers = append(writers, w)

		case "file":
			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(output.Path), 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}

			if output.Rotate.MaxSizeMB > 0 {
				// Use lumberjack for rotation
				w := &lumberjack.Logger{
					Filename:   output.Path,
					MaxSize:    output.Rotate.MaxSizeMB,
					MaxBackups: output.Rotate.MaxBackups,
					MaxAge:     output.Rotate.MaxAgeDays,
					Compress:   output.Rotate.Compress,
				}
				m.writers = append(m.writers, w)
				writers = append(writers, w)
			} else {
				// Simple file writer
				file, err := os.OpenFile(output.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
				if err != nil {
					return nil, fmt.Errorf("failed to open log file %s: %w", output.Path, err)
				}
				m.writers = append(m.writers, file)
				writers = append(writers, file)
			}

		default:
			return nil, fmt.Errorf("unsupported output type: %s", output.Type)
		}
	}

	// If file outputs are configured and format is console, wrap them with console writer
	if cfg.Format == "console" && len(writers) > 0 {
		var enhancedWriters []io.Writer
		for i, w := range writers {
			// Only wrap file outputs with console writer
			if i < len(cfg.Output) && cfg.Output[i].Type == "file" {
				enhancedWriters = append(enhancedWriters, zerolog.ConsoleWriter{
					Out:        w,
					TimeFormat: "2006-01-02 15:04:05.000",
					NoColor:    false, // Keep colors in file for readability
					FormatLevel: func(i interface{}) string {
						return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
					},
				})
			} else {
				enhancedWriters = append(enhancedWriters, w)
			}
		}
		writers = enhancedWriters
	}

	return writers, nil
}

// createLogger creates a configured zerolog logger
func (m *Manager) createLogger(w io.Writer, level zerolog.Level) zerolog.Logger {
	ctx := zerolog.New(w).Level(level)

	// Add timestamp if configured
	if m.config.Context.IncludeTimestamp {
		ctx = ctx.With().Timestamp().Logger()
	}

	// Add caller if configured
	if m.config.Context.IncludeCaller {
		ctx = ctx.With().Caller().Logger()
	}

	// Configure stack trace
	if m.config.Context.IncludeStackTrace != "" {
		ctx = ctx.With().Stack().Logger()
		// Note: Stack trace will be included based on the log level
	}

	// Add sampling if configured
	if m.config.Sampling.Enabled {
		sampler := &zerolog.BurstSampler{
			Burst:       m.config.Sampling.Initial,
			Period:      m.config.Sampling.Tick,
			NextSampler: &zerolog.BasicSampler{N: m.config.Sampling.Thereafter},
		}
		ctx = ctx.Sample(sampler)
	}

	return ctx
}

// GetLogger returns a logger for a specific package
func (m *Manager) GetLogger(pkg string) zerolog.Logger {
	m.mu.RLock()
	if logger, exists := m.packageLoggers[pkg]; exists {
		m.mu.RUnlock()
		return logger
	}
	m.mu.RUnlock()

	// Create new logger for package
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check again in case it was created while waiting for lock
	if logger, exists := m.packageLoggers[pkg]; exists {
		return logger
	}

	// Determine level for this package
	level := parseLevel(m.config.Level) // Default to global level
	if pkgLevel, exists := m.config.Levels[pkg]; exists {
		level = parseLevel(pkgLevel)
	}

	// Create package-specific logger with package field
	logger := m.globalLogger.With().Str("pkg", pkg).Logger().Level(level)
	m.packageLoggers[pkg] = logger

	return logger
}

// SetPackageLevel dynamically sets the log level for a package
func (m *Manager) SetPackageLevel(pkg string, level string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	parsedLevel := parseLevel(level)

	// Update config
	if m.config.Levels == nil {
		m.config.Levels = make(map[string]string)
	}
	m.config.Levels[pkg] = level

	// Update or create logger
	if logger, exists := m.packageLoggers[pkg]; exists {
		m.packageLoggers[pkg] = logger.Level(parsedLevel)
	}
}

// Close closes all file writers
func (m *Manager) Close() error {
	for _, w := range m.writers {
		if closer, ok := w.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseLevel converts string level to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch strings.ToUpper(level) {
	case "TRACE":
		return zerolog.TraceLevel
	case "DEBUG":
		return zerolog.DebugLevel
	case "INFO":
		return zerolog.InfoLevel
	case "WARN", "WARNING":
		return zerolog.WarnLevel
	case "ERROR":
		return zerolog.ErrorLevel
	case "FATAL":
		return zerolog.FatalLevel
	case "PANIC":
		return zerolog.PanicLevel
	default:
		return zerolog.InfoLevel
	}
}

// Global manager instance
var globalManager *Manager
var once sync.Once

// Initialize initializes the global logger manager
func Initialize(cfg *config.LogConfig) error {
	var err error
	once.Do(func() {
		globalManager, err = NewManager(cfg)
	})
	return err
}

// GetLogger returns a logger for the specified package
func GetLogger(pkg string) zerolog.Logger {
	if globalManager == nil {
		// Return a discard logger if not initialized to avoid stdout/stderr pollution
		return zerolog.New(io.Discard).With().Timestamp().Logger()
	}
	return globalManager.GetLogger(pkg)
}

// Close closes the global logger manager
func CloseGlobal() error {
	if globalManager != nil {
		return globalManager.Close()
	}
	return nil
}
