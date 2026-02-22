.PHONY: help backend frontend mobile install dev build test clean

help:
	@echo "Logistics Project - Available Commands"
	@echo "======================================"
	@echo "  make install     - Install all dependencies"
	@echo "  make dev         - Start development servers (backend + frontend)"
	@echo "  make build       - Build all projects"
	@echo "  make test        - Run all tests"
	@echo "  make clean       - Clean build artifacts"
	@echo ""
	@echo "Project-specific:"
	@echo "  make backend     - Run backend commands (use: make backend CMD=target)"
	@echo "  make frontend    - Run frontend commands (use: make frontend CMD=target)"
	@echo "  make mobile      - Run mobile commands (use: make mobile CMD=target)"

CMD ?= 

backend:
ifdef CMD
	$(MAKE) -C backend $(CMD)
else
	$(MAKE) -C backend
endif

frontend:
ifdef CMD
	$(MAKE) -C frontend $(CMD)
else
	$(MAKE) -C frontend
endif

mobile:
ifdef CMD
	$(MAKE) -C mobile $(CMD)
else
	$(MAKE) -C mobile
endif

install:
	$(MAKE) -C backend deps
	$(MAKE) -C frontend install

dev:
	@echo "Starting development servers..."
	@echo "Backend: http://localhost:8080"
	@echo "Frontend: http://localhost:3000"
	@trap 'kill 0' INT; \
	$(MAKE) -C backend dev & \
	$(MAKE) -C frontend dev & \
	wait

build:
	$(MAKE) -C backend build
	$(MAKE) -C frontend build

test:
	$(MAKE) -C backend test
	$(MAKE) -C frontend test

clean:
	$(MAKE) -C backend clean
	$(MAKE) -C frontend clean