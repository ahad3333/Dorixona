# Makefile for Pharmacy Telegram Bot

.PHONY: build run clean dev test railway-build migration-up migration-down migrate-up migrate-down help

# Database URL (Railway production yoki local)
DATABASE_URL ?= postgresql://postgres:LVoaaZpQnLHpnMDriIKOqrOLCiAbLLWF@yamabiko.proxy.rlwy.net:15284/railway

# Build the application
build:
	@echo "🔨 Building application..."
	cd cmd && go build -ldflags="-w -s" -o ../out main.go
	@echo "✅ Build completed: ./out"

# Run the application (after build)
run: build
	@echo "🚀 Starting bot..."
	./out

# Development mode (hot reload with go run)
dev:
	@echo "🔧 Running in development mode..."
	cd cmd && go run main.go

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	rm -f out
	@echo "✅ Cleaned"

# Test the application
test:
	@echo "🧪 Running tests..."
	go test ./...

# Railway build (same as build, but explicit)
railway-build: build

# Database migration up (golang-migrate tool)
migration-up:
	@echo "⬆️  Running migrations (golang-migrate)..."
	migrate -path ./migrations/postgres -database '$(DATABASE_URL)' up
	@echo "✅ Migrations completed"

# Database migration down (golang-migrate tool)
migration-down:
	@echo "⬇️  Rolling back migrations (golang-migrate)..."
	migrate -path ./migrations/postgres -database '$(DATABASE_URL)' down
	@echo "✅ Rollback completed"

# Database migration up (psql - alternative method)
migrate-up:
	@echo "⬆️  Running migrations (psql)..."
	psql $(DATABASE_URL) -f migrations/postgres/01_create_users.up.sql
	@echo "✅ Migrations completed"

# Database migration down (psql - alternative method)
migrate-down:
	@echo "⬇️  Rolling back migrations (psql)..."
	psql $(DATABASE_URL) -f migrations/postgres/01_create_users.down.sql
	@echo "✅ Rollback completed"

# Help
help:
	@echo "📖 Available commands:"
	@echo ""
	@echo "  🔨 Build & Run:"
	@echo "    make build           - Build the application"
	@echo "    make run             - Build and run the application"
	@echo "    make dev             - Run in development mode (hot reload)"
	@echo "    make clean           - Remove build artifacts"
	@echo "    make railway-build   - Build for Railway deployment"
	@echo ""
	@echo "  🧪 Testing:"
	@echo "    make test            - Run tests"
	@echo ""
	@echo "  🗄️  Database Migrations:"
	@echo "    make migration-up    - Run migrations (golang-migrate)"
	@echo "    make migration-down  - Rollback migrations (golang-migrate)"
	@echo "    make migrate-up      - Run migrations (psql)"
	@echo "    make migrate-down    - Rollback migrations (psql)"
	@echo ""
	@echo "  💡 Environment Variables:"
	@echo "    DATABASE_URL         - Database connection string"
	@echo ""
	@echo "  📝 Examples:"
	@echo "    make dev                                    - Local development"
	@echo "    DATABASE_URL=postgres://... make migrate-up - Custom DB migration"
