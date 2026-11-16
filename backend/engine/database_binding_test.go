package engine

import "testing"

func TestDatabaseConnectionTargetKeepsResourceCredentials(t *testing.T) {
	for _, kind := range []string{"PostgreSQL", "MySQL", "OceanBase"} {
		defaults := map[string]any{"host": "db", "database": "postgres", "username": "reader", "password": "secret"}
		got, err := DatabaseConnectionTarget(kind, defaults, map[string]any{"database": "opskeeper", "username": "app", "password": "wrong"})
		if err != nil || got["database"] != "opskeeper" || got["username"] != "reader" || got["password"] != "secret" || defaults["database"] != "postgres" {
			t.Fatalf("%s: target=%v err=%v", kind, got, err)
		}
	}
	got, err := DatabaseConnectionTarget("Oracle", map[string]any{"sid": "old"}, map[string]any{"service_name": "business"})
	if err != nil || got["service_name"] != "business" || got["sid"] != nil {
		t.Fatalf("Oracle target=%v err=%v", got, err)
	}
	if _, err := DatabaseConnectionTarget("Oracle", nil, map[string]any{"service_name": "x", "sid": "y"}); err == nil {
		t.Fatal("ambiguous service/SID accepted")
	}
	if _, err := DatabaseConnectionTarget("PostgreSQL", nil, map[string]any{"target_conflict": true}); err == nil {
		t.Fatal("conflicting business targets accepted")
	}
}

func TestDatabaseToolArgumentsOnlySuppliesSupportedSchema(t *testing.T) {
	binding := map[string]any{"database": "orders", "schema": "business", "username": "app"}
	got := DatabaseToolArguments("PostgreSQL", binding, []byte(`{"properties":{}}`), nil)
	if len(got) != 0 {
		t.Fatalf("connection fields injected: %v", got)
	}
	got = DatabaseToolArguments("PostgreSQL", binding, []byte(`{"properties":{"schema":{}}}`), map[string]any{"table": "orders"})
	if got["schema"] != "business" || got["table"] != "orders" || got["username"] != nil {
		t.Fatalf("args=%v", got)
	}
	got = DatabaseToolArguments("PostgreSQL", binding, []byte(`{"properties":{"schema":{}}}`), map[string]any{"schema": "explicit"})
	if got["schema"] != "explicit" {
		t.Fatalf("explicit schema replaced: %v", got)
	}
}
