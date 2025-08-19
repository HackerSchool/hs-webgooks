package main

import (
	"fmt"
	"io"
	"net/http"
)

// MockAPIServer represents a simple mock API server for testing
type MockAPIServer struct {
	port string
}

// NewMockAPIServer creates a new mock API server
func NewMockAPIServer(port string) *MockAPIServer {
	return &MockAPIServer{port: port}
}

// Start starts the mock API server
func (m *MockAPIServer) Start() {
	http.HandleFunc("/tasks", m.handleTasks)
	
	go func() {
		fmt.Printf("🚀 Mock API server running on port %s\n", m.port)
		fmt.Printf("📝 POST to http://localhost:%s/tasks to test\n", m.port)
		if err := http.ListenAndServe(":"+m.port, nil); err != nil {
			fmt.Printf("❌ Mock API server failed: %v\n", err)
		}
	}()
}

// handleTasks handles POST requests to /tasks endpoint
func (m *MockAPIServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Log what we received
	fmt.Printf("📨 Mock API received POST to /tasks:\n")
	fmt.Printf("   Headers: %v\n", r.Header)
	fmt.Printf("   Body: %s\n", string(body))

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := `{
		"status": "success",
		"message": "Task completion recorded",
		"points_awarded": 30,
		"received_data": ` + string(body) + `
	}`
	w.Write([]byte(response))
}
