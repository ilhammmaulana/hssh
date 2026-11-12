package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndSaveLabels(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hssh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configContent := `# Global options
ServerAliveInterval 60

# label: prod,api
Host production-api
    HostName 10.0.0.1
    User ubuntu
    Port 2222
    IdentityFile ~/.ssh/id_rsa

# tags: staging,web
Host staging-web
    HostName 10.0.0.2
    User deploy
    Port 22

# env: dev
Host local-dev
    HostName 127.0.0.1

# Host with label defined INSIDE/AFTER host line
Host internal-db
    HostName 10.0.0.3
    # label: database,cluster
    User postgres

# Host with OpenSSH Tag directive
Host tagged-redis
    HostName 10.0.0.4
    Tag cache,redis
    User redis

# Host with OpenSSH Label directive
Host labeled-queue
    HostName 10.0.0.5
    Label rabbitmq
    User queue

# Host with space-separated comment and extra spaces
#   label worker,background
Host worker-job
    HostName 10.0.0.6
`
	testConfigFile := filepath.Join(tempDir, "config")
	if err := os.WriteFile(testConfigFile, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadSSHConfigFile(testConfigFile)
	if err != nil {
		t.Fatalf("LoadSSHConfigFile failed: %v", err)
	}

	if len(cfg.Entries) != 7 {
		t.Fatalf("Expected 7 entries, got %d", len(cfg.Entries))
	}

	// 1. Label before Host
	prod := cfg.Find("production-api")
	if prod == nil || prod.Tags != "prod,api" {
		t.Errorf("production-api tags mismatch: got %v, expected 'prod,api'", prod)
	}

	// 2. Tags before Host
	staging := cfg.Find("staging-web")
	if staging == nil || staging.Tags != "staging,web" {
		t.Errorf("staging-web tags mismatch: got %v, expected 'staging,web'", staging)
	}

	// 3. Env before Host
	dev := cfg.Find("local-dev")
	if dev == nil || dev.Tags != "dev" {
		t.Errorf("local-dev tags mismatch: got %v, expected 'dev'", dev)
	}

	// 4. Label INSIDE/AFTER Host line
	db := cfg.Find("internal-db")
	if db == nil || db.Tags != "database,cluster" {
		t.Errorf("internal-db tags mismatch: got %v, expected 'database,cluster'", db)
	}

	// 5. Tag directive inside Host
	redis := cfg.Find("tagged-redis")
	if redis == nil || redis.Tags != "cache,redis" {
		t.Errorf("tagged-redis tags mismatch: got %v, expected 'cache,redis'", redis)
	}

	// 6. Label directive inside Host
	queue := cfg.Find("labeled-queue")
	if queue == nil || queue.Tags != "rabbitmq" {
		t.Errorf("labeled-queue tags mismatch: got %v, expected 'rabbitmq'", queue)
	}

	// 7. Space-separated label comment with extra spaces
	worker := cfg.Find("worker-job")
	if worker == nil || worker.Tags != "worker,background" {
		t.Errorf("worker-job tags mismatch: got %v, expected 'worker,background'", worker)
	}

	// Test saving and re-parsing
	prod.Tags = "prod,cluster,v2"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	savedBytes, err := os.ReadFile(testConfigFile)
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}
	savedStr := string(savedBytes)

	if !strings.Contains(savedStr, "# label: prod,cluster,v2") {
		t.Errorf("Saved config does not contain updated label '# label: prod,cluster,v2':\n%s", savedStr)
	}

	// Reload saved config
	reloadedCfg, err := LoadSSHConfigFile(testConfigFile)
	if err != nil {
		t.Fatalf("Failed to reload saved config: %v", err)
	}
	reloadedProd := reloadedCfg.Find("production-api")
	if reloadedProd == nil || reloadedProd.Tags != "prod,cluster,v2" {
		t.Errorf("Reloaded prod tags mismatch: got %v, expected 'prod,cluster,v2'", reloadedProd)
	}
}
