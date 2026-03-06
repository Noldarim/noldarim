// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useMemo } from "react";

import { highestSeverity, scanAttentionFlags } from "../../lib/attention-flags";
import type { AIActivityRecord } from "../../lib/types";

type Props = {
  activities: AIActivityRecord[];
  onClick?: () => void;
};

export function AttentionBadge({ activities, onClick }: Props) {
  const flags = useMemo(() => scanAttentionFlags(activities), [activities]);
  const severity = highestSeverity(flags);

  if (flags.length === 0 || !severity) return null;

  const label = `${flags.length} item${flags.length > 1 ? "s" : ""} need attention`;

  if (onClick) {
    return (
      <button
        type="button"
        className={`attention-badge attention-badge--${severity}`}
        onClick={onClick}
        title={label}
      >
        {flags.length}
      </button>
    );
  }

  return (
    <span
      className={`attention-badge attention-badge--${severity}`}
      title={label}
    >
      {flags.length}
    </span>
  );
}
