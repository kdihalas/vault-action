package main

import (
	"context"
	"strings"

	"github.com/hashicorp/vault-client-go"
	"github.com/sethvargo/go-githubactions"
)

// handleKVSecrets processes the kv_secrets input, which is expected to be a list of lines in the format:
// <vault-mount>/<secret-path> <key>|<env-var-name>
//
// For each line, it reads the specified key from the Vault KV secret at the given path and mount, and then sets
// the value as an environment variable with the specified name.
func handleKVSecrets(ctx context.Context, client *vault.Client, token string, lines []string) {
	for _, line := range lines {
		parts := strings.Split(line, "|")
		left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		leftParsed := strings.Split(left, " ")
		parsedPath := strings.Split(strings.TrimSpace(leftParsed[0]), "/")
		mountPath := parsedPath[0]
		secretPath := strings.Join(parsedPath[1:], "/")
		key := strings.TrimSpace(leftParsed[1])
		vaultSecret, err := client.Secrets.KvV2Read(ctx, secretPath,
			vault.WithToken(token),
			vault.WithMountPath(mountPath),
		)
		if err != nil {
			githubactions.Fatalf("Failed to read secret %s/%s: %v", mountPath, secretPath, err)
		}
		githubactions.SetEnv(right, vaultSecret.Data.Data[key].(string))
		githubactions.AddMask(vaultSecret.Data.Data[key].(string))
	}
}
