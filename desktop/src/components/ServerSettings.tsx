type Props = {
  serverUrl: string;
  onServerUrlChange: (value: string) => void;
  onConnect: () => void;
  isConnecting: boolean;
  connectionError: string | null;
};

export function ServerSettings({
  serverUrl,
  onServerUrlChange,
  onConnect,
  isConnecting,
  connectionError
}: Props) {
  return (
    <section className="panel server-settings">
      <h2>Server</h2>
      <div className="server-controls">
        <input
          aria-label="Server URL"
          value={serverUrl}
          onChange={(event) => onServerUrlChange(event.target.value)}
          placeholder="http://127.0.0.1:8080"
        />
        <button type="button" onClick={onConnect} disabled={isConnecting || !serverUrl.trim()}>
          {isConnecting ? "Connecting..." : "Connect"}
        </button>
      </div>
      {connectionError && <p className="error-text">{connectionError}</p>}
    </section>
  );
}
