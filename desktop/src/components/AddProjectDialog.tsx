// Copyright (C) 2025-2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { createProject } from "../lib/api";
import { messageFromError } from "../lib/formatting";
import type { Project } from "../lib/types";

async function pickDirectory(): Promise<string | null> {
  try {
    const { open } = await import("@tauri-apps/plugin-dialog");
    const selected = await open({ directory: true, multiple: false, title: "Select repository" });
    return selected ?? null;
  } catch {
    // Not running in Tauri (e.g. browser dev mode) — no native picker available.
    return null;
  }
}

type Props = {
  isOpen: boolean;
  onClose: () => void;
  serverUrl: string;
  onProjectCreated: (project: Project) => void;
};

export function AddProjectDialog({ isOpen, onClose, serverUrl, onProjectCreated }: Props) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);

  const [repoPath, setRepoPath] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    const dialog = dialogRef.current;
    if (!dialog) return;

    if (isOpen && !dialog.open) {
      setRepoPath("");
      setName("");
      setDescription("");
      setError(null);
      dialog.showModal();
      requestAnimationFrame(() => nameRef.current?.focus());
    } else if (!isOpen && dialog.open) {
      dialog.close();
    }
  }, [isOpen]);

  const handleBrowse = useCallback(async () => {
    const path = await pickDirectory();
    if (path) setRepoPath(path);
  }, []);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setError(null);

      const trimmedName = name.trim();
      const trimmedPath = repoPath.trim();

      if (!trimmedName) {
        setError("Project name is required.");
        return;
      }
      if (!trimmedPath) {
        setError("Repository path is required.");
        return;
      }
      if (!trimmedPath.startsWith("/")) {
        setError("Repository path must be an absolute path.");
        return;
      }

      setSubmitting(true);
      try {
        const project = await createProject(serverUrl, {
          name: trimmedName,
          description: description.trim(),
          repository_path: trimmedPath
        });
        toast.success(`Project "${project.name}" created`);
        onProjectCreated(project);
        onClose();
      } catch (err) {
        setError(messageFromError(err));
      } finally {
        setSubmitting(false);
      }
    },
    [name, repoPath, description, serverUrl, onProjectCreated, onClose]
  );

  return (
    <dialog ref={dialogRef} className="modal-dialog add-project-dialog" onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <div className="modal-dialog__header add-project-dialog__header">
          <h2>Add Project</h2>
          <button type="button" onClick={onClose} className="modal-dialog__close">
            Close
          </button>
        </div>

        <div className="add-project-dialog__body">
          <div className="field-group">
            <label htmlFor="add-project-name">Name</label>
            <input
              ref={nameRef}
              id="add-project-name"
              type="text"
              placeholder="My Project"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={255}
              disabled={submitting}
            />
          </div>

          <div className="field-group">
            <label htmlFor="add-project-repo">Repository path</label>
            <div className="add-project-dialog__path-row">
              <input
                id="add-project-repo"
                type="text"
                placeholder="/Users/you/projects/my-repo"
                value={repoPath}
                onChange={(e) => setRepoPath(e.target.value)}
                disabled={submitting}
              />
              <button type="button" onClick={handleBrowse} disabled={submitting}>
                Browse
              </button>
            </div>
          </div>

          <div className="field-group">
            <label htmlFor="add-project-desc">Description <span className="muted-text">(optional)</span></label>
            <textarea
              id="add-project-desc"
              placeholder="What this project is about..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={1000}
              rows={3}
              disabled={submitting}
            />
          </div>

          {error && <p className="error-text">{error}</p>}

          <div className="add-project-dialog__actions">
            <button type="button" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <button type="submit" className="primary-button" disabled={submitting}>
              {submitting ? "Creating..." : "Create Project"}
            </button>
          </div>
        </div>
      </form>
    </dialog>
  );
}
