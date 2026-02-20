// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

type DevNavToggleProps = {
  isSandbox: boolean;
};

export function DevNavToggle({ isSandbox }: DevNavToggleProps) {
  const toggle = () => {
    window.location.hash = isSandbox ? "" : "#sandbox";
  };

  return (
    <button type="button" className="sandbox-nav-toggle" onClick={toggle}>
      {isSandbox ? "Exit Sandbox" : "Sandbox"}
    </button>
  );
}
