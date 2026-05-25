# Authentication API (Cookie + CSRF)

Base URL: http://localhost:8000

Auth cookies (browser flow):

- `apant_access`: access token (HttpOnly, short-lived)
- `apant_refresh`: refresh token (HttpOnly, long-lived)
- `apant_csrf`: CSRF token (readable by JS; send as `X-CSRF-Token`)

Browser requirements:

- Send requests with credentials (cookies).
- For non-GET requests, include `X-CSRF-Token` header with the value from `apant_csrf` cookie.

## Endpoints

### `GET /api/v1/auth/csrf`

Purpose: set CSRF cookie for new tabs/app load.

Response example:

```json
{
  "success": true,
  "message": "csrf ready",
  "data": {}
}
```

### `POST /api/v1/auth/register`

Request body:

```json
{
  "username": "string",
  "email": "string",
  "password": "string"
}
```

Response example:

```json
{
  "success": true,
  "message": "user registered successfully",
  "data": {
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "string",
      "username": "string",
      "email": "string",
      "role": "pentester"
    }
  }
}
```

### `POST /api/v1/auth/login`

Request body:

```json
{
  "username": "string",
  "password": "string"
}
```

Response example:

```json
{
  "success": true,
  "message": "login successful",
  "data": {
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "string",
      "username": "string",
      "email": "string",
      "role": "pentester"
    }
  }
}
```

### `POST /api/v1/auth/refresh-token`

Headers:

- `X-CSRF-Token: <apant_csrf>`

Request body (optional when using cookies):

```json
{
  "refresh_token": "string"
}
```

Response example:

```json
{
  "success": true,
  "message": "token refreshed successfully",
  "data": {
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "string",
      "username": "string",
      "email": "string",
      "role": "pentester"
    }
  }
}
```

### `POST /api/v1/auth/logout`

Headers:

- `X-CSRF-Token: <apant_csrf>`

Request body (optional when using cookies):

```json
{
  "refresh_token": "string"
}
```

Response example:

```json
{
  "success": true,
  "message": "logout successful",
  "data": {}
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
