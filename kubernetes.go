package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/sethvargo/go-githubactions"
	"gopkg.in/yaml.v3"
)

// kubeconfig structs — minimal v1 kubeconfig with yaml tags.
// We use gopkg.in/yaml.v3 (already an indirect dep) to avoid pulling in
// the full k8s.io/client-go tree.
type kubeClusterData struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type kubeClusterEntry struct {
	Name    string          `yaml:"name"`
	Cluster kubeClusterData `yaml:"cluster"`
}

type kubeUserData struct {
	Token string `yaml:"token"`
}

type kubeUserEntry struct {
	Name string       `yaml:"name"`
	User kubeUserData `yaml:"user"`
}

type kubeContextData struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace"`
}

type kubeContextEntry struct {
	Name    string          `yaml:"name"`
	Context kubeContextData `yaml:"context"`
}

type kubeConfig struct {
	APIVersion     string             `yaml:"apiVersion"`
	Kind           string             `yaml:"kind"`
	CurrentContext string             `yaml:"current-context"`
	Clusters       []kubeClusterEntry `yaml:"clusters"`
	Users          []kubeUserEntry    `yaml:"users"`
	Contexts       []kubeContextEntry `yaml:"contexts"`
}

// handleKubeSecrets processes the kube_secrets input, which is expected to be a list of lines in the format:
// <vault-mount>/<vault-role> <namespace> <api_server_url> <ca_cert_b64>|<kube-context-name>
//
// For each line, it generates Kubernetes credentials using the specified Vault mount and role, and then constructs
// a kubeconfig file with a context for each entry. The resulting kubeconfig is written to a temporary file, and
// the KUBECONFIG environment variable is set to point to that file. The path to the kubeconfig file is also set as an output variable "kubeconfig".
func handleKubeSecrets(ctx context.Context, client *vault.Client, token string, lines []string) {
	cfg := kubeConfig{
		APIVersion: "v1",
		Kind:       "Config",
	}

	for i, line := range lines {
		pipeIdx := strings.LastIndex(line, "|")
		if pipeIdx < 0 {
			githubactions.Fatalf("kube_secrets entry missing '|' separator: %q", line)
		}
		left := strings.TrimSpace(line[:pipeIdx])
		contextName := strings.TrimSpace(line[pipeIdx+1:])

		fields := strings.Fields(left)
		if len(fields) != 4 {
			githubactions.Fatalf("kube_secrets entry must have 4 space-separated fields left of '|' (<mount>/<role> <namespace> <api_server_url> <ca_cert_b64>), got %d in: %q", len(fields), left)
		}
		mountRole := fields[0]
		namespace := fields[1]
		apiServerURL := fields[2]
		caCertB64 := fields[3]

		mountPath, roleName, ok := strings.Cut(mountRole, "/")
		if !ok {
			githubactions.Fatalf("kube_secrets entry: expected <mount>/<role>, got %q", mountRole)
		}

		kubeCreds, err := client.Secrets.KubernetesGenerateCredentials(ctx, roleName,
			schema.KubernetesGenerateCredentialsRequest{
				KubernetesNamespace: namespace,
			},
			vault.WithToken(token),
			vault.WithMountPath(mountPath),
		)
		if err != nil {
			githubactions.Fatalf("Failed to generate Kubernetes credentials for %s/%s: %v", mountPath, roleName, err)
		}

		saToken, _ := kubeCreds.Data["service_account_token"].(string)
		saName, _ := kubeCreds.Data["service_account_name"].(string)
		saNamespace, _ := kubeCreds.Data["service_account_namespace"].(string)
		leaseID := kubeCreds.LeaseID

		githubactions.AddMask(saToken)
		githubactions.Infof("=> Kubernetes credentials generated (context: %s, lease_id: %s, sa: %s/%s)",
			contextName, leaseID, saNamespace, saName)

		if i == 0 {
			cfg.CurrentContext = contextName
		}

		cfg.Clusters = append(cfg.Clusters, kubeClusterEntry{
			Name: contextName,
			Cluster: kubeClusterData{
				Server:                   apiServerURL,
				CertificateAuthorityData: caCertB64,
			},
		})
		cfg.Users = append(cfg.Users, kubeUserEntry{
			Name: contextName,
			User: kubeUserData{Token: saToken},
		})
		cfg.Contexts = append(cfg.Contexts, kubeContextEntry{
			Name: contextName,
			Context: kubeContextData{
				Cluster:   contextName,
				User:      contextName,
				Namespace: namespace,
			},
		})
	}

	kubeYAML, err := yaml.Marshal(cfg)
	if err != nil {
		githubactions.Fatalf("Failed to marshal kubeconfig: %v", err)
	}

	runnerTemp := os.Getenv("RUNNER_TEMP")
	if runnerTemp == "" {
		runnerTemp = os.TempDir()
	}
	kubeconfigPath := filepath.Join(runnerTemp, fmt.Sprintf("vault-action-kubeconfig-%d.yaml", rand.Int63()))

	if err := os.WriteFile(kubeconfigPath, kubeYAML, 0600); err != nil {
		githubactions.Fatalf("Failed to write kubeconfig to %s: %v", kubeconfigPath, err)
	}

	githubactions.Infof("=> kubeconfig written to %s", kubeconfigPath)
	githubactions.SetEnv("KUBECONFIG", kubeconfigPath)
	githubactions.SetOutput("kubeconfig", kubeconfigPath)
}
