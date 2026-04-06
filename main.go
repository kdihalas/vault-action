package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/sethvargo/go-githubactions"
)

func main() {
	ctx := context.Background()
	githubactions.Infof("=> starting vault action")

	// Read required inputs and fail if they're not provided
	vaultUrl := readInputWithFail("url")
	vaultRole := readInputWithFail("role")
	vaultJwtClaim := readInputWithFail("jwt_claim")
	namespace := readInput("namespace")
	// Get the ID token for the specified claim, and fail if it cannot be obtained
	token := readTokenWithFail(ctx, vaultJwtClaim)

	githubactions.Infof("=> creating vault client")
	httpClient := &http.Client{
		Transport: &debugTransport{rt: http.DefaultTransport},
	}
	client, err := vault.New(
		vault.WithAddress(vaultUrl),
		vault.WithHTTPClient(httpClient),
		vault.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		githubactions.Fatalf("Failed to create vault client: %v", err)
	}
	// Set the namespace on the client if provided
	if namespace != "" {
		client.SetNamespace(namespace)
	}
	// Read the Vault Output Token flag and convert it to boolean
	vaultOutputToken := readBoolInputWithFail("output_token")

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

	// Helper function to parse the secrets input into lines, splitting on ';' and newlines, and trimming whitespace and trailing semicolons
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

	// Process the secrets inputs if they are provided
	secrets := githubactions.GetInput("secrets")
	if secrets != "empty" && secrets != "" {
		githubactions.Infof("=> reading secrets")
		handleKVSecrets(ctx, client, resp.Auth.ClientToken, parseLines(secrets))
	}

	// Process AWS secrets if provided
	awsSecrets := githubactions.GetInput("aws_secrets")
	if awsSecrets != "empty" && awsSecrets != "" {
		githubactions.Infof("=> generating AWS dynamic credentials")
		handleAwsSecrets(ctx, client, resp.Auth.ClientToken, parseLines(awsSecrets))
	}

	// Process Kubernetes secrets if provided
	kubeSecrets := githubactions.GetInput("kube_secrets")
	if kubeSecrets != "empty" && kubeSecrets != "" {
		githubactions.Infof("=> generating Kubernetes dynamic credentials")
		handleKubeSecrets(ctx, client, resp.Auth.ClientToken, parseLines(kubeSecrets))
	}
}
