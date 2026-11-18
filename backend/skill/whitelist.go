package skill

import (
	"sort"

	elasticsearch "opsseek/backend/tool/elasticsearch"
	kafka "opsseek/backend/tool/kafka"
	kubernetes "opsseek/backend/tool/kubernetes"
	minio "opsseek/backend/tool/minio"
	mysql "opsseek/backend/tool/mysql"
	oceanbase "opsseek/backend/tool/oceanbase"
	oracle "opsseek/backend/tool/oracle"
	postgresql "opsseek/backend/tool/postgresql"
	rabbitmq "opsseek/backend/tool/rabbitmq"
	redis "opsseek/backend/tool/redis"
)

// connectorToolNames are the coarse-grained Connector catalog tools that
// predate the per-domain registries. Published builtin Skill versions still
// reference them, so they remain part of the whitelist next to the registry
// tools.
var connectorToolNames = []string{
	"connector_kubernetes_read",
	"connector_metrics_query",
	"connector_logs_query",
	"connector_traces_query",
	"connector_alerts_get",
	"connector_postgresql_inspect",
	"connector_kafka_inspect",
}

// registryToolNames is derived from the tool registries in backend/tool/*,
// which makes the registries the single source of truth for every
// fine-grained diagnostic tool a Skill version may declare. Adding a tool to
// a registry extends the whitelist automatically; a removed tool disappears
// from it, so the two can no longer drift apart.
func registryToolNames() []string {
	names := make([]string, 0, 128)
	names = append(names, connectorToolNames...)
	for _, tool := range elasticsearch.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range kafka.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range kubernetes.AvailableTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range minio.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range mysql.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range oceanbase.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range oracle.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range postgresql.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range rabbitmq.ListTools() {
		names = append(names, tool.Name)
	}
	for _, tool := range redis.ListTools() {
		names = append(names, tool.Name)
	}
	return names
}

var allowedTools = func() map[string]struct{} {
	items := registryToolNames()
	allowed := make(map[string]struct{}, len(items))
	for _, name := range items {
		allowed[name] = struct{}{}
	}
	return allowed
}()

// AllowedTools returns the sorted platform tool whitelist: the Connector
// catalog tools plus every tool registered by the backend/tool registries.
func AllowedTools() []string {
	items := registryToolNames()
	sort.Strings(items)
	return items
}

func allowedToolName(name string) bool {
	_, ok := allowedTools[name]
	return ok
}
