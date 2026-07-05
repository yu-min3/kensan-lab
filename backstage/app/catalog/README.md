# Production Catalog

This directory contains the **production** catalog data for Backstage running in Kubernetes.

## Purpose

- **Production Only**: This catalog is loaded in the Kubernetes (production) environment
- **Single catalog**: Loaded in production and local development alike (the scaffold-era `examples/` demo data has been removed)
- **Version Controlled**: Changes to organizational structure are tracked in Git

## Important Notes

- ✅ **Included in Docker image**: Copied to `/app/catalog` in production containers
- ✅ **Loaded in Kubernetes**: Referenced by `app-config.kubernetes.yaml`
- ✅ **Also loaded locally**: `app-config.yaml` points at the same files for `make dev`

## Directory Structure

```
catalog/
├── domains/               # Production domains and systems
│   └── platform.yaml     # Platform infrastructure domain
└── organizations/         # Production teams and users
    └── teams.yaml        # Team structure and members
```

## File Descriptions

### Domains (`domains/`)
- **platform.yaml**: Platform infrastructure domain
  - Domain: `platform` - Infrastructure and internal tools
  - System: `developer-portal` - Backstage itself

### Organizations (`organizations/`)
- **teams.yaml**: Production team structure
  - Groups: `platform-engineering`, `application-developers`
  - Users: Platform members

## Usage

### Adding a New Domain

1. Create a new file in `domains/`:
   ```yaml
   # domains/my-domain.yaml
   apiVersion: backstage.io/v1alpha1
   kind: Domain
   metadata:
     name: my-domain
     description: Description of the domain
   spec:
     owner: team-name
   ```

2. Add related systems:
   ```yaml
   ---
   apiVersion: backstage.io/v1alpha1
   kind: System
   metadata:
     name: my-system
     description: System description
   spec:
     owner: team-name
     domain: my-domain
   ```

3. Commit and deploy (via GitOps)

### Adding a New Team

1. Edit `organizations/teams.yaml`:
   ```yaml
   ---
   apiVersion: backstage.io/v1alpha1
   kind: Group
   metadata:
     name: my-team
     description: Team description
   spec:
     type: team
     children: []
   ```

2. Add team members:
   ```yaml
   ---
   apiVersion: backstage.io/v1alpha1
   kind: User
   metadata:
     name: user.name
   spec:
     profile:
       displayName: User Name
       email: user@example.com
     memberOf: [my-team]
   ```

3. Commit and deploy

## Authentication Integration

**Note**: In production, user/group data should ideally come from an external identity provider (Keycloak, Okta, Azure AD) rather than being statically defined here.

The current static user definitions are for:
- Initial setup and testing
- Service accounts or system users
- Teams that don't map to external IdP

## Deployment

Changes to this catalog are automatically applied when:
1. Code is committed to Git
2. Docker image is rebuilt with `make build TAG=vX.Y.Z`
3. Image is pushed and Kubernetes deployment is updated (via Argo CD)

## Development vs Production

Both environments load this directory. The only difference is the config file that references it
(`app-config.yaml` for `make dev`, `app-config.kubernetes.yaml` in the Docker image).

## See Also

- [Backstage Catalog Documentation](https://backstage.io/docs/features/software-catalog/)
