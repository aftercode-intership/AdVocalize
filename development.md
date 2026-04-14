# VOCALIZE - LOCAL DEVELOPMENT SETUP

## Prerequisites
- Docker Desktop (v4.0+)
- Docker Compose (v2.0+)
- Git
- Node.js 18+ (for running local frontend without Docker)
- Go 1.21+ (for backend development)
- Python 3.10+ (for service development)

## Quick Start

### 1. Clone Repository
\`\`\`bash
git clone https://github.com/vocalize/vocalize.git
cd vocalize
\`\`\`

### 2. Setup Environment
\`\`\`bash
cp .env.example .env.local
# Edit .env.local if needed for local configuration
\`\`\`

### 3. Start All Services
\`\`\`bash
docker-compose up -d
\`\`\`

Wait for all services to be healthy:
\`\`\`bash
docker-compose ps
\`\`\`

All services should show "healthy" or "running" status.

### 4. Access Services

| Service | URL | Credentials |
|---------|-----|-------------|
| Frontend | http://localhost:3000 | - |
| API | http://localhost:8081 | - |
| PostgreSQL | localhost:5432 | user: vocalize / pass: localpassword |
| Redis | localhost:6379 | - |
| MinIO Console | http://localhost:9001 | user: minioadmin / pass: minioadmin |
| Prometheus | http://localhost:9090 | - |
| Grafana | http://localhost:3001 | user: admin / pass: admin |

### 5. Verify Setup
\`\`\`bash
# Check API health
curl http://localhost:8081/health

# Check database
docker exec vocalize-postgres psql -U vocalize -d vocalize_db -c "SELECT 1"

# Check Redis
docker exec vocalize-redis redis-cli ping
\`\`\`

## Development Workflow

### Backend Development (Go)

1. Install dependencies:
   \`\`\`bash
   cd backend
   go mod download
   \`\`\`

2. Run hot-reload (requires air):
   \`\`\`bash
   go install github.com/cosmtrek/air@latest
   cd backend && air
   \`\`\`

3. Run migrations:
   \`\`\`bash
   docker exec vocalize-postgres psql -U vocalize -d vocalize_db -f migrations/001_initial.sql
   \`\`\`

### Frontend Development (Next.js)

1. Install dependencies:
   \`\`\`bash
   cd frontend
   npm install
   \`\`\`

2. Run development server:
   \`\`\`bash
   npm run dev
   \`\`\`

3. Access at http://localhost:3000

### Service Development (Python)

Each service can run independently:

\`\`\`bash
cd services/script-gen
pip install -r requirements.txt
python main.py
\`\`\`

## Useful Commands

### Docker
\`\`\`bash
# View logs
docker-compose logs -f backend

# Stop all services
docker-compose down

# Rebuild images
docker-compose build

# Run specific service
docker-compose up -d postgres
\`\`\`

### Database
\`\`\`bash
# Connect to database
docker exec -it vocalize-postgres psql -U vocalize -d vocalize_db

# Run migrations
docker exec vocalize-postgres psql -U vocalize -d vocalize_db -f migrations/001_initial.sql

# Reset database
docker exec vocalize-postgres psql -U vocalize -c "DROP DATABASE vocalize_db;"
docker exec vocalize-postgres psql -U vocalize -c "CREATE DATABASE vocalize_db;"
\`\`\`

### Redis
\`\`\`bash
# Connect to Redis
docker exec -it vocalize-redis redis-cli

# Clear all data
docker exec vocalize-redis redis-cli FLUSHALL
\`\`\`

## Troubleshooting

### Port Already in Use
\`\`\`bash
# Find what's using port
lsof -i :3000
# Kill process
kill -9 <PID>
\`\`\`

### Database Connection Error
1. Ensure PostgreSQL container is running: \`docker-compose ps postgres\`
2. Check credentials in .env.local
3. Reset database (see above)

### Service Won't Start
1. Check logs: \`docker-compose logs service-name\`
2. Verify all dependencies are running
3. Rebuild image: \`docker-compose build service-name\`

## Next Steps

1. Create database migrations (see Database Setup)
2. Implement authentication endpoints
3. Set up API testing
4. Configure IDE/Editor for development