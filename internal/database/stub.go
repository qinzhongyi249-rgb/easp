// Package database provides a database connection stub for the EASP open source core.
// The commercial version includes full MySQL connection pooling and AutoMigrate.
// In the open source version, DB is nil — server.go and builtin_governance.go
// will skip database operations when DB is nil.
package database

import "github.com/jmoiron/sqlx"

// DB is the global database instance. In the open source version, this is nil.
// Code using DB should check for nil before executing queries.
var DB *sqlx.DB
