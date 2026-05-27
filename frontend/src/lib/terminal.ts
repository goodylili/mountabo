// Terminal page fetchers. These call the thin Next proxy routes under
// /api/terminal/* and /api/ai/*, which forward to the Go backend (the backend is
// the only thing that holds SSH keys and the Anthropic key). The shapes mirror
// the backend JSON exactly.

// Result of running one command on a server over SSH.
export type ExecResult = {
  output: string;
  exitCode: number;
  // True when the command produced more output than the captured cap.
  truncated: boolean;
};

// Result of asking the AI helper for a command suggestion. configured is false
// when ANTHROPIC_API_KEY is not set on the backend, in which case explanation
// carries the hint to set it and command is empty.
export type AICommandResult = {
  configured: boolean;
  command: string;
  explanation: string;
};

// runCommand sends a single command to the server and returns its output and
// exit code. A non-zero exit code is a normal result, not an error: the command
// ran and the server answered. It throws only when the request itself fails
// (backend unreachable, server not set up, etc.) with a readable message.
export async function runCommand(serverId: string, command: string): Promise<ExecResult> {
  const resp = await fetch(`/api/terminal/${serverId}/exec`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ command }),
  });
  if (!resp.ok) {
    const msg = await errorMessage(resp);
    throw new Error(msg);
  }
  return (await resp.json()) as ExecResult;
}

// suggestCommand asks the AI helper for a shell command for a plain-English
// request. It never throws on a missing API key: the backend returns
// configured=false so the UI can show a hint. It throws only on a transport or
// backend failure.
export async function suggestCommand(prompt: string, context?: string): Promise<AICommandResult> {
  const resp = await fetch(`/api/ai/command`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ prompt, context: context ?? "" }),
  });
  if (!resp.ok) {
    const msg = await errorMessage(resp);
    throw new Error(msg);
  }
  return (await resp.json()) as AICommandResult;
}

// errorMessage pulls the backend's {error} field from a failed response, falling
// back to a generic message so the UI always has something readable to show.
async function errorMessage(resp: Response): Promise<string> {
  try {
    const body = (await resp.json()) as { error?: string };
    if (body.error) return body.error;
  } catch {
    // not JSON, fall through
  }
  return `request failed with status ${resp.status}`;
}
