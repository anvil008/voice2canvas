# Security

## Browser-provided Gemini keys

The reference frontend keeps a pasted Gemini API key only in React memory. It
does not write the key to `localStorage`, `sessionStorage`, cookies, URLs, or
logs. The key is sent once in the WebSocket `start` message and the backend uses
it to construct model clients for that connection.

The Go backend does not persist or intentionally log the key. It remains in
process memory for the life of the associated model clients and becomes
eligible for collection after the session stops or disconnects.

This protects against accidental persistence, not a malicious or compromised
server. Use the browser-key flow only with a backend you control. Production
deployments should use server-managed credentials, TLS, authentication,
rate-limiting, origin restrictions, and a secrets manager.

## Reporting a vulnerability

Please open a private GitHub security advisory for the repository rather than a
public issue. Do not include active credentials in reports, screenshots, test
fixtures, or logs.
