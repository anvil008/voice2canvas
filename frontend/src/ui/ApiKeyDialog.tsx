import { useEffect, useRef, useState, type FormEvent } from "react";

interface ApiKeyDialogProps {
  onCancel: () => void;
  onSave: (apiKey: string) => void;
}

export function ApiKeyDialog({ onCancel, onSave }: ApiKeyDialogProps) {
  const [apiKey, setApiKey] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onCancel();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onCancel]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const value = apiKey.trim();
    if (value) onSave(value);
  };

  const handleTryDemo = () => {
    window.location.search = "?mock=1";
  };

  return (
    <div className="api-key-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onCancel();
    }}>
      <section aria-labelledby="api-key-title" aria-modal="true" className="api-key-dialog" role="dialog">
        <div className="api-key-kicker">Bring your own model</div>
        <h2 id="api-key-title">Connect Gemini</h2>
        <p>
          Paste a Gemini API key from Google AI Studio. Voice2Canvas keeps it in this tab&apos;s
          memory, sends it only to your backend for the current session, and clears it on reload.
        </p>
        <form onSubmit={submit}>
          <label htmlFor="gemini-api-key">Gemini API key</label>
          <input
            autoComplete="off"
            id="gemini-api-key"
            name="gemini-api-key"
            onChange={(event) => setApiKey(event.target.value)}
            placeholder="AIza…"
            ref={inputRef}
            spellCheck={false}
            type="password"
            value={apiKey}
          />
          <div className="api-key-actions">
            <button className="api-key-demo" onClick={handleTryDemo} type="button">
              Try Demo Mode
            </button>
            <a href="https://aistudio.google.com/app/apikey" rel="noreferrer" target="_blank">
              Get a key
            </a>
            <button className="api-key-cancel" onClick={onCancel} type="button">Cancel</button>
            <button className="api-key-save" disabled={!apiKey.trim()} type="submit">Use this key</button>
          </div>
        </form>
      </section>
    </div>
  );
}
