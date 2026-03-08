package testdata

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

// User represents a user in the test API.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// TestAPIServer is a simple HTTP API for load testing.
type TestAPIServer struct {
	mu       sync.RWMutex
	users    map[int]User
	nextID   int
	server   *http.Server
	Addr     string
	listener net.Listener
}

// NewTestAPIServer creates a server with seed data.
func NewTestAPIServer() *TestAPIServer {
	s := &TestAPIServer{
		users:  make(map[int]User),
		nextID: 1,
	}

	// Seed data
	for _, name := range []string{"Alice", "Bob", "Charlie"} {
		s.users[s.nextID] = User{
			ID:    s.nextID,
			Name:  name,
			Email: fmt.Sprintf("%s@example.com", strings.ToLower(name)),
		}
		s.nextID++
	}

	return s
}

// Start listens on a random available port and serves requests.
func (s *TestAPIServer) Start() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	s.listener = ln
	s.Addr = ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/users", s.handleUsers)
	mux.HandleFunc("/users/", s.handleUserByID)
	mux.HandleFunc("/login", s.handleLogin)

	s.server = &http.Server{Handler: mux}
	go s.server.Serve(ln)
	return nil
}

// Stop shuts down the server.
func (s *TestAPIServer) Stop() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *TestAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *TestAPIServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		users := make([]User, 0, len(s.users))
		for _, u := range s.users {
			users = append(users, u)
		}
		s.mu.RUnlock()
		writeJSON(w, http.StatusOK, users)

	case http.MethodPost:
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
		s.mu.Lock()
		u.ID = s.nextID
		s.nextID++
		s.users[u.ID] = u
		s.mu.Unlock()
		writeJSON(w, http.StatusCreated, u)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *TestAPIServer) handleUserByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from /users/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/users/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing user id"})
		return
	}
	var id int
	if _, err := fmt.Sscanf(parts[0], "%d", &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		u, ok := s.users[id]
		s.mu.RUnlock()
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		writeJSON(w, http.StatusOK, u)

	case http.MethodDelete:
		s.mu.Lock()
		if _, ok := s.users[id]; !ok {
			s.mu.Unlock()
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		delete(s.users, id)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *TestAPIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	if body.Email == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password required"})
		return
	}

	// Simple check: any seeded user email with password "secret" works
	s.mu.RLock()
	var found bool
	for _, u := range s.users {
		if u.Email == body.Email {
			found = true
			break
		}
	}
	s.mu.RUnlock()

	if !found || body.Password != "secret" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": "test-jwt-token-123"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
