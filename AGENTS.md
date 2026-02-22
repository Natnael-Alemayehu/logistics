# AGENTS.md

## Project Overview

This is a logistics project with a monorepo structure. Agents working on this codebase should reference the following documents:

## Project Structure

```
logistics/
├── backend/     # Go API server, database, and services
├── frontend/    # Next.js web application
├── mobile/      # Mobile application (not yet implemented)
└── *.md         # Documentation files
```

## Key Documents

- **Business Model**: See `./business_plan.md` for details on what is being built
- **Functional Requirements**: See `./functional_requirements.md` for functional requirements
- **Non-Functional Requirements**: See `./non_functional_requirements.md` for non-functional requirements
- **First Plan**: See `./plan.md` for the complete plan to build the MVP
- **Next Plan**: See `./plan-phase4.md` for the second iteration for this implementation
- **Frontend Implementation**: See `./frontend-plan.md` for the frontend implementation plan

## Development Guidelines

Before making changes or implementing features, consult `plan.md` first as it contains all necessary information for building the MVP.

### Quick Start

```bash
# Install all dependencies
make install

# Start development servers
make dev

# Or run individual projects
make backend CMD=dev
make frontend CMD=dev
```
