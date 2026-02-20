// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { createPortal } from "react-dom";

type Props = {
  x: number;
  y: number;
  sha: string;
  onRunPipeline: () => void;
  disabled?: boolean;
};

export function CommitContextMenu({ x, y, sha, onRunPipeline, disabled }: Props) {
  return createPortal(
    <div
      className="commit-context-menu"
      style={{ left: x, top: y }}
      role="menu"
    >
      <p className="commit-context-menu__sha">
        <code>{sha.slice(0, 8)}</code>
      </p>
      <button
        type="button"
        role="menuitem"
        className="primary-button"
        onClick={onRunPipeline}
        disabled={disabled}
      >
        Run pipeline from here
      </button>
    </div>,
    document.body
  );
}
