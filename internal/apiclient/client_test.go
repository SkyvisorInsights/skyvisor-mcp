package apiclient

import "testing"

func TestNewRequiresSecureRemoteURLAndAccessToken(t *testing.T) {
	t.Parallel()
	const token = "01234567890123456789012345678901"
	if _, err := New("http://api.example.com", token); err == nil {
		t.Fatal("expected insecure remote URL to fail")
	}
	if _, err := New("https://api.example.com", ""); err == nil {
		t.Fatal("expected missing token to fail")
	}
	if _, err := New("http://127.0.0.1:8080", token); err != nil {
		t.Fatalf("loopback URL failed: %v", err)
	}
	if _, err := New("http://api.staging.svc.cluster.local:8080", token); err != nil {
		t.Fatalf("cluster-local URL failed: %v", err)
	}
}
