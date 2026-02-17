type Props = { output: string };

export function AgentOutput({ output }: Props) {
  return (
    <details className="step-output-details">
      <summary>Agent output</summary>
      <pre className="step-output-pre">{output}</pre>
    </details>
  );
}
