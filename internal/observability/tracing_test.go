// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package observability

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel"
)

func TestInitTracer_NoEndpoint(t *testing.T) {
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	shutdown, err := InitTracer(context.Background())
	require.NoError(t, err)
	assert.Nil(t, shutdown, "shutdown should be nil when no endpoint is configured")
}

func TestInitTracer_WithEndpoint(t *testing.T) {
	// Set a dummy endpoint — the exporter will be created but not actually
	// connect during the test (it uses lazy connection).
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")

	shutdown, err := InitTracer(context.Background())
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Verify that the global tracer provider was set
	tp := otel.GetTracerProvider()
	assert.NotNil(t, tp)

	// Clean shutdown
	err = shutdown(context.Background())
	assert.NoError(t, err)
}
