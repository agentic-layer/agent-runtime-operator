package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GetRequest sends a GET request and returns the response body, status code, and error.
// This function does not treat non-200 status codes as errors,
// allowing callers to explicitly check for specific status codes like 404.
func GetRequest(url string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	return executeRequest(req)
}

// PostRequest sends a POST request with a JSON payload and custom headers,
// returning the response body, status code, and error.
// This function does not treat non-200 status codes as errors.
func PostRequest(url string, payload any, headers map[string]string) ([]byte, int, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return executeRequest(req)
}

// executeRequest executes an HTTP request and returns the response body, status code, and error.
func executeRequest(req *http.Request) ([]byte, int, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}(resp.Body)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return bodyBytes, resp.StatusCode, nil
}
