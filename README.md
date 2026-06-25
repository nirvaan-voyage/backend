# Nirvaan Backend

Gin backend for Nirvaan registration. This repository currently implements only Meghna's registration responsibilities: user signup, PostgreSQL persistence, bcrypt password hashing, and JWT generation after registration.

## Folder Structure

```text
nirvaan-backend/
├── cmd/
│   └── main.go
├── internal/
│   ├── auth/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── jwt.go
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   ├── migrations/
│   │   │   └── 001_create_users.sql
│   │   └── postgres.go
│   └── models/
│       └── user.go
├── .env.example
├── docker-compose.yml
├── go.mod
└── README.md
```

## Environment Variables

Copy `.env.example` to `.env`:

```bash
PORT=8080
DATABASE_URL=postgres://USER:PASSWORD@HOST/neondb?sslmode=require
JWT_SECRET=replace-with-a-long-random-secret
JWT_EXPIRES_IN_HOURS=168
GIN_MODE=debug
```

Use the Neon connection string for `DATABASE_URL`. Keep `sslmode=require` for Neon.

## Database

The users table is defined in:

```text
internal/database/migrations/001_create_users.sql
```

For Neon, run that SQL in the Neon SQL editor before starting the API.

For a local PostgreSQL database:

```bash
docker compose up -d
```

Local Docker URL:

```text
postgres://nirvaan:nirvaan@localhost:5432/nirvaan?sslmode=disable
```

## Run the Server

```bash
go mod tidy
go run ./cmd
```

Health check:

```bash
curl http://localhost:8080/health
```

## Registration Endpoint

`POST /auth/register`

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Meghna",
    "username": "meghna",
    "email": "meghna@gmail.com",
    "phone": "9876543210",
    "age": 20,
    "password": "Secret123"
  }'
```

Successful response:

```json
{
  "token": "jwt-token",
  "user": {
    "id": 1,
    "name": "Meghna",
    "username": "meghna",
    "email": "meghna@gmail.com",
    "phone": "9876543210",
    "age": 20,
    "created_at": "2026-06-25T00:00:00Z",
    "updated_at": "2026-06-25T00:00:00Z"
  }
}
```

If a username is taken:

```json
{
  "message": "Username is already taken",
  "field": "username",
  "suggestions": ["meghna_1", "meghna_24", "meghna_101"]
}
```

## Flutter Connection

Flutter calls this endpoint from:

```text
lib/core/services/auth_service.dart
```

Run Flutter with a backend URL:

```bash
flutter run --dart-define=NIRVAAN_API_URL=http://10.0.2.2:8080
```

Use `http://localhost:8080` for desktop builds and `http://10.0.2.2:8080` for Android emulator.
