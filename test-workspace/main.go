package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Initialize authentication
	if err := initAuth(); err != nil {
		log.Fatal(err)
	}

	// Initialize database
	if err := initDB(); err != nil {
		log.Fatal(err)
	}

	// Setup API routes
	setupRoutes()

	// Start server
	fmt.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Global variable to store JWT secret
var jwtSecret []byte

// Global variable to store database connection pool
var dbPool *pgxpool.Pool

// Alice implements: JWT authentication initialization
// TODO: Load JWT secret from environment, validate configuration
func initAuth() error {
	// Load JWT secret from environment variable
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return errors.New("JWT_SECRET environment variable is not set")
	}

	// Validate secret length (minimum 32 characters for security)
	if len(secret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters long")
	}

	// Store secret in global variable
	jwtSecret = []byte(secret)
	log.Println("JWT authentication initialized successfully")
	return nil
}

// Alice implements: JWT token validation
// TODO: Parse JWT token, verify signature, check expiration
func authenticate(tokenString string) (bool, error) {
	// Parse the JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to parse token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return false, errors.New("invalid token")
	}

	// Verify claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, errors.New("invalid token claims")
	}

	// Check expiration time
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return false, errors.New("token has expired")
		}
	} else {
		return false, errors.New("token missing expiration claim")
	}

	return true, nil
}

// Bob implements: PostgreSQL connection pool initialization
// Loads DATABASE_URL from environment, configures connection pool settings,
// and verifies the database connection is healthy
func initDB() error {
	// Load DATABASE_URL from environment variable
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL environment variable is not set")
	}

	// Parse and create connection pool configuration
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse DATABASE_URL: %w", err)
	}

	// Configure connection pool settings
	config.MaxConns = 25                           // Maximum number of connections in the pool
	config.MinConns = 5                            // Minimum number of connections to maintain
	config.MaxConnLifetime = time.Hour             // Maximum connection lifetime
	config.MaxConnIdleTime = 30 * time.Minute      // Maximum idle time before closing connection
	config.HealthCheckPeriod = 1 * time.Minute     // Health check interval

	// Create the connection pool
	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection by pinging the database
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Store pool in global variable
	dbPool = pool
	log.Println("PostgreSQL connection pool initialized successfully")
	log.Printf("Pool config - MaxConns: %d, MinConns: %d", config.MaxConns, config.MinConns)
	return nil
}

// Bob implements: Database connection helper
// Returns the active connection pool and verifies its health
func connectDB() (*pgxpool.Pool, error) {
	// Check if pool is initialized
	if dbPool == nil {
		return nil, errors.New("database pool not initialized, call initDB() first")
	}

	// Verify pool health with a quick ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := dbPool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database health check failed: %w", err)
	}

	return dbPool, nil
}

// Charlie implements: RESTful API route setup
// Registers /api/users endpoint with authentication middleware
func setupRoutes() {
	http.HandleFunc("/api/users", handleAPI)
	http.HandleFunc("/api/users/", handleAPI) // For /api/users/{id} pattern
	log.Println("API routes registered: /api/users")
}

// Charlie implements: RESTful API handler
// Supports GET/POST/PUT/DELETE for user resources
// Integrates Alice's authenticate() and Bob's connectDB()
func handleAPI(w http.ResponseWriter, r *http.Request) {
	// Step 1: Authentication (Alice's implementation)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error":"Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	// Extract token from "Bearer <token>"
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	// Validate JWT token using Alice's authenticate()
	isValid, err := authenticate(token)
	if err != nil || !isValid {
		http.Error(w, fmt.Sprintf(`{"error":"Authentication failed: %v"}`, err), http.StatusUnauthorized)
		return
	}

	// Step 2: Database connection (Bob's implementation)
	pool, err := connectDB()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Database unavailable: %v"}`, err), http.StatusServiceUnavailable)
		return
	}

	// Set JSON response header
	w.Header().Set("Content-Type", "application/json")

	// Step 3: Route to appropriate handler based on HTTP method
	switch r.Method {
	case http.MethodGet:
		handleGetUsers(w, r, pool)
	case http.MethodPost:
		handleCreateUser(w, r, pool)
	case http.MethodPut:
		handleUpdateUser(w, r, pool)
	case http.MethodDelete:
		handleDeleteUser(w, r, pool)
	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// GET /api/users - List all users
// GET /api/users/{id} - Get specific user
func handleGetUsers(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	ctx := r.Context()

	// Check if requesting specific user by ID
	path := r.URL.Path
	if len(path) > len("/api/users/") {
		// Extract user ID from path
		userID := path[len("/api/users/"):]

		var id int
		var name, email string
		err := pool.QueryRow(ctx, "SELECT id, name, email FROM users WHERE id = $1", userID).Scan(&id, &name, &email)
		if err != nil {
			http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
			return
		}

		fmt.Fprintf(w, `{"id":%d,"name":"%s","email":"%s"}`, id, name, email)
		return
	}

	// List all users
	rows, err := pool.Query(ctx, "SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Query failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Write([]byte(`{"users":[`))
	first := true
	for rows.Next() {
		var id int
		var name, email string
		if err := rows.Scan(&id, &name, &email); err != nil {
			continue
		}

		if !first {
			w.Write([]byte(","))
		}
		first = false
		fmt.Fprintf(w, `{"id":%d,"name":"%s","email":"%s"}`, id, name, email)
	}
	w.Write([]byte(`]}`))
}

// POST /api/users - Create new user
func handleCreateUser(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	ctx := r.Context()

	// Parse form data or JSON body
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")

	if name == "" || email == "" {
		http.Error(w, `{"error":"Missing required fields: name, email"}`, http.StatusBadRequest)
		return
	}

	// Insert new user using Bob's parameterized query (SQL injection safe)
	var userID int
	err := pool.QueryRow(ctx, "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id", name, email).Scan(&userID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"id":%d,"name":"%s","email":"%s","message":"User created successfully"}`, userID, name, email)
}

// PUT /api/users/{id} - Update user
func handleUpdateUser(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	ctx := r.Context()

	// Extract user ID from path
	path := r.URL.Path
	if len(path) <= len("/api/users/") {
		http.Error(w, `{"error":"User ID required"}`, http.StatusBadRequest)
		return
	}
	userID := path[len("/api/users/"):]

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")

	if name == "" || email == "" {
		http.Error(w, `{"error":"Missing required fields: name, email"}`, http.StatusBadRequest)
		return
	}

	// Update user using parameterized query
	result, err := pool.Exec(ctx, "UPDATE users SET name = $1, email = $2 WHERE id = $3", name, email, userID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to update user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, `{"id":%s,"name":"%s","email":"%s","message":"User updated successfully"}`, userID, name, email)
}

// DELETE /api/users/{id} - Delete user
func handleDeleteUser(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool) {
	ctx := r.Context()

	// Extract user ID from path
	path := r.URL.Path
	if len(path) <= len("/api/users/") {
		http.Error(w, `{"error":"User ID required"}`, http.StatusBadRequest)
		return
	}
	userID := path[len("/api/users/"):]

	// Delete user using parameterized query
	result, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to delete user: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, `{"message":"User %s deleted successfully"}`, userID)
}
