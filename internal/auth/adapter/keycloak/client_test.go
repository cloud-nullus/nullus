package keycloak

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKeycloakClient_RegisterOIDCClient_SendsExpectedRequest(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var tokenCalls int
	var receivedAuth string
	var receivedClientID string
	var receivedName string
	var receivedRedirectURIs []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			mu.Lock()
			tokenCalls++
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":300}`))
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/nullus/clients":
			receivedAuth = r.Header.Get("Authorization")
			var payload struct {
				ClientID     string   `json:"clientId"`
				Name         string   `json:"name"`
				RedirectURIs []string `json:"redirectUris"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			receivedClientID = payload.ClientID
			receivedName = payload.Name
			receivedRedirectURIs = payload.RedirectURIs
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	kc := NewKeycloakClient(server.URL, "nullus", "admin", "admin")
	err := kc.RegisterOIDCClient(context.Background(), "gitlab", []string{"https://gitlab.nullus.local/users/auth/openid_connect/callback"}, "GitLab CE")
	require.NoError(t, err)

	mu.Lock()
	require.Equal(t, 1, tokenCalls)
	mu.Unlock()
	require.Equal(t, "Bearer token-1", receivedAuth)
	require.Equal(t, "gitlab", receivedClientID)
	require.Equal(t, "GitLab CE", receivedName)
	require.Equal(t, []string{"https://gitlab.nullus.local/users/auth/openid_connect/callback"}, receivedRedirectURIs)
}

func TestKeycloakClient_DeleteOIDCClient_SendsDeleteForResolvedClientID(t *testing.T) {
	t.Parallel()

	var gotDeletePath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/nullus/clients" && r.URL.Query().Get("clientId") == "argocd":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"kc-internal-id"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/admin/realms/nullus/clients/kc-internal-id":
			gotDeletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	kc := NewKeycloakClient(server.URL, "nullus", "admin", "admin")
	err := kc.DeleteOIDCClient(context.Background(), "argocd")
	require.NoError(t, err)
	require.Equal(t, "/admin/realms/nullus/clients/kc-internal-id", gotDeletePath)
}

func TestKeycloakClient_GetToken_CachesAndRefreshesOnExpiry(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	tokenValues := []string{"token-1", "token-2"}
	var tokenCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/realms/master/protocol/openid-connect/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		mu.Lock()
		idx := tokenCalls
		tokenCalls++
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + tokenValues[idx] + `","expires_in":300}`))
	}))
	defer server.Close()

	kc := NewKeycloakClient(server.URL, "nullus", "admin", "admin")

	token1, err := kc.getToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-1", token1)

	token2, err := kc.getToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-1", token2)

	kc.mu.Lock()
	kc.tokenExpiry = time.Now().Add(-1 * time.Second)
	kc.mu.Unlock()

	token3, err := kc.getToken(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-2", token3)

	mu.Lock()
	require.Equal(t, 2, tokenCalls)
	mu.Unlock()
}

func TestKeycloakClient_ListClients_ReturnsParsedClients(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/nullus/clients":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"1","clientId":"gitlab","name":"GitLab CE"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	kc := NewKeycloakClient(server.URL, "nullus", "admin", "admin")
	clients, err := kc.ListClients(context.Background())
	require.NoError(t, err)
	require.Len(t, clients, 1)
	require.Equal(t, "1", clients[0].ID)
	require.Equal(t, "gitlab", clients[0].ClientID)
	require.Equal(t, "GitLab CE", clients[0].Name)
}

func TestSSOProvisioner_ProvisionSSO_UsesStepMapping(t *testing.T) {
	t.Parallel()

	var receivedPayload string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":300}`))
		case r.Method == http.MethodPost && r.URL.Path == "/admin/realms/nullus/clients":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			receivedPayload = string(body)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	kc := NewKeycloakClient(server.URL, "nullus", "admin", "admin")
	p := NewSSOProvisioner(kc)

	err := p.ProvisionSSO(context.Background(), "installing_grafana")
	require.NoError(t, err)
	require.Contains(t, receivedPayload, `"clientId":"grafana"`)
	require.Contains(t, receivedPayload, `"name":"Grafana"`)
	require.Contains(t, receivedPayload, `"https://grafana.nullus.local/login/generic_oauth"`)
}

func TestSSOProvisioner_DeprovisionSSO_DeletesMappedClient(t *testing.T) {
	t.Parallel()

	var deletedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/realms/master/protocol/openid-connect/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":300}`))
		case r.Method == http.MethodGet && r.URL.Path == "/admin/realms/nullus/clients" && r.URL.Query().Get("clientId") == "minio":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"minio-id","clientId":"minio"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/admin/realms/nullus/clients/minio-id":
			deletedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	kc := NewKeycloakClient(server.URL, "nullus", "admin", "admin")
	p := NewSSOProvisioner(kc)

	err := p.DeprovisionSSO(context.Background(), "installing_minio")
	require.NoError(t, err)
	require.Equal(t, "/admin/realms/nullus/clients/minio-id", deletedPath)
}

func TestSSOProvisioner_UnknownStep_ReturnsErrorOnProvision(t *testing.T) {
	t.Parallel()

	kc := NewKeycloakClient("http://127.0.0.1:1", "nullus", "admin", "admin")
	p := NewSSOProvisioner(kc)

	err := p.ProvisionSSO(context.Background(), "installing_unknown_tool")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown SSO tool")
	require.NoError(t, p.DeprovisionSSO(context.Background(), "installing_unknown_tool"))
}
