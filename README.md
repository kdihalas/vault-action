# Vault Github Action

---

This is not an official Action and has no affiliation to hashicorp

---

## Authentication Methods

This action supports only JWT authentication with OIDC Provider. To setup your vault instance please follow the github documentation: https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-hashicorp-vault

## Example Usage

Authenticate with Vault and get a Vault Token

``` yaml
jobs:
  build:
    # ...
    steps:
      # ...
      - name: Import Secrets
        id: import-secrets
        uses: hashicorp/vault-action@v2
        with:
          url: https://vault.mycompany.com:8200
          output_token: "true"
      # ...
```

Get secrets and export them to env variables, secrets format is like this: `<path> <key> | <env_variable>;`

``` yaml
jobs:
  build:
    # ...
    steps:
      # ...
      - name: Import Secrets
        id: import-secrets
        uses: hashicorp/vault-action@v2
        with:
          url: https://vault.mycompany.com:8200
          secrets: |
            secret/data/ci/aws accessKey | AWS_ACCESS_KEY_ID;
      # ...

```