package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ServiceProcess represents a running service process
type ServiceProcess struct {
	name    string
	cmd     *exec.Cmd
	port    int
	baseURL string
}

// TestEnv holds the test environment
type TestEnv struct {
	ctx          context.Context
	cancel       context.CancelFunc
	pgContainer  *postgres.PostgresContainer
	pool         *pgxpool.Pool
	services map[string]*ServiceProcess
	dbHost   string
	dbPort       string
}

// SetupTestEnv creates a new test environment with PostgreSQL container
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("kensan_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		cancel()
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	// Get host and port
	host, err := pgContainer.Host(ctx)
	if err != nil {
		cancel()
		t.Fatalf("Failed to get postgres host: %v", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		cancel()
		t.Fatalf("Failed to get postgres port: %v", err)
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cancel()
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Connect to database
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		cancel()
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Run migrations
	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		cancel()
		t.Fatalf("Failed to run migrations: %v", err)
	}

	env := &TestEnv{
		ctx:         ctx,
		cancel:      cancel,
		pgContainer: pgContainer,
		pool:        pool,
		services:    make(map[string]*ServiceProcess),
		dbHost:      host,
		dbPort:      port.Port(),
	}

	return env
}

// StartService builds and starts a service
func (e *TestEnv) StartService(t *testing.T, serviceName string, port int) {
	t.Helper()

	// Find backend directory
	backendDir := findBackendDir()
	if backendDir == "" {
		t.Fatal("Backend directory not found")
	}

	// Build service
	// Handle both "user" and "user-service" formats
	shortName := serviceName
	svcName := serviceName

	if len(serviceName) > 8 && serviceName[len(serviceName)-8:] == "-service" {
		// Input is "user-service" format
		shortName = serviceName[:len(serviceName)-8]
		svcName = serviceName
	} else {
		// Input is "user" format
		shortName = serviceName
		svcName = serviceName + "-service"
	}

	binPath := filepath.Join(backendDir, "bin", svcName)
	buildCmd := exec.Command("go", "build", "-o", binPath, fmt.Sprintf("./services/%s/cmd", shortName))
	buildCmd.Dir = backendDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build %s: %v", serviceName, err)
	}

	// Set environment variables
	env := []string{
		fmt.Sprintf("SERVER_PORT=%d", port),
		fmt.Sprintf("DB_HOST=%s", e.dbHost),
		fmt.Sprintf("DB_PORT=%s", e.dbPort),
		"DB_USER=test",
		"DB_PASSWORD=test",
		"DB_NAME=kensan_test",
		"DB_SSLMODE=disable",
		"JWT_SECRET=test-secret-key-for-e2e-tests",
		"SERVER_ENV=test",
	}

	// Start service
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start %s: %v", serviceName, err)
	}

	baseURL := fmt.Sprintf("http://localhost:%d", port)
	e.services[serviceName] = &ServiceProcess{
		name:    serviceName,
		cmd:     cmd,
		port:    port,
		baseURL: baseURL,
	}

	// Wait for service to be ready
	if err := waitForService(baseURL, 30*time.Second); err != nil {
		cmd.Process.Kill()
		t.Fatalf("Service %s failed to start: %v", serviceName, err)
	}

	t.Logf("Service %s started on port %d", serviceName, port)
}

// ServiceURL returns the base URL for a service
func (e *TestEnv) ServiceURL(serviceName string) string {
	if svc, ok := e.services[serviceName]; ok {
		return svc.baseURL
	}
	return ""
}

// Cleanup cleans up the test environment
func (e *TestEnv) Cleanup(t *testing.T) {
	t.Helper()

	// Stop all services
	for name, svc := range e.services {
		if svc.cmd != nil && svc.cmd.Process != nil {
			if err := svc.cmd.Process.Kill(); err != nil {
				t.Logf("Failed to kill %s: %v", name, err)
			}
			svc.cmd.Wait()
		}
	}

	// Close database connection
	if e.pool != nil {
		e.pool.Close()
	}

	// Terminate PostgreSQL container
	if e.pgContainer != nil {
		if err := e.pgContainer.Terminate(e.ctx); err != nil {
			t.Logf("Failed to terminate postgres container: %v", err)
		}
	}

	if e.cancel != nil {
		e.cancel()
	}
}

// CleanupDB cleans all data from the database
func (e *TestEnv) CleanupDB(t *testing.T) {
	t.Helper()

	tables := []string{
		"ai_review_reports",
		"diary_entries",
		"learning_records",
		"memos",
		"time_entries",
		"time_blocks",
		"running_timers",
		"task_tags",
		"tasks",
		"milestones",
		"goals",
		"tags",
		"user_settings",
		"users",
	}

	for _, table := range tables {
		_, err := e.pool.Exec(e.ctx, fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			t.Logf("Failed to clean table %s: %v", table, err)
		}
	}
}

// Context returns the test context
func (e *TestEnv) Context() context.Context {
	return e.ctx
}

// Pool returns the database connection pool
func (e *TestEnv) Pool() *pgxpool.Pool {
	return e.pool
}

// runMigrations runs database migrations
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir := findMigrationsDir()
	if migrationsDir == "" {
		return fmt.Errorf("migrations directory not found")
	}

	migrationFiles := []string{
		"001_init.sql",
		"002_master.sql",
	}

	for _, file := range migrationFiles {
		path := filepath.Join(migrationsDir, file)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	return nil
}

// findMigrationsDir finds the migrations directory
func findMigrationsDir() string {
	paths := []string{
		"../migrations-v2",
		"../../migrations-v2",
		"migrations-v2",
		"backend/migrations-v2",
	}

	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "001_init.sql")); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

	return ""
}

// findBackendDir finds the backend directory
func findBackendDir() string {
	paths := []string{
		"..",
		"../..",
		".",
		"backend",
	}

	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}

	return ""
}

// waitForService waits for a service to be ready
func waitForService(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	healthURL := baseURL + "/health"

	for time.Now().Before(deadline) {
		resp, err := httpGet(healthURL)
		if err == nil && resp.StatusCode == 200 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for service at %s", baseURL)
}

// httpGet makes a simple GET request
func httpGet(url string) (*httpResponse, error) {
	client := NewHTTPClient(nil)
	client.t = nil // Disable require assertions
	resp := client.GetRaw(url)
	return &httpResponse{StatusCode: resp.StatusCode}, nil
}

type httpResponse struct {
	StatusCode int
}
