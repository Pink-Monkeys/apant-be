# APANT BE API (Markdown)

Base URL: http://localhost:8000

Auth:

- Bearer token via `Authorization: Bearer <token>` for protected endpoints.

## Endpoints

### Public

- `GET /api/v1/health` - health check
- `GET /api/v1/providers` - list AI providers
- `POST /api/v1/auth/register` - register user
- `POST /api/v1/auth/login` - login
- `POST /api/v1/auth/refresh-token` - refresh token

### Protected

- `POST /api/v1/auth/logout` - logout
- `POST /api/v1/chat` - chat with AI provider
- `POST /api/v1/agent/chat` - agent chat (may return tool intent)
- `POST /api/v1/agent/execute` - execute tool intent
- `POST /api/v1/agent/loop` - plan, execute, finalize
- `GET /api/v1/tools` - list tools
- `POST /api/v1/sessions` - create session
- `GET /api/v1/sessions` - list sessions
- `GET /api/v1/sessions/{id}` - get session by ID

## Request Bodies

### RegisterRequest

```json
{
  "username": "string",
  "password": "string"
}
```

### LoginRequest

```json
{
  "username": "string",
  "password": "string"
}
```

### RefreshTokenRequest

```json
{
  "refresh_token": "string"
}
```

### LogoutRequest (optional)

```json
{
  "refresh_token": "string"
}
```

### ChatRequest

```json
{
  "provider": "openai|claude",
  "model": "string",
  "message": "string",
  "system": "string"
}
```

### AgentChatRequest

```json
{
  "session_id": "string",
  "provider": "openai|claude",
  "model": "string",
  "message": "string",
  "system": "string"
}
```

### ToolIntent

```json
{
  "name": "nmap_scan",
  "params": {
    "target": "scanme.nmap.org",
    "top_ports": 100,
    "service_detection": false
  }
}
```

### AgentExecuteRequest

```json
{
  "tool_intent": {
    "name": "nmap_scan",
    "params": {
      "target": "scanme.nmap.org",
      "top_ports": 100,
      "service_detection": false
    }
  }
}
```

## Response Envelope

Success response:

```json
{
  "success": true,
  "message": "string",
  "data": {}
}
```

Error response:

```json
{
  "success": false,
  "error": {
    "message": "string"
  }
}
```

## Notes

- Full OpenAPI YAML is available in `docs/openapi.yaml`.
- The API returns JSON for all endpoints.
