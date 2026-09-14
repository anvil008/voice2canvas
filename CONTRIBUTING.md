# Contributing to Voice2Canvas

Thank you for your interest in contributing to Voice2Canvas! We welcome bug reports, feature suggestions, documentation improvements, and code contributions.

## Development Workflow

Voice2Canvas consists of a Go backend and a React/TypeScript frontend.

### Prerequisites

- **Go**: 1.26 or later
- **Node.js**: 22 or later (with npm)
- **Git**

### Fork & Branch

1. Fork the repository on GitHub.
2. Clone your fork locally:
   ```bash
   git clone https://github.com/<your-username>/voice2canvas.git
   cd voice2canvas
   ```
3. Create a feature branch for your changes:
   ```bash
   git checkout -b my-feature
   ```

### Running Locally

1. **Frontend development**:
   ```bash
   cd frontend
   npm ci
   npm run dev
   ```

2. **Backend development**:
   In another terminal:
   ```bash
   cd backend
   go run ./cmd/server
   ```

### Running Checks & Tests

Before opening a pull request, ensure all tests and linting pass locally:

**Frontend**:
```bash
cd frontend
npm test
npm run typecheck
npm run build
```

**Backend**:
```bash
cd backend
go test ./...
go vet ./...
go build ./cmd/server
```

## Guidelines

- **Protocol & Schema changes**: If modifying protocol messages or the extended catalog, ensure changes are reflected in `PROTOCOL.md`, `catalog/EXTENDED_CATALOG.md`, and corresponding JSON schemas in `catalog/` and `backend/internal/a2ui/schema/`.
- **Tests**: Include tests for both accepted and rejected behaviors (e.g., valid frames, malformed inputs, edge cases).
- **Credentials & Privacy**: Never commit API keys, environment credentials, or recordings of user speech.
- **Code style**: Use standard Go conventions (`gofmt`, `go vet`) and TypeScript tooling.

## Submitting a Pull Request

1. Push your branch to your GitHub fork:
   ```bash
   git push origin my-feature
   ```
2. Open a Pull Request against the `main` branch.
3. Provide a clear summary of your changes, motivation, and any testing performed.
