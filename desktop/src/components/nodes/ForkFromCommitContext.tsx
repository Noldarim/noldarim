// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { createContext, useContext } from "react";

const ForkFromCommitContext = createContext<((sha: string) => void) | null>(null);

export const ForkFromCommitProvider = ForkFromCommitContext.Provider;

export function useForkFromCommit(): ((sha: string) => void) | null {
  return useContext(ForkFromCommitContext);
}
