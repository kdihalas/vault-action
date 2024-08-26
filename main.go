package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/sethvargo/go-githubactions"
)

func main() {
	ctx := context.Background()
	githubactions.Infof("=> starting vault action")

	// Read inputs
	vaultUrl := githubactions.GetInput("url")
	if vaultUrl == "" {
		githubactions.Fatalf("url is required")
	}
	vaultRole := githubactions.GetInput("role")
	if vaultRole == "" {
		githubactions.Fatalf("role is required")
	}
	vaultJwtClaim := githubactions.GetInput("jwt_claim")
	if vaultJwtClaim == "" {
		githubactions.Fatalf("jwt_claim is required")
	}
	githubactions.Infof("=> reading the github token")
	token, err := githubactions.GetIDToken(ctx, vaultJwtClaim)
	if err != nil {
		githubactions.Fatalf("Failed to get github token: %v", err)
	}

	githubactions.Infof("=> creating vault client")
	client, err := vault.New(
		vault.WithAddress(vaultUrl),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		githubactions.Fatalf("Failed to create vault client: %v", err)
	}

	// Read the Vault Output Token flag and convert it to boolean
	vaultOutputToken, err := strconv.ParseBool(githubactions.GetInput("output_token"))
	if err != nil {
		githubactions.Fatalf("Failed to parse output_token: %v", err)
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
		githubactions.Infof("=> exporting vault token as env variable")
		githubactions.SetEnv("VAULT_TOKEN", resp.Auth.ClientToken)
		githubactions.AddMask(resp.Auth.ClientToken)
	}

	secrets := githubactions.GetInput("secrets")
	if secrets == "empty" {
		githubactions.Infof("=> no secrets to read")
		return
	}

	githubactions.Infof("=> reading secrets")
	for _, line := range strings.Split(secrets, ";\n") {
		secret := strings.TrimRight(strings.TrimSpace(line), ";")
		if secret == "" {
			continue
		}
		secretParsed := strings.Split(secret, "|")
		left, right := strings.TrimSpace(secretParsed[0]), strings.TrimSpace(secretParsed[1])
		leftParsed := strings.Split(left, " ")
		parsedPath := strings.Split(strings.TrimSpace(leftParsed[0]), "/")
		mountPath := parsedPath[0]
		secretPath := strings.Join(parsedPath[1:], "/")
		key := strings.TrimSpace(leftParsed[1])
		vaultSecret, err := client.Secrets.KvV2Read(ctx, secretPath,
			vault.WithToken(resp.Auth.ClientToken),
			vault.WithMountPath(mountPath),
		)
		if err != nil {
			githubactions.Fatalf("Failed to read secret %s%s: %v", mountPath, secretPath, err)
		}
		githubactions.SetEnv(right, vaultSecret.Data.Data[key].(string))
		githubactions.AddMask(right)
	}
}
