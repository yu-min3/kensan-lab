# Backstage Examples

This directory contains example data for local development of the Backstage application.

## Important Notes

- **Local Development Only**: These examples are ONLY loaded when running `make dev`
  - Uses `app-config.yaml` which references these examples
- **Not in Production**: Examples are NOT included in:
  - Docker images (`make run`, `make build`)
  - Kubernetes deployments (production environment)
  - `app-config.kubernetes.yaml` does NOT reference these examples

## Directory Structure

```
examples/
├── components/              # Example microservices components
│   ├── auth-service/       # Authentication service with TechDocs
│   ├── order-management-api/
│   ├── product-catalog-api/
│   └── recommendation-service/
├── domains/                # Domain and System definitions
│   └── e-commerce.yaml     # E-commerce domain with 4 systems
├── organizations/          # Team structure (Groups and Users)
│   └── microservices-teams.yaml  # Spotify model: Tribes, Squads, Chapters, Users
├── templates/              # Example Backstage templates
│   ├── template.yaml       # Template definition
│   └── content/            # Template skeleton files
├── entities.yaml           # Example entities (systems, resources)
└── org.yaml               # Example organization (users, groups)
```

## File Descriptions

### Components (`components/`)
Each component includes:
- `catalog-info.yaml`: Backstage catalog metadata
- `docs/`: TechDocs documentation
- `mkdocs.yml`: MkDocs configuration for TechDocs

### Domains (`domains/`)
- **e-commerce.yaml**: Defines the e-commerce Domain and its Systems
  - Domain: `e-commerce`
  - Systems: `product-catalog-system`, `order-management-system`, `user-authentication-system`, `recommendation-engine-system`

### Organizations (`organizations/`)
- **microservices-teams.yaml**: Spotify model organizational structure
  - Groups: Product Management, Tribes, Squads, Chapters
  - Users: Team members with squad and chapter memberships

## Usage

When running `make dev` in the backstage-app directory, all examples will be automatically loaded into the Backstage catalog.

To disable examples, comment out the relevant sections in `app-config.yaml`.

## Adding New Examples

1. Add your example files to the appropriate subdirectory
2. Update `app-config.yaml` catalog locations if needed
3. Restart the development server with `make dev`
