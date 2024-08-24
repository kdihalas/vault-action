package main

import (
	"context"
	"strconv"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/sethvargo/go-githubactions"
)

func main() {
	ctx := context.Background()
	githubactions.Infof("Starting vault action")

	// Read inputs
	vaultUrl := githubactions.GetInput("vault_url")
	if vaultUrl == "" {
		githubactions.Fatalf("vault_url is required")
	}
	vaultRole := githubactions.GetInput("vault_role")
	if vaultRole == "" {
		githubactions.Fatalf("vault_role is required")
	}
	vaultJwtClaim := githubactions.GetInput("vault_jwt_claim")
	if vaultJwtClaim == "" {
		githubactions.Fatalf("vault_jwt_claim is required")
	}
	githubactions.Infof("Reading the github token")
	token, err := githubactions.GetIDToken(ctx, vaultJwtClaim)
	if err != nil {
		githubactions.Fatalf("Failed to get github token: %v", err)
	}

	githubactions.Infof("creating vault client")
	client, err := vault.New(
		vault.WithAddress(vaultUrl),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		githubactions.Fatalf("Failed to create vault client: %v", err)
	}

	// Read the Vault Output Token flag and convert it to boolean
	vaultOutputToken, err := strconv.ParseBool(githubactions.GetInput("vault_output_token"))
	if err != nil {
		githubactions.Fatalf("Failed to parse vault_output_token: %v", err)
	}

	// Login to vault
	resp, err := client.Auth.JwtLogin(ctx, schema.JwtLoginRequest{
		Jwt:  token,
		Role: vaultRole,
	})
	if err != nil {
		githubactions.Fatalf("Failed to login to vault: %v", err)
	}
	// Set the vault token as an environment variable
	if vaultOutputToken {
		githubactions.Infof("exporting vault token as env variable")
		githubactions.SetEnv("VAULT_TOKEN", resp.Auth.ClientToken)
	}

}
