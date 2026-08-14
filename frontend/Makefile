.PHONY: help dev build start lint typecheck test install clean

help:
	@echo "Frontend - Logistics Web Application"
	@echo "===================================="
	@echo "  make install     - Install dependencies"
	@echo "  make dev         - Start development server"
	@echo "  make build       - Build for production"
	@echo "  make start       - Start production server"
	@echo "  make lint        - Run ESLint"
	@echo "  make typecheck   - Run TypeScript type checking"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"

install:
	npm install

dev:
	npm run dev

build:
	npm run build

start:
	npm run start

lint:
	npm run lint

typecheck:
	npm run typecheck

test:
	npm test

clean:
	rm -rf .next node_modules