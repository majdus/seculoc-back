.PHONY: all build test clean sqlc docker-up docker-down db-reset

# Variables
DB_URL=postgres://seculoc_user:seculoc_password@localhost:5432/seculoc_db?sslmode=disable

all: test build

run:
	go run cmd/server/main.go

build:
	go build -o bin/server cmd/server/main.go

test:
	go test -v ./...

clean:
	rm -rf bin

# Code Generation
sqlc:
	$(HOME)/go/bin/sqlc generate

swagger:
	$(HOME)/go/bin/swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

# Docker & Database
docker-up:
	docker compose up -d

docker-down:
	docker compose down

db-reset:
	@echo "Resetting database..."
	cat db/drop.sql db/schemas.sql | docker compose exec -T postgres psql -U seculoc_user -d seculoc_db

# Utils
logs:
	docker compose logs -f
