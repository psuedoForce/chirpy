# Chirpy

Chirpy is a simple social media platform built with Go, allowing users to create accounts, post short messages called "chirps", and interact with the platform via a REST API. It's designed as a Twitter-like application with user authentication, chirp management, and basic web serving capabilities.

## Features

- **User Management**: Create user accounts with email and password
- **Authentication**: JWT-based authentication with refresh tokens
- **Chirp Management**: Create, read, update, and delete chirps (posts)
- **Web Interface**: Basic HTML interface served from the root path
- **Metrics**: Admin endpoint for server metrics and hit counts
- **Webhook Support**: Integration with external services via webhooks

## Tech Stack

- **Backend**: Go 1.26.1
- **Database**: PostgreSQL
- **Authentication**: JWT tokens with Argon2id password hashing
- **Database Queries**: sqlc for type-safe SQL generation
- **Environment**: godotenv for configuration

## Prerequisites

- Go 1.26.1 or later
- PostgreSQL database
- Environment variables configured (see Setup section)

## Setup

1. **Clone the repository**:
   ```bash
   git clone https://github.com/psuedoforce/chirpy.git
   cd chirpy
   ```

2. **Install dependencies**:
   ```bash
   go mod download
   ```

3. **Set up the database**:
   - Create a PostgreSQL database
   - Run the schema migrations from `sql/schema/` in order

4. **Generate database code**:
   ```bash
   sqlc generate
   ```

5. **Configure environment variables**:
   Create a `.env` file in the root directory with:
   ```
   DB_URL=postgresql://username:password@localhost:5432/chirpy_db
   SECRET_TOKEN=your_jwt_secret_here
   POLKA_KEY=your_polka_webhook_key_here
   ```

6. **Run the application**:
   ```bash
   go run main.go
   ```

The server will start on `http://localhost:8080`.

## API Endpoints

### Health Check
- `GET /api/healthz` - Health check endpoint

### User Management
- `POST /api/users` - Create a new user
- `PUT /api/users` - Update user information (requires authentication)
- `POST /api/login` - User login

### Chirps
- `POST /api/chirps` - Create a new chirp (requires authentication)
- `GET /api/chirps` - Get all chirps
- `GET /api/chirps/{chirpID}` - Get a specific chirp
- `DELETE /api/chirps/{chirpID}` - Delete a chirp (requires authentication)

### Authentication
- `POST /api/refresh` - Refresh access token
- `POST /api/revoke` - Revoke refresh token

### Admin
- `GET /admin/metrics` - View server metrics
- `POST /admin/reset` - Reset server state (development only)

### Webhooks
- `POST /api/polka/webhooks` - Handle external webhooks

## Usage

1. Visit `http://localhost:8080` for the basic web interface
2. Use API endpoints to interact with the platform programmatically
3. Static assets are served from `/app/` and `/api/assets/`

## Development

- Database queries are defined in `sql/queries/`
- Schema migrations in `sql/schema/`
- Generated database code in `internal/database/`
- Authentication logic in `internal/auth/`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This project is open source. Please check the license file for details.
