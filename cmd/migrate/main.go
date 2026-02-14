// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/noldarim/noldarim/internal/config"
	"github.com/noldarim/noldarim/internal/orchestrator/database"
)

func main() {
	// Load configuration
	cfg, err := config.NewConfig("config.yaml")
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create database connection
	db, err := database.NewGormDB(&cfg.Database)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Println("🚀 Starting database migration...")
	fmt.Printf("Database: %s\n", cfg.Database.GetDSN())

	// Run migrations
	if err := db.AutoMigrate(); err != nil {
		fmt.Printf("❌ Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Database migration completed successfully!")

	// Validate schema to confirm everything is correct
	if err := db.ValidateSchema(); err != nil {
		fmt.Printf("⚠️  Warning: Schema validation failed after migration: %v\n", err)
		fmt.Println("This might indicate a problem with the migration or model definitions.")
		os.Exit(1)
	}

	fmt.Println("✅ Schema validation passed - database is ready to use!")
}
