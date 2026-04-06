package main

import (
	"context"
	"strconv"

	"github.com/sethvargo/go-githubactions"
)

// readInputWithFail is a helper function that reads an input variable and fails the action if it's empty.
func readInputWithFail(name string) string {
	githubactions.Infof("=> reading input: %s", name)
	value := githubactions.GetInput(name)
	if value == "" {
		githubactions.Fatalf("%s is required", name)
	}
	return value
}

// readInput is a helper function that reads an input variable and returns it (or an empty string if not provided).
func readInput(name string) string {
	githubactions.Infof("=> reading input: %s", name)
	return githubactions.GetInput(name)
}

// readBoolInputWithFail is a helper function that reads an input variable, parses it as a boolean, and fails the action if it's empty or cannot be parsed.
func readBoolInputWithFail(name string) bool {
	githubactions.Infof("=> reading input: %s", name)
	value := githubactions.GetInput(name)
	if value == "" {
		githubactions.Fatalf("%s is required", name)
	}
	parsedValue, err := strconv.ParseBool(value)
	if err != nil {
		githubactions.Fatalf("Failed to parse %s as boolean: %v", name, err)
	}
	return parsedValue
}

// readTokenWithFail is a helper function that gets an ID token for the specified claim and fails the action if it cannot be obtained.
func readTokenWithFail(ctx context.Context, claim string) string {
	token, err := githubactions.GetIDToken(ctx, claim)
	if err != nil {
		githubactions.Fatalf("Failed to get ID token for claim %q: %v", claim, err)
	}
	return token
}
