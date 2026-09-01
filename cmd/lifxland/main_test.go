package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPrintVersion(t *testing.T) {
	original := version
	version = "v1.2.3"
	t.Cleanup(func() { version = original })

	var output bytes.Buffer
	printVersion(&output)
	if got, want := output.String(), "lifxland v1.2.3\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

func TestValidateExposureRequiresTokenBeyondLoopback(t *testing.T) {
	allowed := []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"}
	for _, address := range allowed {
		if err := validateExposure(address, ""); err != nil {
			t.Errorf("validateExposure(%q) = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", ":8080", "192.168.1.2:8080"} {
		if err := validateExposure(address, ""); err == nil {
			t.Errorf("validateExposure(%q) returned no error", address)
		}
		if err := validateExposure(address, "secret"); err != nil {
			t.Errorf("validateExposure(%q, token) = %v", address, err)
		}
	}
}

func TestBearerAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := bearerAuth(next, "secret")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/devices", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", response.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/devices", nil)
	request.Header.Set("Authorization", "Bearer secret")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d", response.Code)
	}

	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("health status = %d", response.Code)
	}
}
