# Animation Server

A server for generating and storing p5.js animations using Claude API.

## Prerequisites

- Go 1.22 or later
- PostgreSQL database
- Claude API key

## Setup

1. Clone the repository
2. Install dependencies:
   ```
   go get
   ```
3. Set up PostgreSQL database:
   - Make sure PostgreSQL is installed and running
   - Create a new database named "animations" or use an existing one
   - You can use the provided init_db.sql script to set up the database:
     ```
     psql -U postgres -f init_db.sql
     ```
   
4. Create a `.env` file based on `env.example`:
   ```
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
| DB_HOST | PostgreSQL database host | localhost |
| DB_PORT | PostgreSQL database port | 5432 |
| DB_USER | PostgreSQL database user | postgres |
| DB_PASSWORD | PostgreSQL database password | password |
| DB_NAME | PostgreSQL database name | animations |
| ALLOWED_ORIGINS | Comma-separated list of allowed origins for CORS | https://animate-frontend-production.up.railway.app,http://localhost:3000 |

## Running the server

Start the server with:

```
go run .
```

The server will run on port 8080 by default.

## API Endpoints

- `POST /generate-animation` - Generate animation from a description
- `POST /save-animation` - Save an animation to the database
- `GET /animation/{id}` - Retrieve an animation by ID
- `POST /fix-animation` - Fix broken animation code using error message

## Request Examples

### Fix Animation

```json
POST /fix-animation
Content-Type: application/json

{
  "broken_code": "new p5(function(p) { p.setup = function() { p.createCanvas(400, 400); }; p.draw = function() { p.background(220); p.ellipse(mouseX, mouseY, 50, 50); }; });",
  "error_message": "Uncaught ReferenceError: mouseX is not defined"
}
```

Response:
```json
{
  "code": "new p5(function(p) { p.setup = function() { p.createCanvas(400, 400); }; p.draw = function() { p.background(220); p.ellipse(p.mouseX, p.mouseY, 50, 50); }; });"
}
```

## Database Schema

The animations are stored in a PostgreSQL database with the following schema:

```sql
CREATE TABLE animations (
    id VARCHAR(32) PRIMARY KEY,
    code TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
``` 