// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package providers

import (
	"fmt"

	"github.com/noldarim/noldarim/pkg/runtime"
	"github.com/noldarim/noldarim/pkg/runtime/local"
)

// NewProvider creates a RuntimeProvider based on the provider name.
func NewProvider(name string, dockerHost string) (runtime.Provider, error) {
	switch name {
	case runtime.ProviderLocal, "":
		return local.New(dockerHost, nil), nil
	default:
		return nil, fmt.Errorf("unknown runtime provider: %q", name)
	}
}
