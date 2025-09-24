# Agents.md

This file lays out conventions, setup steps, and guidelines for AI agents (Codex, etc.) working on **prism-plan** so things stay predictable, clean, and ready for real deployment.

## 📚 Project at a Glance

- **Name:** Prism Plan  
- **Type:** Task & Time Management App (pet / experimental)  
- **Tech Stack:**  
  - Backend: .NET 9 (Domain Service / C#), Go (Read-Model Updater)  
  - API: Go + Azure Functions  
  - Frontend: JS/TS (likely some SPA framework)  
  - Infrastructure: Docker Compose, Azure storage / functions, Auth0 for auth  
  - Pattern: Event-Sourcing, Microservices / CQRS (commands + read models)  

- **Important Structure:**  
  - /frontend → UI / client side
  - /prism-api → API layer (commands & queries)
  - /domain-service → C# domain logic for commands / domain events
  - /read-model-updater → Go service projecting events to read models
  - /storage-init → bootstrap / provisioning storage / tables / queues
  - /scripts → helper utilities / deployment scripts
  - /tests → integration / performance / other tests
  - /docker-compose.yml → local dev orchestration

## ⚙️ Dev / Build & Run Commands

Use these to set things up and run locally:

- Install required SDKs: .NET 9, Go, Node.js  
- Setup local environment: copy `.env.example` → `.env` and fill in env vars (Auth0 config, storage connection strings, queue/table names etc.)  
- Generate certificates if needed: `scripts/generate-cert.sh` (or `.bat` on Windows)  
- To spin up local dev:  `docker-compose up --build`
- API server: Depends on API location from VITE_API_BASE_URL in frontend/.env if frontend is served separately  
- Frontend dev: `npm run dev` (or whatever the script is in package.json)  
- Build frontend for production: e.g. `npm run build` in /frontend

## 🧪 Tests

- Run all tests: navigate to `/tests` and use whatever test runner is set up (likely some combination: Go + .NET + frontend).  
- Run single test/unit test: e.g. `cd tests && <test-command> tests/.../file_name`  
- Integration/performance tests: see `/tests/README.md` for specific commands.  
- Test coverage / target: none explicitly stated, but ensure that critical paths (command handling, read model correctness) are covered.

## 📐 Code Style & Conventions

- Formatting / linting: stick with the conventions already in repo (e.g. for C#: whatever our .editorconfig / `.csproj` rules; for Go: gofmt / go vet; for JS/TS: ESLint / prettier if set up)  
- Naming:  
  - Files/modules: lower-case kebab or snake case (as seen)  
  - Types / classes: PascalCase in C#; Go style for Go (e.g. CamelCase for structs / methods)  
  - Variables: follow language idioms (e.g. `camelCase` in JS/TS)  
- Layout: keep services (domain, read model) separated; only shared code goes to shared folder or scripts  
- Comments / docs: when writing or modifying key logic (events, projections, domain logic), leave comments/descriptions especially around edge cases or async/event sourcing patterns  

## 🔄 Commit / PR Guidelines

- Branch names: `feature/<short-name>`, `bugfix/<short-name>`, `refactor/...` etc.  
- Commit messages: use prefixes like `feat:`, `fix:`, `chore:`, `docs:`  
- Before merging:  
- All relevant tests must pass  
- Lint / format checks passed  
- Code compiles in CI (where applicable)  
- No breaking of existing functionality unless intentional & documented  
- PR description: what changed, why, any migration or infra changes needed  

## 🛠 Dev Environment Tips & Gotchas

- `.env` is required for running locally; do **not** commit secrets / real Auth0 credentials. Use placeholder or free-tier values for local dev.  
- For Docker Compose: ensure storage queues & tables provisioned (via `storage-init`) before other services start.  
- Dev SSL / certificate generation step is required (for frontend over HTTPS) — watch out for OS differences.  
- When updating the event sourcing parts (e.g. new events), ensure corresponding projections/read model logic is updated.  

## 🗺 Architecture & Module Layout

- **Command side (write model):** Fronted via Prism API, commands processed by Domain Service, events emitted onto queues etc.  
- **Read model side:** Read-model-updater picks up domain events and projects them into tables for queries.  
- **Frontend / API communication:** Frontend talks to `/api` endpoints; Auth0 handles authentication/authorization.

## 🔐 Security & Performance Notes

- Do not leak secrets (Auth0, storage connection strings) in code or commits.  
- Validate JWTs properly (Auth0 setup) in both API & domain service.  
- Error handling: watch for event sourcing edge cases (e.g. event ordering, equal timestamps, retries). Consider circuit breakers if integrating external dependencies.  
- Performance: read model updates should be idempotent & fast; for Azure Functions / HTTP endpoints, aim for low cold start; cache or throttle where needed.  

## ✅ What Success Looks Like
When an agent (or you) finishes a task or PR, check:
- All new + existing tests pass (unit, integration)  
- No lint/format/auth issues  
- API & read model behave as expected (manual / automated tests)  
- Changes documented (README or comments) for major modules or behaviors  
- No regressions in performance or error rates (where measurable)  
- Deployment scripts / infra changes validated locally
- Last but not least, avoid introducing new packages, modules and libraries and strive to implement things via existing ones (E.g. when you need to implement gzip in golang API written with echo - you must use echo-native gzip middleware instead of building your own!!!)