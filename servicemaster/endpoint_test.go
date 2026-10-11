package servicemaster

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func setTestAWSEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{
		"AWS_REGION":                "us-east-1",
		"AWS_ACCESS_KEY_ID":         "test",
		"AWS_SECRET_ACCESS_KEY":     "test",
		"AWS_EC2_METADATA_DISABLED": "true",
	} {
		key := key
		old, existed := os.LookupEnv(key)
		if err := os.Setenv(key, value); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
	}
}

func TestLoadAWSConfigCustomEndpoint(t *testing.T) {
	setTestAWSEnv(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if err := r.ParseForm(); err != nil || r.Form.Get("Action") != "GetCallerIdentity" {
			t.Errorf("unexpected STS request: %v, %v", r.Form, err)
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Account>123456789012</Account></GetCallerIdentityResult></GetCallerIdentityResponse>`))
	}))
	defer server.Close()

	cfg, err := LoadAWSConfig(context.Background(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := cfg.EndpointResolver.ResolveEndpoint("sts", cfg.Region)
	if err != nil || endpoint.URL != server.URL || !endpoint.HostnameImmutable || endpoint.Source != aws.EndpointSourceCustom {
		t.Fatalf("unexpected custom endpoint: %+v, %v", endpoint, err)
	}
	if _, err := sts.NewFromConfig(cfg).GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{}); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("got %d STS requests, want 1", requests)
	}
}

func TestLoadAWSConfigDefaultAndInvalidEndpoint(t *testing.T) {
	setTestAWSEnv(t)
	cfg, err := LoadAWSConfig(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EndpointResolver != nil {
		t.Fatal("default configuration unexpectedly overrides AWS endpoints")
	}

	for _, endpoint := range []string{"localhost:4566", "ftp://localhost:4566", "http://", "http://:4566", "http://user:pass@localhost", "http://localhost#fragment"} {
		_, err := LoadAWSConfig(context.Background(), endpoint)
		if err == nil || !strings.Contains(err.Error(), "invalid endpoint URL") {
			t.Errorf("endpoint %q: expected validation error, got %v", endpoint, err)
		}
	}
}
