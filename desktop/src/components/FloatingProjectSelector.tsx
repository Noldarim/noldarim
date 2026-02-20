// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import type { Project } from "../lib/types";

type Props = {
  projects: Project[];
  selectedProjectId: string;
  onSelectProject: (id: string) => void;
  disabled?: boolean;
};

export function FloatingProjectSelector({ projects, selectedProjectId, onSelectProject, disabled }: Props) {
  return (
    <div className="floating-project-selector">
      <select
        value={selectedProjectId}
        onChange={(e) => onSelectProject(e.target.value)}
        disabled={disabled || projects.length === 0}
      >
        {projects.length === 0 && <option value="">No projects</option>}
        {projects.map((p) => (
          <option key={p.id} value={p.id}>
            {p.name}
          </option>
        ))}
      </select>
    </div>
  );
}
