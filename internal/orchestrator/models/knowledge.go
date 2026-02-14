// Copyright (C) 2026 Noldarim
// SPDX-License-Identifier: AGPL-3.0-or-later

package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// UniversalType defines the meta-category of knowledge
type UniversalType string

const (
	TypeProblem    UniversalType = "PROBLEM"    // e.g., "State Synchronization"
	TypeStrategy   UniversalType = "STRATEGY"   // e.g., "Virtual DOM"
	TypeConcept    UniversalType = "CONCEPT"    // e.g., "Gravity", "Latency"
	TypeRole       UniversalType = "ROLE"       // e.g., "Frontend Engineer"
	TypeConstraint UniversalType = "CONSTRAINT" // e.g., "Must respond < 100ms"
	TypeDefinition UniversalType = "DEFINITION" // e.g., "What is a 'User'?"
)

// Universal represents a node in the Global Knowledge Graph (Plane A).
// These are timeless, abstract concepts.
type Universal struct {
	ID          string        `gorm:"primaryKey"` // URN: noldarim://global/concepts/auth
	Name        string        `gorm:"index"`
	Type        UniversalType `gorm:"index"`
	Definition  string        // Human readable definition/intent
	Description string        // Extended description

	// AI Searchability - Stored as JSON for SQLite compatibility for now,
	// but intended for pgvector later.
	Embedding JSONMap `gorm:"type:text"`

	// Schema Validation: If this Universal is a "Task", what fields does it need?
	// e.g. {"required": ["deadline", "assignee"]}
	SchemaDefinition JSONMap `gorm:"type:text"`

	// Versioning
	Version int

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Particular represents an instance of a Universal in a specific context (Plane B).
// These are time-bound, concrete manifestations.
type Particular struct {
	ID   string `gorm:"primaryKey"` // UUID
	Name string

	UniversalID string `gorm:"index"` // Pointer to the Abstract Concept (Is-A relation)
	Universal   Universal

	ContextID string `gorm:"index"` // Which "Garden" does this belong to? (e.g., ProjectID)

	// The actual data state (The "Matter")
	// e.g. {"status": "failed", "retry_count": 3, "file_path": "src/auth.go"}
	StatePayload JSONMap `gorm:"type:text"`

	// Health Metadata
	LastVerifiedAt  time.Time
	IntegrityScore  float64 // 0.0-1.0 (Is this instance adhering to its Universal?)
	VerificationLog string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Transformation records how Abstract Knowledge changed Concrete State (Plane C).
// This captures the "Causality" and "Reasoning" of the system.
type Transformation struct {
	ID        string    `gorm:"primaryKey"`
	Timestamp time.Time `gorm:"index"`

	// 1. The Intent (Why?)
	GoalDescription string

	// 2. The Strategy Applied (How?)
	StrategyID string `gorm:"index"`
	Strategy   Universal

	// 3. The Agent (Who?)
	AgentID string // Or HumanID

	// 4. The Reasoning Artifact
	ReasoningLog string

	// 5. Audit
	CreatedAt time.Time
}

// ParticularReference links a Particular to a Transformation.
// This forms the edges of the causal graph (Inputs/Outputs).
type ParticularReference struct {
	ID               string `gorm:"primaryKey"`
	TransformationID string `gorm:"index"`
	ParticularID     string `gorm:"index"`

	Role string // "INPUT", "OUTPUT", "CONTEXT"

	// Snapshot captures the state of the Particular at the moment of transformation.
	// This enables time-travel debugging.
	Snapshot JSONMap `gorm:"type:text"`
}

// Edge represents a relationship between nodes across planes.
// e.g. Particular A --DEPENDS_ON--> Particular B
// e.g. Universal A --SOLVES--> Universal B
type Edge struct {
	ID        string `gorm:"primaryKey"`
	SourceID  string `gorm:"index"`
	TargetID  string `gorm:"index"`
	Predicate string `gorm:"index"` // MANIFESTS, SOLVES, CONTRADICTS, DEPENDS_ON

	Weight     float64
	Confidence float64 // For AI-inferred links

	CreatedAt time.Time
}

// JSONMap is a helper for storing map[string]any in GORM
type JSONMap map[string]any

func (m *JSONMap) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), m)
}

func (m JSONMap) Value() (driver.Value, error) {
	return json.Marshal(m)
}
