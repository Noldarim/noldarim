// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package protocol

import "github.com/noldarim/noldarim/internal/common"

// Re-export common types for backward compatibility.
// New code should import from common directly.
type Metadata = common.Metadata

// Event is re-exported from common for backward compatibility.
type Event = common.Event

// CurrentProtocolVersion is re-exported from common.
const CurrentProtocolVersion = common.CurrentProtocolVersion
