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

	parseLines := func(input string) []string {
		var lines []string
		for line := range strings.SplitSeq(input, ";\n") {
			line = strings.TrimRight(strings.TrimSpace(line), ";")
			if line != "" {
				lines = append(lines, line)
			}
		}
		return lines
	}

	secrets := githubactions.GetInput("secrets")
	if secrets != "empty" && secrets != "" {
		githubactions.Infof("=> reading secrets")
		for _, line := range parseLines(secrets) {
			parts := strings.Split(line, "|")
			left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
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
				githubactions.Fatalf("Failed to read secret %s/%s: %v", mountPath, secretPath, err)
			}
			githubactions.SetEnv(right, vaultSecret.Data.Data[key].(string))
			githubactions.AddMask(vaultSecret.Data.Data[key].(string))
		}
	}

	awsSecrets := githubactions.GetInput("aws_secrets")
	if awsSecrets != "empty" && awsSecrets != "" {
		githubactions.Infof("=> generating AWS dynamic credentials")
		for _, line := range parseLines(awsSecrets) {
			parts := strings.Split(line, "|")
			mountRole := strings.TrimSpace(parts[0])
			prefix := strings.TrimRight(strings.TrimSpace(parts[1]), "_")

			mountPath, roleName, ok := strings.Cut(mountRole, "/")
			if !ok {
				githubactions.Fatalf("Invalid aws_secrets entry (expected <mount>/<role>): %q", mountRole)
			}

			awsCreds, err := client.Secrets.AwsGenerateCredentials(ctx, roleName, "", "", "",
				vault.WithToken(resp.Auth.ClientToken),
				vault.WithMountPath(mountPath),
			)
			if err != nil {
				githubactions.Fatalf("Failed to generate AWS credentials for %s/%s: %v", mountPath, roleName, err)
			}

			githubactions.Infof("=> AWS credentials generated (lease_id: %s)", awsCreds.LeaseID)

			accessKey, _ := awsCreds.Data["access_key"].(string)
			secretKey, _ := awsCreds.Data["secret_key"].(string)
			sessionToken, _ := awsCreds.Data["security_token"].(string)

			githubactions.SetEnv(prefix+"_ACCESS_KEY_ID", accessKey)
			githubactions.AddMask(accessKey)

			githubactions.SetEnv(prefix+"_SECRET_ACCESS_KEY", secretKey)
			githubactions.AddMask(secretKey)

			if sessionToken != "" {
				githubactions.SetEnv(prefix+"_SESSION_TOKEN", sessionToken)
				githubactions.AddMask(sessionToken)
			}
		}
	}
}
