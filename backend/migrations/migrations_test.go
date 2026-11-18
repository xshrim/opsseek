package migrations

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoadOrdersEmbeddedMigrations(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("load() returned %d migrations, want the initial baseline and four forward migrations", len(items))
	}
	if items[0].version != 1 || items[0].name != "initial" {
		t.Fatalf("loaded migration = %#v, want version 1 initial", items[0])
	}
	if items[1].version != 2 || items[1].name != "oceanbase_default_port" {
		t.Fatalf("loaded migration = %#v, want version 2 oceanbase_default_port", items[1])
	}
	if items[2].version != 3 || items[2].name != "mongodb_resource_and_document_database" {
		t.Fatalf("unexpected MongoDB migration: %#v", items[2])
	}
	for _, item := range items {
		if item.sql == "" || item.checksum == "" {
			t.Fatalf("migration is missing SQL or checksum: %#v", item)
		}
		if item.checksum != migrationChecksum([]byte(item.sql)) {
			t.Fatalf("migration checksum does not match SQL: %#v", item)
		}
	}
	for index := 1; index < len(items); index++ {
		if items[index-1].version >= items[index].version {
			t.Fatalf("migrations are not ordered: %#v", items)
		}
	}
}

func TestMongoDBResourceSchema(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	sql := items[2].sql
	start, end := strings.Index(sql, "'{"), strings.Index(sql, "}'::jsonb")
	if start < 0 || end < start {
		t.Fatal("MongoDB schema missing")
	}
	var schema struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Default   any  `json:"default"`
			Sensitive bool `json:"sensitive"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(sql[start+1:end+1]), &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "host" || schema.Properties["port"].Default != float64(27017) {
		t.Fatalf("unexpected MongoDB schema: %#v", schema)
	}
	for _, key := range []string{"username", "password"} {
		if !schema.Properties[key].Sensitive {
			t.Errorf("%s must be encrypted", key)
		}
	}
}

func TestBaselineDefinesLatestColumnsWithoutIncrementalChanges(t *testing.T) {
	items, err := load()
	if err != nil {
		t.Fatal(err)
	}
	sql := items[0].sql
	for _, forbidden := range []string{"ADD COLUMN", "DROP COLUMN", "Consolidated from", "UPDATE resource_schemas", "UPDATE resources", "UPDATE diagnosis_sessions"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("baseline contains historical change %q", forbidden)
		}
	}
	for _, expected := range []string{"CREATE TABLE project_topology_annotations", "CREATE TABLE project_topology_positions", "CREATE TABLE project_topology_edges", "scope_shared boolean", "collaboration smallint", "lease_owner text"} {
		if !strings.Contains(sql, expected) {
			t.Errorf("baseline missing final definition %q", expected)
		}
	}
}

func TestRepositoryApplicationTargetsDropPrimaryKeyBeforeMakingResourceNullable(t *testing.T) {
	items, err := loadLegacy()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	for _, item := range items {
		if item.version != 90 {
			continue
		}
		dropPrimaryKey := strings.Index(item.sql, "DROP CONSTRAINT inspection_policy_targets_pkey")
		makeResourceNullable := strings.Index(item.sql, "ALTER COLUMN resource_id DROP NOT NULL")
		if dropPrimaryKey < 0 || makeResourceNullable < 0 || dropPrimaryKey > makeResourceNullable {
			t.Fatalf("migration 90 must drop the resource_id primary key before making it nullable")
		}
		return
	}
	t.Fatal("migration 90 was not loaded")
}

func TestResourceTypeLabelsMigrationCoversRequestedKinds(t *testing.T) {
	items, err := loadLegacy()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	for _, item := range items {
		if item.version == 91 {
			for _, expected := range []string{"'Artifact' THEN '制品仓库'", "'Repository' THEN '代码仓库'", "'Nacos' THEN '注册中心和配置中心'", "'TongHttpServer' THEN '前端代理'", "'Kafka' THEN '消息队列'", "'Elasticsearch' THEN '文档检索'", "'MinIO' THEN '对象存储'", "'MCPServer' THEN '工具接入'", "'Monitor' THEN '可观察性'"} {
				if !strings.Contains(item.sql, expected) {
					t.Errorf("migration 91 does not contain %q", expected)
				}
			}
			return
		}
	}
	t.Fatal("migration 91 was not loaded")
}

func TestObservabilityModesMigrationAddsSkywalkingAndNormalizesLegacyModes(t *testing.T) {
	items, err := loadLegacy()
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	for _, item := range items {
		if item.version == 95 {
			for _, expected := range []string{"'Skywalking'", "'Prometheus'", "'Alertmanager'", "SET subtype = 'Direct'"} {
				if !strings.Contains(item.sql, expected) {
					t.Errorf("migration 95 does not contain %q", expected)
				}
			}
			return
		}
	}
	t.Fatal("migration 95 was not loaded")
}
