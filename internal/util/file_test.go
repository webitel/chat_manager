package util

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchFileDetails(t *testing.T) {
	// Mock server to simulate HTTP HEAD request
	handler := http.NewServeMux()
	handler.HandleFunc("/testfile.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", "12345")
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Define the URL to test
	url := server.URL + "/testfile.jpg"

	// Call the function to test
	filename, mimetype, extension, size, err := FetchFileDetails(url)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	if filename != "testfile.jpg" {
		t.Errorf("Expected file name 'testfile.jpg', but got: %s", filename)
	}

	if mimetype != "image/jpeg" {
		t.Errorf("Expected mime type 'image/jpeg', but got: %s", mimetype)
	}

	if extension != ".jpg" {
		t.Errorf("Expected extension '.jpg', but got: %s", extension)
	}

	if size != 12345 {
		t.Errorf("Expected size 12345, but got: %d", size)
	}
}

func TestFetchFileDetailsFromHeaders(t *testing.T) {
	// Mock server to simulate HTTP HEAD request
	handler := http.NewServeMux()
	handler.HandleFunc("/some/path", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", "12345")
		w.Header().Set("Content-Disposition", "inline;filename=testfile")
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Define the URL to test
	url := server.URL + "/some/path"

	// Call the function to test
	filename, mimetype, extension, size, err := FetchFileDetails(url)

	// Assertions
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	if filename != "testfile" {
		t.Errorf("Expected file name 'testfile', but got: %s", filename)
	}

	if mimetype != "image/jpeg" {
		t.Errorf("Expected mime type 'image/jpeg', but got: %s", mimetype)
	}

	if extension != "" {
		t.Errorf("Expected extension '', but got: %s", extension)
	}

	if size != 12345 {
		t.Errorf("Expected size 12345, but got: %d", size)
	}
}

func TestFetchFileDetailsError(t *testing.T) {
	// Test case for a failed HTTP request (simulate error)

	// Invalid URL to trigger error
	invalidURL := "http://invalid-url/testfile.jpg"

	// Call the function to test
	_, _, _, _, err := FetchFileDetails(invalidURL)

	// Assertions
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}
