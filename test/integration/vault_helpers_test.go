package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// startVaultDev starts a Vault dev server on a random port and returns
// its address and root token. It registers a cleanup function to kill
// the server when the test finishes. If the vault binary is not found,
// the test is skipped.
func startVaultDev(t *testing.T) (addr, rootToken string) {
	t.Helper()

	vaultBin := os.Getenv("VAULT_BIN")
	if vaultBin == "" {
		var err error
		vaultBin, err = exec.LookPath("vault")
		if err != nil {
			t.Skip("vault binary not found; set VAULT_BIN or install vault")
		}
	}

	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("finding free port: %v", err)
	}
	listenAddr := ln.Addr().String()
	ln.Close()

	// Start vault dev server with a pipe for stdout.
	cmd := exec.Command(vaultBin, "server", "-dev",
		"-dev-listen-address="+listenAddr,
		"-dev-no-store-token",
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout pipe

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting vault dev server: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	addr = "http://" + listenAddr

	// Read output in a goroutine, protected by a mutex.
	var mu sync.Mutex
	var output strings.Builder
	tokenCh := make(chan string, 1)

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			output.WriteString(line)
			output.WriteByte('\n')
			mu.Unlock()
			if strings.Contains(line, "Root Token:") {
				parts := strings.SplitN(line, "Root Token:", 2)
				if len(parts) == 2 {
					select {
					case tokenCh <- strings.TrimSpace(parts[1]):
					default:
					}
				}
			}
		}
	}()

	// Wait for root token.
	deadline := time.Now().Add(15 * time.Second)
	select {
	case rootToken = <-tokenCh:
	case <-time.After(time.Until(deadline)):
		mu.Lock()
		out := output.String()
		mu.Unlock()
		t.Fatalf("vault dev server did not emit root token within 15s; output:\n%s", out)
	}

	// Wait for health endpoint.
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr + "/v1/sys/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return addr, rootToken
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	mu.Lock()
	out := output.String()
	mu.Unlock()
	t.Fatalf("vault dev server failed to become healthy within 15s; output:\n%s", out)
	return "", ""
}

// vaultRequest makes an HTTP request to the Vault API and returns the
// response body as a map. It fails the test on error.
func vaultRequest(t *testing.T, addr, token, method, path string, body any) map[string]any {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshaling request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := addr + "/v1/" + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("X-Vault-Token", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("vault request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Not all responses have JSON bodies (e.g. 204 No Content).
		result = nil
	}

	if resp.StatusCode >= 400 {
		t.Fatalf("vault request %s %s returned %d: %v", method, path, resp.StatusCode, result)
	}

	return result
}

// vaultRequestNoFail is like vaultRequest but returns nil on error instead
// of failing the test.
func vaultRequestNoFail(t *testing.T, addr, token, method, path string, body any) map[string]any {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil
		}
		reqBody = bytes.NewReader(data)
	}

	url := addr + "/v1/" + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil
	}
	req.Header.Set("X-Vault-Token", token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

// enableUserpass enables the userpass auth backend on the Vault server.
func enableUserpass(t *testing.T, addr, token string) {
	t.Helper()
	vaultRequest(t, addr, token, http.MethodPost, "sys/auth/userpass", map[string]any{
		"type": "userpass",
	})
}

// createUser creates a user in the userpass auth backend with full KV access.
func createUser(t *testing.T, addr, token, username, password string) {
	t.Helper()

	// Create a policy granting full access to the KV secrets engine.
	grantKVAccess(t, addr, token, "zhi-test-policy")

	// Create the user with the policy attached.
	vaultRequest(t, addr, token, http.MethodPost, "auth/userpass/users/"+username, map[string]any{
		"password":       password,
		"token_policies": "zhi-test-policy",
	})
}

// grantKVAccess creates a Vault policy that grants full access to the
// KV v2 secrets engine used by zhi integration tests.
func grantKVAccess(t *testing.T, addr, token, policyName string) {
	t.Helper()
	policy := `
path "secret/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "secret/data/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "secret/metadata/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "secret/destroy/*" {
  capabilities = ["create", "update"]
}
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
`
	vaultRequest(t, addr, token, http.MethodPut, "sys/policies/acl/"+policyName, map[string]any{
		"policy": policy,
	})
}

// enableAppRole enables the approle auth backend on the Vault server.
func enableAppRole(t *testing.T, addr, token string) {
	t.Helper()
	vaultRequest(t, addr, token, http.MethodPost, "sys/auth/approle", map[string]any{
		"type": "approle",
	})
}

// createAppRole creates an AppRole role and returns its role_id and secret_id.
func createAppRole(t *testing.T, addr, token, roleName string) (roleID, secretID string) {
	t.Helper()

	// Ensure the policy exists.
	grantKVAccess(t, addr, token, "zhi-test-policy")

	// Create the role with the policy.
	vaultRequest(t, addr, token, http.MethodPost, "auth/approle/role/"+roleName, map[string]any{
		"token_ttl":      "1h",
		"token_policies": "zhi-test-policy",
	})

	// Read role_id.
	roleResp := vaultRequest(t, addr, token, http.MethodGet, "auth/approle/role/"+roleName+"/role-id", nil)
	data, _ := roleResp["data"].(map[string]any)
	roleID, _ = data["role_id"].(string)
	if roleID == "" {
		t.Fatal("failed to get role_id")
	}

	// Generate secret_id.
	secretResp := vaultRequest(t, addr, token, http.MethodPost, "auth/approle/role/"+roleName+"/secret-id", nil)
	data, _ = secretResp["data"].(map[string]any)
	secretID, _ = data["secret_id"].(string)
	if secretID == "" {
		t.Fatal("failed to get secret_id")
	}

	return roleID, secretID
}

// createShortTTLToken creates a Vault token with a short TTL and returns it.
func createShortTTLToken(t *testing.T, addr, rootToken, ttl string) string {
	t.Helper()

	resp := vaultRequest(t, addr, rootToken, http.MethodPost, "auth/token/create", map[string]any{
		"ttl":       ttl,
		"renewable": false,
	})
	auth, _ := resp["auth"].(map[string]any)
	token, _ := auth["client_token"].(string)
	if token == "" {
		t.Fatal("failed to create short-TTL token")
	}
	return token
}
