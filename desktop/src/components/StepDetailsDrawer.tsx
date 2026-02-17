import type { AIActivityRecord, StepDraft, StepResult } from "../lib/types";
import { groupToolEventsByName } from "../lib/obs-mapping";
import { StepResultSummary, AgentOutput, GitDiffView, ToolActivityPanel, EventTimeline } from "./step-detail";

type Props = {
  step: StepDraft | null;
  result: StepResult | null;
  events: AIActivityRecord[];
  isOpen: boolean;
  onClose: () => void;
};

export function StepDetailsDrawer({ step, result, events, isOpen, onClose }: Props) {
  if (!isOpen || !step) {
    return null;
  }

  const toolGroups = groupToolEventsByName(events);

  return (
    <>
      <div className="details-drawer-backdrop" onClick={onClose} />
      <aside className="details-drawer" role="complementary" aria-label="Step details">
        <header className="details-drawer__header">
          <div>
            <h3>{step.name}</h3>
            <p className="muted-text">{step.id}</p>
          </div>
          <button type="button" onClick={onClose}>
            Close
          </button>
        </header>

        {result && <StepResultSummary result={result} />}
        {result?.agent_output && <AgentOutput output={result.agent_output} />}
        {result?.git_diff && <GitDiffView diff={result.git_diff} />}
        <ToolActivityPanel groups={toolGroups} />
        <EventTimeline events={events} />
      </aside>
    </>
  );
}
