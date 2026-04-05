# Vault GitHub Action

A GitHub Action for authenticating to HashiCorp Vault using GitHub's OIDC provider and optionally reading secrets into your workflow.

**This is not an official HashiCorp project and has no affiliation with HashiCorp.**

## Features

- Authenticate to Vault using GitHub's OpenID Connect (OIDC) provider
- Optional JWT token export for direct Vault API calls in subsequent steps
- Automatic secret retrieval from Vault KV v2 mounts
- Masked output for all sensitive values
- No external dependencies—runs in a Docker container

## Usage

### Minimal: authenticate and get a Vault token

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - name: Authenticate to Vault
        uses: kdihalas/vault-action@v0
        with:
          url: https://vault.example.com:8200
          output_token: "true"

      - name: Use Vault token
        run: |
          curl -H "X-Vault-Token: $VAULT_TOKEN" \
            https://vault.example.com:8200/v1/secret/data/my-secret
        env:
          VAULT_TOKEN: ${{ env.VAULT_TOKEN }}
```

### Typical: fetch secrets into environment variables

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - name: Import secrets from Vault
        uses: kdihalas/vault-action@v0
        with:
          url: https://vault.example.com:8200
          secrets: |
            secret/data/ci/aws accessKey | AWS_ACCESS_KEY_ID;
            secret/data/ci/aws secretKey | AWS_SECRET_ACCESS_KEY;

      - name: Deploy with AWS credentials
        run: |
          aws s3 ls
          aws sts get-caller-identity
```

### Full: custom role, multiple secrets, downstream usage

```yaml
jobs:
  ci:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4

      - name: Fetch secrets and token from Vault
        uses: kdihalas/vault-action@v0
        with:
          url: https://vault.example.com:8200
          role: my-custom-role
          jwt_claim: ref
          output_token: "true"
          secrets: |
            secret/data/prod/database password | DB_PASSWORD;
            secret/data/prod/api_key token | API_TOKEN;
            secret/data/ci/npm token | NPM_TOKEN;

      - name: Build and test
        run: |
          npm ci
          npm run build
          npm run test
        env:
          NPM_TOKEN: ${{ env.NPM_TOKEN }}
          DATABASE_URL: postgres://user:${{ env.DB_PASSWORD }}@db.example.com/prod

      - name: Deploy (direct Vault call)
        run: |
          # Use the Vault token directly for advanced operations
          curl -X POST \
            -H "X-Vault-Token: $VAULT_TOKEN" \
            -d @config.json \
            https://vault.example.com:8200/v1/auth/approle/role/my-role/secret-id
        env:
          VAULT_TOKEN: ${{ env.VAULT_TOKEN }}
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `url` | yes | — | Vault instance URL (e.g., `https://vault.example.com:8200`) |
| `role` | no | `github-action` | Vault JWT authentication role name |
| `jwt_claim` | no | `actor` | GitHub claim to use as the JWT audience (e.g., `actor`, `ref`, `repo`) |
| `output_token` | no | `false` | If `true`, export the Vault client token as `VAULT_TOKEN` environment variable (masked) |
| `secrets` | no | `empty` | Multi-line string of secrets to fetch from Vault (see format below) |

## Secrets format

The `secrets` input expects a multi-line string with entries in the format:

```
<mount>/<path> <key> | <ENV_VAR_NAME>;
```

Each entry is separated by `;\n` (semicolon + newline). Parsing rules:

- The first path segment (`secret`) is the KV v2 mount name.
- Remaining segments (`data/ci/aws`) form the secret path.
- The `<key>` is the key within that secret object.
- `<ENV_VAR_NAME>` is the environment variable name to export.
- All values are automatically masked in workflow logs.
- Only string values are supported (nested objects will cause an error).

### Example

```yaml
secrets: |
  secret/data/ci/aws accessKey | AWS_ACCESS_KEY_ID;
  secret/data/ci/aws secretKey | AWS_SECRET_ACCESS_KEY;
  secret/data/github token | GITHUB_TOKEN;
```

This reads three secrets from the `secret` KV v2 mount and exports them as masked environment variables.

## Vault setup

### Prerequisites

1. **Enable JWT auth method** in your Vault instance:
   ```bash
   vault auth enable jwt
   ```

2. **Configure the JWT auth method** to trust GitHub's OIDC provider:
   ```bash
   vault write auth/jwt/config \
     jwks_url="https://token.actions.githubusercontent.com/.well-known/jwks.json" \
     bound_issuer="https://token.actions.githubusercontent.com"
   ```

3. **Create a Vault role** (name matches the `role` input, default `github-action`):
   ```bash
   vault write auth/jwt/role/github-action \
     bound_audiences="<jwt_claim>" \
     user_claim="actor" \
     role_type="jwt" \
     policies="ci-policy"
   ```
   Adjust `bound_audiences` to match your `jwt_claim` input and bind claims to your repository and branch requirements.

For complete setup instructions, including claim bindings and policy configuration, see GitHub's official guide: [Configuring OpenID Connect in HashiCorp Vault](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-hashicorp-vault).

## Permissions

Your job must include the following permissions to allow the action to request the GitHub OIDC token:

```yaml
permissions:
  id-token: write
  contents: read
```

Adjust additional permissions (e.g., `contents: write`, `packages: write`) based on your workflow's needs.

## Versioning

This action is released via [release-please](https://github.com/googleapis/release-please). Consume it by pinning to a major version or full semantic version tag:

- `@v0` — latest v0.x.x release (recommended for stability)
- `@v0.0.43` — specific release version
- `@main` — bleeding edge (not recommended for production)

Released images are available on GitHub Container Registry: `ghcr.io/kdihalas/vault-action:<tag>`

## License

MIT License. See [LICENSE](LICENSE) for details.
