# Vault GitHub Action

[![E2E](https://github.com/kdihalas/vault-action/actions/workflows/e2e.yml/badge.svg)](https://github.com/kdihalas/vault-action/actions/workflows/e2e.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/kdihalas/vault-action)](https://goreportcard.com/report/github.com/kdihalas/vault-action)

A GitHub Action for authenticating to HashiCorp Vault using GitHub's OIDC provider and optionally reading secrets into your workflow.

**This is not an official HashiCorp project and has no affiliation with HashiCorp.**

## Features

- Authenticate to Vault using GitHub's OpenID Connect (OIDC) provider
- Optional JWT token export for direct Vault API calls in subsequent steps
- Automatic secret retrieval from Vault KV v2 mounts
- AWS dynamic credentials via the Vault AWS secrets engine
- Kubernetes dynamic credentials via the Vault Kubernetes secrets engine — generates a merged multi-cluster kubeconfig
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
| `namespace` | no | | Set the vault namespace |
| `secrets` | no | `empty` | Multi-line string of KV v2 secrets to fetch from Vault (see format below) |
| `aws_secrets` | no | `empty` | Multi-line string of AWS secrets engine roles to generate dynamic credentials from (see format below) |
| `aws_duration` | no | 900s | Duration for AWS credentials (e.g. "900s", "15m"). Only applies to aws_secrets entries. |
| `kube_secrets` | no | `empty` | Multi-line string of Kubernetes secrets engine roles to generate dynamic service account tokens from (see format below) |

## Outputs

| Output | Description |
|--------|-------------|
| `kubeconfig` | Path to the generated kubeconfig file (only set when `kube_secrets` is provided) |

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

## AWS dynamic credentials

The `aws_secrets` input uses Vault's [AWS secrets engine](https://developer.hashicorp.com/vault/docs/secrets/aws) to generate short-lived IAM credentials on demand. Each entry specifies a mount path, a Vault AWS role name, and an environment variable prefix:

```
<mount>/<role> | <ENV_PREFIX>;
```

For each entry the action calls `GET /v1/<mount>/creds/<role>` and exports:

- `<PREFIX>_ACCESS_KEY_ID`
- `<PREFIX>_SECRET_ACCESS_KEY`
- `<PREFIX>_SESSION_TOKEN` — only when Vault returns a session token (STS / assumed-role credential types); omitted for plain `iam_user` roles

All three values are masked in workflow logs. The lease ID is printed as an info log so you can revoke credentials manually if needed.

### Example: KV secrets and AWS dynamic credentials in one step

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4

      - name: Fetch secrets and AWS credentials from Vault
        uses: kdihalas/vault-action@v0
        with:
          url: https://vault.example.com:8200
          role: github-action
          secrets: |
            secret/data/prod/database password | DB_PASSWORD;
            secret/data/prod/api token | API_TOKEN;
          aws_secrets: |
            aws/deploy-role | AWS;
            aws-prod/readonly | PROD_AWS;

      - name: Deploy
        run: |
          aws s3 sync ./dist s3://my-bucket
          aws ecs update-service --cluster prod --service api --force-new-deployment
        env:
          AWS_ACCESS_KEY_ID: ${{ env.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ env.AWS_SECRET_ACCESS_KEY }}
          AWS_SESSION_TOKEN: ${{ env.AWS_SESSION_TOKEN }}
```

`PROD_AWS_ACCESS_KEY_ID`, `PROD_AWS_SECRET_ACCESS_KEY`, and `PROD_AWS_SESSION_TOKEN` are also available from the second entry.

### Vault AWS secrets engine setup

Enable and configure the AWS secrets engine before using this input:

```bash
vault secrets enable -path=aws aws

vault write aws/config/root \
  access_key=<ADMIN_ACCESS_KEY> \
  secret_key=<ADMIN_SECRET_KEY> \
  region=us-east-1

vault write aws/roles/deploy-role \
  credential_type=assumed_role \
  role_arns=arn:aws:iam::123456789012:role/DeployRole
```

See the full guide at https://developer.hashicorp.com/vault/docs/secrets/aws.

## Kubernetes dynamic credentials

The `kube_secrets` input uses Vault's [Kubernetes secrets engine](https://developer.hashicorp.com/vault/docs/secrets/kubernetes) to generate short-lived service account tokens on demand. Each entry specifies a Vault mount/role, a target namespace, and the cluster's API server URL and CA certificate:

```
<mount>/<role> <namespace> <api_server_url> <ca_cert_b64> | <context_name>;
```

For each entry the action calls `POST /v1/<mount>/creds/<role>` and:

- Builds a kubeconfig `cluster` entry from `<api_server_url>` and `<ca_cert_b64>`.
- Adds a `user` entry with the generated `service_account_token` (masked in logs).
- Adds a `context` entry named `<context_name>` linking cluster, user, and namespace.
- Sets `current-context` to the first entry's context name.

The merged kubeconfig is written to `$RUNNER_TEMP/vault-action-kubeconfig-<random>.yaml` with mode `0600`. Its path is exported as `KUBECONFIG` (so `kubectl` in subsequent steps picks it up automatically) and also emitted as the `kubeconfig` step output. The lease ID and service account name/namespace are printed as info logs per entry; the token itself is never logged.

### Example: multi-cluster deploy

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read
    steps:
      - uses: actions/checkout@v4

      - name: Fetch Kubernetes credentials from Vault
        id: vault
        uses: kdihalas/vault-action@v0
        with:
          url: https://vault.example.com:8200
          role: github-action
          kube_secrets: |
            k8s/deployer default https://prod-api.example.com:6443 LS0tLS1CRUdJTi... | prod;
            k8s/deployer staging https://stg-api.example.com:6443 LS0tLS1CRUdJTi... | staging;

      # KUBECONFIG is already set; kubectl uses the merged file automatically.
      # current-context is "prod" (first entry).
      - name: Deploy to prod
        run: |
          kubectl apply -f k8s/prod/

      - name: Deploy to staging
        run: |
          kubectl config use-context staging
          kubectl apply -f k8s/staging/

      # Explicit path via the step output if needed.
      - name: Print kubeconfig path
        run: echo "kubeconfig at ${{ steps.vault.outputs.kubeconfig }}"
```

### Vault Kubernetes secrets engine setup

Enable and configure the Kubernetes secrets engine before using this input:

```bash
# Enable the engine at a custom mount path
vault secrets enable -path=k8s kubernetes

# Point it at your cluster (runs inside the cluster or with explicit config)
vault write k8s/config \
  kubernetes_host="https://prod-api.example.com:6443" \
  kubernetes_ca_cert=@/path/to/ca.crt \
  service_account_jwt="$(kubectl get secret vault-sa-token -o jsonpath='{.data.token}' | base64 -d)"

# Create a role — choose one of: kubernetes_role_name (existing ClusterRole/Role),
# generated_role_rules (inline RBAC rules), or service_account_name (existing SA).
vault write k8s/roles/deployer \
  allowed_kubernetes_namespaces="default,staging" \
  kubernetes_role_name=deploy-role \
  token_ttl=10m \
  token_max_ttl=30m
```

The Vault policy for the GitHub Actions JWT role must allow `read` on `k8s/creds/<role-name>`.

See the full guide at https://developer.hashicorp.com/vault/docs/secrets/kubernetes.

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
