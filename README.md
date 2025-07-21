# Animation Server

A server for generating and storing p5.js animations using Claude API.


## Prerequisites

- Go 1.22 or later
- PostgreSQL database
- Claude API key

## Setup

1. Clone the repository
2. Install dependencies:
   ```bash
   make deps
   ```
   or
   ```bash
   go mod download && go mod tidy
   ```

3. Set up PostgreSQL database:
   - Make sure PostgreSQL is installed and running
   - Create a new database named "animations" or use an existing one
   - You can use the provided init_db.sql script to set up the database:
     ```bash
     psql -U postgres -f init_db.sql
     ```
   
4. Create a `.env` file based on `env.example`:
   ```bash
   cp env.example .env
   ```
   
5. Edit the `.env` file with your specific configuration:
   - Set your Claude API key
   - Set your PostgreSQL database credentials
   - Set ALLOWED_ORIGINS to control CORS (comma-separated list of domains, e.g., "https://frontend1.example.com,https://frontend2.example.com" or use "*" for development)

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| CLAUDE_API_KEY | Your Claude API key | sk_123456789 |
| JWT_SECRET_KEY | Secret key for JWT token signing | your-secret-key |
| DB_HOST | PostgreSQL database host | localhost |
| DB_PORT | PostgreSQL database port | 5432 |
| DB_USER | PostgreSQL database user | postgres |
| DB_PASSWORD | PostgreSQL database password | password |
| DB_NAME | PostgreSQL database name | animations |
| ALLOWED_ORIGINS | Comma-separated list of allowed origins for CORS | https://animate-frontend-production.up.railway.app,http://localhost:3000 |

## Building and Running

### Using Makefile (Recommended)

```bash
# Build the application
make build

# Run the application
make run

# Build and run in one command
make build && ./bin/animate-server

# Clean build artifacts
make clean

# Run tests
make test

# Format code
make fmt

# See all available commands
make help
```

### Using Go commands directly

```bash
# Build the application
go build -o bin/animate-server ./cmd/animate-server

# Run the application
./bin/animate-server

# Or run directly without building
go run ./cmd/animate-server
```

The server will run on port 8080 by default.

## API Endpoints

### Authentication
- `POST /register` - Register a new user
- `POST /login` - Login user

### Animations (Protected routes require JWT token)
- `POST /generate-animation` - Generate animation from a description
- `POST /save-animation` - Save an animation to the database
- `GET /animation/{id}` - Retrieve an animation by ID (public)
- `GET /feed` - Get a random animation (public)
- `POST /save-mood` - Save user's mood after viewing an animation

## Request Examples

### Register User

```json
POST /register
Content-Type: application/json

{
  "username": "john_doe",
  "email": "john@example.com",
  "password": "securepassword"
}
```

### Generate Animation

```json
POST /generate-animation
Content-Type: application/json
Authorization: Bearer <jwt-token>

{
  "description": "A bouncing ball animation with rainbow colors"
}
```

### Save Mood

```json
POST /save-mood
Content-Type: application/json
Authorization: Bearer <jwt-token>

{
  "animationId": "abc123",
  "mood": "better"
}
```

## Database Schema

The animations are stored in a PostgreSQL database with the following schema:

```sql
CREATE TABLE animations (
    id VARCHAR(32) PRIMARY KEY,
    code TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE users (
    id VARCHAR(32) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255),
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_moods (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(32) NOT NULL,
    animation_id VARCHAR(32) NOT NULL,
    mood VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (animation_id) REFERENCES animations(id)
);
```

## Development

For development with hot reload, you can use [air](https://github.com/cosmtrek/air):

```bash
# Install air
go install github.com/cosmtrek/air@latest

# Run with hot reload
make dev
``` 
``` 
</rewritten_file>