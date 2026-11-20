package skill

import "testing"

// TestAllowedToolNameCoversAllDomains guards the whitelist that CreateVersion
// enforces. The whitelist is generated from the backend/tool registries, so
// this also fails if a registry drops or renames a tool.
func TestAllowedToolNameCoversAllDomains(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		// Connector catalog tools kept for published builtin versions.
		{"connector_kubernetes_read", true},
		{"connector_metrics_query", true},
		{"connector_logs_query", true},
		{"connector_traces_query", true},
		{"connector_alerts_get", true},
		{"connector_postgresql_inspect", true},
		{"connector_kafka_inspect", true},
		// Legacy fine-grained domains that were already whitelisted.
		{"postgresql_health", true},
		{"postgresql_table_columns", true},
		{"mysql_table_structure", true},
		{"redis_slowlog", true},
		{"kafka_topic_partitions", true},
		// Domains added by the A1 fix.
		{"oracle_health", true},
		{"oracle_table_structure", true},
		{"elasticsearch_index_mapping", true},
		{"rabbitmq_vhosts", true},
		{"minio_bucket_lifecycle", true},
		{"oceanbase_health", true},
		{"oceanbase_table_structure", true},
		// Unknown or manipulated names must fail closed.
		{"", false},
		{"host_shell", false},
		{"postgresql_health ", false},
		{"POSTGRESQL_HEALTH", false},
		{"connector_kubernetes_write", false},
		{"kubernetes_pods", true},
	}
	for _, tc := range cases {
		if got := allowedToolName(tc.name); got != tc.want {
			t.Errorf("allowedToolName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestAllowedToolsMatchesLookup keeps the exported whitelist view consistent
// with the internal membership function.
func TestAllowedToolsMatchesLookup(t *testing.T) {
	items := AllowedTools()
	if len(items) == 0 {
		t.Fatal("AllowedTools() is empty")
	}
	seen := make(map[string]struct{}, len(items))
	for index, name := range items {
		if name == "" {
			t.Fatalf("AllowedTools()[%d] is empty", index)
		}
		if _, duplicate := seen[name]; duplicate {
			t.Fatalf("AllowedTools() contains duplicate %q", name)
		}
		if index > 0 && name < items[index-1] {
			t.Fatalf("AllowedTools() is not sorted: %q before %q", items[index-1], name)
		}
		seen[name] = struct{}{}
		if !allowedToolName(name) {
			t.Errorf("allowedToolName(%q) = false for exported whitelist entry", name)
		}
	}
}
