package github

import (
	"context"
	"driftive/pkg/config"
	"driftive/pkg/gh"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	githubapi "github.com/google/go-github/v88/github"
)

func TestGetChangedFilesFollowsPagination(t *testing.T) {
	var pages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", "<"+"http://"+r.Host+r.URL.Path+"?page=2&per_page=100>; rel=\"next\"")
			_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": "first.txt"}})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{"filename": "second.txt"}})
	}))
	defer server.Close()

	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
	client, err := githubapi.NewClient(githubapi.WithHTTPClient(&http.Client{Transport: transport}))
	if err != nil {
		t.Fatal(err)
	}
	ops := &GHOps{
		config: &config.DriftiveConfig{
			GithubContext: &gh.GithubActionContext{
				Repository:      "owner/repo",
				RepositoryOwner: "owner",
			},
		},
		ghClient: client,
	}

	files, err := ops.GetChangedFiles(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"first.txt", "second.txt"}) {
		t.Fatalf("files = %#v", files)
	}
	if !reflect.DeepEqual(pages, []string{"1", "2"}) {
		t.Fatalf("pages = %#v", pages)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
