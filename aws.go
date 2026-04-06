package main

import (
	"context"
	"strings"

	"github.com/hashicorp/vault-client-go"
	"github.com/sethvargo/go-githubactions"
)

// handleAwsSecrets processes the aws_secrets input, which is expected to be a list of lines in the format:
// <vault-mount>/<vault-role>|<env-var-prefix>
//
// For each line, it generates AWS credentials using the specified Vault mount and role, and then sets the resulting
// access key, secret key, and session token (if present) as environment variables with the specified prefix.
func handleAwsSecrets(ctx context.Context, client *vault.Client, token string, lines []string) {
	for _, line := range lines {
		parts := strings.Split(line, "|")
		mountRole := strings.TrimSpace(parts[0])
		prefix := strings.TrimRight(strings.TrimSpace(parts[1]), "_")

		mountPath, roleName, ok := strings.Cut(mountRole, "/")
		if !ok {
			githubactions.Fatalf("Invalid aws_secrets entry (expected <mount>/<role>): %q", mountRole)
		}

		awsCreds, err := client.Secrets.AwsGenerateCredentials(ctx, roleName, "", "", readInputWithFail("aws_duration"),
			vault.WithToken(token),
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
