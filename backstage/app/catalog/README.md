# Production Catalog

This directory contains the **production** catalog data for Backstage running in Kubernetes.

## Purpose

- **Production Only**: This catalog is loaded in the Kubernetes (production) environment
- **Independent Management**: Completely separate from `examples/` (which is for local development)
- **Version Controlled**: Changes to organizational structure are tracked in Git

## Important Notes

- ✅ **Included in Docker image**: Copied to `/app/catalog` in production containers
- ✅ **Loaded in Kubernetes**: Referenced by `app-config.kubernetes.yaml`
- ❌ **NOT loaded locally**: Local development uses `examples/` instead

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

| Aspect | Local Development (`examples/`) | Production (`catalog/`) |
|--------|--------------------------------|-------------------------|
| Purpose | Learning, testing, demos | Actual organizational data |
| Location | `backstage-app/examples/` | `backstage-app/catalog/` |
| Config | `app-config.yaml` | `app-config.kubernetes.yaml` |
| Docker | NOT included | Included |
| Components | ✅ Included | ❌ Not included |
| Templates | ✅ Included | ❌ Not included |
| Domains | ✅ Included (examples) | ✅ Included (production) |
| Organizations | ✅ Included (examples) | ✅ Included (production) |

## See Also

- [Examples README](../examples/README.md) - Development/testing catalog
- [Backstage Catalog Documentation](https://backstage.io/docs/features/software-catalog/)
