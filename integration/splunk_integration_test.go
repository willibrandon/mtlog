//go:build integration
// +build integration

package integration

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/willibrandon/mtlog"
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/sinks"
)

func TestSplunkIntegration(t *testing.T) {
	splunkURL := os.Getenv("SPLUNK_URL")
	if splunkURL == "" {
		splunkURL = "https://localhost:8088"
	}

	splunkToken := os.Getenv("SPLUNK_TOKEN")
	if splunkToken == "" {
		t.Fatal("SPLUNK_TOKEN environment variable must be set")
	}

	// Check if Splunk is available
	if !isSplunkAvailable(splunkURL, splunkToken) {
		t.Skip("Splunk is not available, skipping integration test")
	}

	// Create a unique test ID to identify our events
	testID := fmt.Sprintf("test-%d", time.Now().UnixNano())

	// Create logger with Splunk sink
	splunkSink, err := sinks.NewSplunkSink(splunkURL, splunkToken,
		sinks.WithSplunkIndex("main"),
		sinks.WithSplunkSourceType("mtlog-test"),
		sinks.WithSplunkBatchSize(10),
		sinks.WithSplunkBatchTimeout(1*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create Splunk sink: %v", err)
	}

	log := mtlog.New(
		mtlog.WithSink(splunkSink),
		mtlog.WithMinimumLevel(core.VerboseLevel),
		mtlog.WithMachineName(),
		mtlog.WithProperty("TestRun", testID),
	)

	// Log various events
	log.Verbose("Verbose message {TestId}", testID)
	log.Debug("Debug message {TestId} with {Value}", testID, 42)
	log.Information("Information message {TestId}", testID)
	log.Warning("Warning message {TestId} with {Count}", testID, 5)

	// Log structured data
	order := struct {
		ID    string
		Total float64
		Items int
	}{
		ID:    "ORD-123",
		Total: 99.99,
		Items: 3,
	}
	log.Information("Order processed: {@Order} for test {TestId}", order, testID)

	// Log with error
	err = fmt.Errorf("sample error for testing")
	log.Error("Error occurred in test {TestId}: {Error}", testID, err)

	// Force flush by closing the sink
	splunkSink.Close()

	// Wait for Splunk to index the events
	time.Sleep(5 * time.Second)

	// Query Splunk to verify our events
	events, err := querySplunkEvents(splunkURL, splunkToken, testID)
	if err != nil {
		t.Fatalf("Failed to query Splunk events: %v", err)
	}

	// Filter events to only include the current test run
	var testEvents []map[string]interface{}
	for _, event := range events {
		if testRun, ok := event["TestRun"]; ok && testRun == testID {
			testEvents = append(testEvents, event)
		}
	}

	// Verify we got the expected number of events from this test run
	if len(testEvents) != 6 {
		t.Errorf("Expected 6 events, got %d", len(testEvents))
		for i, evt := range testEvents {
			t.Logf("Event %d: %+v", i, evt)
		}
	}

	// Verify event structure
	messageCounts := make(map[string]int)

	for _, event := range testEvents {
		// Check required Splunk fields
		if _, ok := event["_time"]; !ok {
			t.Error("Missing _time field")
		}
		if rawMsg, ok := event["_raw"]; !ok {
			t.Error("Missing _raw field")
		} else {
			// Count message types from _raw content
			raw := rawMsg.(string)
			switch {
			case strings.Contains(raw, "Verbose message"):
				messageCounts["Verbose"]++
			case strings.Contains(raw, "Debug message"):
				messageCounts["Debug"]++
			case strings.Contains(raw, "Information message"):
				messageCounts["Information"]++
			case strings.Contains(raw, "Warning message"):
				messageCounts["Warning"]++
			case strings.Contains(raw, "Error occurred"):
				messageCounts["Error"]++
			case strings.Contains(raw, "Order processed"):
				messageCounts["Information"]++
			}
		}

		// Check our test properties (these should be indexed as fields)
		if testRun, ok := event["TestRun"]; !ok || testRun != testID {
			t.Error("Missing or incorrect TestRun property")
		}
		if testIdField, ok := event["TestId"]; !ok || testIdField != testID {
			t.Error("Missing or incorrect TestId property")
		}
	}

	// Verify we got events at different levels based on message content
	expectedMessages := map[string]int{
		"Verbose":     1,
		"Debug":       1,
		"Information": 2, // One direct info message + one order processed
		"Warning":     1,
		"Error":       1,
	}

	for messageType, expected := range expectedMessages {
		if actual := messageCounts[messageType]; actual != expected {
			t.Errorf("Expected %d %s messages, got %d", expected, messageType, actual)
		}
	}

	// Verify we can find the error message in _raw content
	errorEventFound := false
	for _, event := range testEvents {
		if rawMsg, ok := event["_raw"].(string); ok {
			if strings.Contains(rawMsg, "Error occurred") && strings.Contains(rawMsg, "sample error for testing") {
				errorEventFound = true
				break
			}
		}
	}
	if !errorEventFound {
		t.Error("Error event with expected content not found")
	}
}

func TestSplunkBatching(t *testing.T) {
	splunkURL := os.Getenv("SPLUNK_URL")
	if splunkURL == "" {
		splunkURL = "https://localhost:8088"
	}

	splunkToken := os.Getenv("SPLUNK_TOKEN")
	if splunkToken == "" {
		splunkToken = "eb6baeef-eeb3-4a35-ab73-e17a12523b10"
	}

	if !isSplunkAvailable(splunkURL, splunkToken) {
		t.Skip("Splunk is not available, skipping integration test")
	}

	testID := fmt.Sprintf("batch-test-%d", time.Now().UnixNano())

	// Create logger with small batch size to test batching
	splunkSink, err := sinks.NewSplunkSink(splunkURL, splunkToken,
		sinks.WithSplunkIndex("main"),
		sinks.WithSplunkSourceType("mtlog-batch-test"),
		sinks.WithSplunkBatchSize(5),
		sinks.WithSplunkBatchTimeout(10*time.Second), // Long timeout to ensure batch size triggers
	)
	if err != nil {
		t.Fatalf("Failed to create Splunk sink: %v", err)
	}

	log := mtlog.New(
		mtlog.WithSink(splunkSink),
		mtlog.WithProperty("BatchTest", testID),
	)

	// Log exactly 5 events to trigger a batch
	for i := 0; i < 5; i++ {
		log.Information("Batch event {Index} for {TestId}", i, testID)
	}

	// Wait for batch to be sent
	time.Sleep(2 * time.Second)

	// Query and verify
	events, err := querySplunkEvents(splunkURL, splunkToken, testID)
	if err != nil {
		t.Fatalf("Failed to query Splunk events: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("Expected 5 events from first batch, got %d", len(events))
	}

	// Log 3 more events (won't trigger batch by size)
	for i := 5; i < 8; i++ {
		log.Information("Batch event {Index} for {TestId}", i, testID)
	}

	// Close to force flush
	splunkSink.Close()

	time.Sleep(2 * time.Second)

	// Query again
	events, err = querySplunkEvents(splunkURL, splunkToken, testID)
	if err != nil {
		t.Fatalf("Failed to query Splunk events: %v", err)
	}

	if len(events) != 8 {
		t.Errorf("Expected 8 total events, got %d", len(events))
	}
}

func TestSplunkCustomFields(t *testing.T) {
	splunkURL := os.Getenv("SPLUNK_URL")
	if splunkURL == "" {
		splunkURL = "https://localhost:8088"
	}

	splunkToken := os.Getenv("SPLUNK_TOKEN")
	if splunkToken == "" {
		splunkToken = "eb6baeef-eeb3-4a35-ab73-e17a12523b10"
	}

	if !isSplunkAvailable(splunkURL, splunkToken) {
		t.Skip("Splunk is not available, skipping integration test")
	}

	testID := fmt.Sprintf("fields-test-%d", time.Now().UnixNano())

	// Create logger with custom metadata
	splunkSink, err := sinks.NewSplunkSink(splunkURL, splunkToken,
		sinks.WithSplunkIndex("main"),
		sinks.WithSplunkSource("mtlog-test"),
		sinks.WithSplunkSourceType("json"),
		sinks.WithSplunkHost("test-host"),
		sinks.WithSplunkBatchSize(1),
	)
	if err != nil {
		t.Fatalf("Failed to create Splunk sink: %v", err)
	}

	log := mtlog.New(
		mtlog.WithSink(splunkSink),
		mtlog.WithProperty("Environment", "test"),
		mtlog.WithProperty("Service", "mtlog-test"),
		mtlog.WithProperty("Version", "1.0.0"),
		mtlog.WithProperty("TestId", testID),
	)

	// Log event with many properties using ForContext
	contextLog := log.ForContext("user.id", "user-123").
		ForContext("user.name", "John Doe").
		ForContext("transaction.id", "txn-456").
		ForContext("transaction.amount", 123.45).
		ForContext("tags", []string{"test", "integration", "splunk"})
	
	contextLog.Information("Test event with custom fields")

	// Close and wait
	splunkSink.Close()
	time.Sleep(2 * time.Second)

	// Query and verify
	events, err := querySplunkEvents(splunkURL, splunkToken, testID)
	if err != nil {
		t.Fatalf("Failed to query Splunk events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	event := events[0]

	// Verify metadata fields
	if src, ok := event["source"]; !ok || src != "mtlog-test" {
		t.Errorf("Missing or incorrect source field: %v", src)
	}
	if st, ok := event["sourcetype"]; !ok || st != "_json" {
		t.Errorf("Missing or incorrect sourcetype field: %v", st)
	}
	if host, ok := event["host"]; !ok || host != "test-host" {
		t.Errorf("Missing or incorrect host field: %v", host)
	}

	// Verify custom fields (field names with dots are converted to underscores)
	expectedFields := map[string]interface{}{
		"Environment":        "test",
		"Service":            "mtlog-test",
		"Version":            "1.0.0",
		"TestId":             testID,
		"user_id":            "user-123",
		"user_name":          "John Doe",
		"transaction_id":     "txn-456",
		"transaction_amount": 123.45,
	}

	for field, expected := range expectedFields {
		if actual, ok := event[field]; !ok {
			t.Errorf("Missing field %s", field)
		} else {
			// Splunk spath returns arrays, so we need to extract the first element
			var actualValue interface{}
			if actualArray, isArray := actual.([]interface{}); isArray && len(actualArray) > 0 {
				actualValue = actualArray[0]
			} else {
				actualValue = actual
			}
			
			if field == "transaction_amount" {
				// Handle float comparison - Splunk might return as string or different float type
				expectedFloat := expected.(float64)
				if actualFloat, ok := actualValue.(float64); ok {
					if actualFloat != expectedFloat {
						t.Errorf("Field %s: expected %v, got %v", field, expected, actualValue)
					}
				} else if actualFloat32, ok := actualValue.(float32); ok {
					if float64(actualFloat32) != expectedFloat {
						t.Errorf("Field %s: expected %v, got %v", field, expected, actualValue)
					}
				} else if actualStr, ok := actualValue.(string); ok {
					// Try to parse as float from string
					if actualStr != "123.45" { // Just check string value directly
						t.Errorf("Field %s: expected %v, got %v", field, expected, actualValue)
					}
				} else {
					t.Errorf("Field %s: expected float %v, got %v of type %T", field, expected, actualValue, actualValue)
				}
			} else if actualValue != expected {
				t.Errorf("Field %s: expected %v, got %v", field, expected, actualValue)
			}
		}
	}
}

// Helper functions

func isSplunkAvailable(splunkURL, token string) bool {
	healthURL := strings.Replace(splunkURL, "/services/collector", "/services/collector/health", 1)
	if !strings.Contains(healthURL, "/services/collector/health") {
		healthURL = strings.TrimRight(splunkURL, "/") + "/services/collector/health"
	}

	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Splunk "+token)

	// Use client with relaxed TLS for self-signed certs
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func querySplunkEvents(splunkURL, token, testID string) ([]map[string]interface{}, error) {
	// Use Splunk management API with admin credentials for searching
	// Build the management API URL
	searchURL := strings.Replace(splunkURL, ":8088", ":8089", 1)

	// Ensure we have the correct base URL for management API
	if strings.Contains(searchURL, "/services/collector") {
		searchURL = strings.Replace(searchURL, "/services/collector", "", 1)
	}
	searchURL = strings.TrimRight(searchURL, "/") + "/services/search/jobs"

	// Build search query with JSON extraction for custom fields test
	var query string
	if strings.Contains(testID, "fields-test-") {
		// For fields test, search the raw JSON data and extract fields
		query = fmt.Sprintf(`search index=main TestId="%s" | spath input=_raw | fields *`, testID)
	} else {
		// Normal search for other tests
		query = fmt.Sprintf(`search index=main (TestId="%s" OR TestRun="%s" OR BatchTest="%s" OR DataStreamTest="%s" OR AuthTest="%s")`,
			testID, testID, testID, testID, testID)
	}

	// URL encode the parameters for job creation
	params := fmt.Sprintf("search=%s&output_mode=json&earliest_time=-5m&latest_time=now&exec_mode=oneshot",
		url.QueryEscape(query))

	req, err := http.NewRequest("POST", searchURL, strings.NewReader(params))
	if err != nil {
		return nil, err
	}

	// Use admin credentials for search API
	req.SetBasicAuth("admin", "changeme")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use client with relaxed TLS
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("splunk search failed with status %d: %s", resp.StatusCode, body)
	}

	// Parse the search results
	var searchResult struct {
		Results []map[string]interface{} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	return searchResult.Results, nil
}
