// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { createContext, useContext, useState, type ReactNode } from "react";

type PatchExpandContextValue = {
  expandedPatchId: string | null;
  setExpandedPatchId: (id: string | null) => void;
};

const PatchExpandContext = createContext<PatchExpandContextValue>({
  expandedPatchId: null,
  setExpandedPatchId: () => {}
});

export function PatchExpandProvider({ children }: { children: ReactNode }) {
  const [expandedPatchId, setExpandedPatchId] = useState<string | null>(null);
  return (
    <PatchExpandContext.Provider value={{ expandedPatchId, setExpandedPatchId }}>
      {children}
    </PatchExpandContext.Provider>
  );
}

export function usePatchExpand(): PatchExpandContextValue {
  return useContext(PatchExpandContext);
}
