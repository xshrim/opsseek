package connector

import (
	"context"
	"opsseek/backend/engine"
	"testing"
)

func TestDatabaseConnectionsUseBusinessTargetAndDiagnosticCredentials(t *testing.T) {
	s := NewService(nil, dockerResourceReader{secret: []byte(`{"username":"diagnostic_reader","password":"secret","database":"postgres"}`)}, nil, nil, DefaultLimits())
	ctx := context.Background()
	item := engine.ContextResource{Kind: "PostgreSQL", Config: map[string]any{"host": "db", "database": "postgres"}, Binding: map[string]any{"database": "opsseek", "username": "app"}}
	pg, err := s.postgreSQLConnection(ctx, item)
	if err != nil || pg.Database != "opsseek" || pg.Username != "diagnostic_reader" || pg.Password != "secret" {
		t.Fatalf("PostgreSQL connection target or credentials incorrect: %v", err)
	}
	item.Kind = "MySQL"
	my, err := s.mySQLConnection(ctx, item)
	if err != nil || my.Database != "opsseek" || my.Username != "diagnostic_reader" {
		t.Fatalf("MySQL connection target or credentials incorrect: %v", err)
	}
	item.Kind = "OceanBase"
	ob, err := s.oceanBaseConnection(ctx, item)
	if err != nil || ob.Database != "opsseek" || ob.Username != "diagnostic_reader" {
		t.Fatalf("OceanBase connection target or credentials incorrect: %v", err)
	}
	item.Kind = "Oracle"
	item.Config = map[string]any{"sid": "admin"}
	item.Binding = map[string]any{"service_name": "business", "username": "app"}
	or, err := s.oracleConnection(ctx, item)
	if err != nil || or.ServiceName != "business" || or.SID != "" || or.Username != "diagnostic_reader" {
		t.Fatalf("Oracle connection target or credentials incorrect: %v", err)
	}
}
