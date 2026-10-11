package servicestructs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func TestGetServicesUsesCustomEndpoint(t *testing.T) {
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

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if err := r.ParseForm(); err != nil || r.Form.Get("Action") != "ListUsers" {
			t.Errorf("unexpected IAM request: %v, %v", r.Form, err)
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<ListUsersResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><ListUsersResult><Users/></ListUsersResult></ListUsersResponse>`))
	}))
	defer server.Close()

	for _, service := range GetServices(server.URL) {
		if service.SvcName == "iam" {
			client := service.Svc.(*iam.Client)
			if _, err := client.ListUsers(context.Background(), &iam.ListUsersInput{}); err != nil {
				t.Fatal(err)
			}
			if requests != 1 {
				t.Fatalf("got %d IAM requests, want 1", requests)
			}
			return
		}
	}
	t.Fatal("IAM service is missing")
}
