package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain_ExitsOnInvalidConfig(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestMain_ExitsOnInvalidConfigHelper")
	cmd.Env = withEnvOverrides(os.Environ(), map[string]string{
		"GO_LEAD_RUN_MAIN":       "1",
		"CUSTOMER_API_URL":       "",
		"CUSTOMER_API_TOKEN":     "",
		"CUSTOMER_PRODUCT_NAME":  "",
		"ENABLE_AUTH":            "",
		"SHARED_SECRET":          "",
	})

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit code, got success; output: %s", string(output))
	}

	if !strings.Contains(string(output), "Failed to load configuration") {
		t.Fatalf("expected failure message to mention configuration load; output: %s", string(output))
	}
}

func TestMain_ExitsOnInvalidConfigHelper(t *testing.T) {
	if os.Getenv("GO_LEAD_RUN_MAIN") != "1" {
		return
	}

	main()
}

func withEnvOverrides(base []string, overrides map[string]string) []string {
	filtered := make([]string, 0, len(base))
	for _, entry := range base {
		keep := true
		for key := range overrides {
			if strings.HasPrefix(entry, key+"=") {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, entry)
		}
	}

	for key, value := range overrides {
		filtered = append(filtered, key+"="+value)
	}

	return filtered
}
