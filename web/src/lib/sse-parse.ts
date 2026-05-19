/**
 * Incremental SSE (Server-Sent Events) parser for blocks terminated by "\n\n".
 * Calls onData with the reconstructed `data:` payload (joined lines) per event.
 * Comment lines (`:` ...) and heartbeats produce no callback.
 */
export function feedSSE(
  chunk: string,
  buffer: string,
  onData: (payload: string) => void,
): string {
  // Normalize CRLF / bare CR to LF per SSE spec.
  let buf = buffer + chunk.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  for (;;) {
    const idx = buf.indexOf("\n\n");
    if (idx === -1) {
      return buf;
    }
    const block = buf.slice(0, idx);
    buf = buf.slice(idx + 2);
    const payload = dataPayloadFromSSEBlock(block);
    if (payload !== null) {
      onData(payload);
    }
  }
}

function dataPayloadFromSSEBlock(block: string): string | null {
  const lines = block.split("\n");
  const parts: string[] = [];
  for (const line of lines) {
    if (line.startsWith("data:")) {
      const val = line.slice(5);
      parts.push(val.startsWith(" ") ? val.slice(1) : val);
    }
  }
  if (parts.length === 0) {
    return null;
  }
  return parts.join("\n");
}
