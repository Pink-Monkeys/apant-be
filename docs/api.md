# API Endpoints (Non-Auth)

Base URL: http://localhost:8000

## Public

- `GET /api/v1/health` - health check
- `GET /api/v1/providers` - list AI providers

## Protected

- `POST /api/v1/chat` - chat with AI provider
- `POST /api/v1/agent/chat` - agent chat (may return tool intent)
- `POST /api/v1/agent/execute` - execute tool intent
- `POST /api/v1/agent/loop` - plan, execute, finalize
- `GET /api/v1/tools` - list tools
- `POST /api/v1/sessions` - create session
- `GET /api/v1/sessions` - list sessions
- `GET /api/v1/sessions/{id}` - get session by ID
