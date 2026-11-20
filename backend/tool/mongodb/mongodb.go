package mongodb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ConnectionInput struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	Database       string `json:"database,omitempty"`
	AuthSource     string `json:"auth_source,omitempty"`
	Username       string `json:"username,omitempty"`
	Password       string `json:"password,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type ToolInfo struct{ Name, Description string }

var tools = []ToolInfo{
	{Name: "mongodb_health", Description: "Check MongoDB connectivity and server health."},
	{Name: "mongodb_database_info", Description: "Read MongoDB server and database statistics."},
	{Name: "mongodb_collections", Description: "List collections in the configured MongoDB database."},
	{Name: "mongodb_collection_stats", Description: "Read bounded statistics for one MongoDB collection."},
	{Name: "mongodb_indexes", Description: "Read indexes for one MongoDB collection."},
	{Name: "mongodb_collection_schema", Description: "Infer a bounded top-level field schema from a MongoDB collection sample."},
}

func ListTools() []ToolInfo { return append([]ToolInfo(nil), tools...) }

func InputSchema(extra map[string]any) map[string]any {
	p := map[string]any{
		"host":            map[string]any{"type": "string", "minLength": 1},
		"port":            map[string]any{"type": "integer", "minimum": 1, "maximum": 65535},
		"database":        map[string]any{"type": "string"},
		"auth_source":     map[string]any{"type": "string"},
		"username":        map[string]any{"type": "string"},
		"password":        map[string]any{"type": "string"},
		"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 300},
	}
	for k, v := range extra {
		p[k] = v
	}
	return map[string]any{"type": "object", "properties": p, "additionalProperties": false}
}

type CollectionInput struct {
	Collection string `json:"collection"`
}

type HealthOutput struct {
	Version   string `json:"version,omitempty"`
	Database  string `json:"database,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
}
type DatabaseInfoOutput struct {
	Database string         `json:"database"`
	Build    map[string]any `json:"build,omitempty"`
	Stats    map[string]any `json:"stats,omitempty"`
}
type CollectionsOutput struct {
	Collections []string `json:"collections"`
}
type CollectionStatsOutput struct {
	Collection string         `json:"collection"`
	Stats      map[string]any `json:"stats"`
}
type IndexInfo struct {
	Name string         `json:"name"`
	Keys map[string]any `json:"keys"`
}
type IndexesOutput struct {
	Collection string      `json:"collection"`
	Indexes    []IndexInfo `json:"indexes"`
}
type FieldInfo struct {
	Name  string   `json:"name"`
	Types []string `json:"types"`
	Count int      `json:"count"`
}
type CollectionSchemaOutput struct {
	Collection string      `json:"collection"`
	SampleSize int         `json:"sample_size"`
	Fields     []FieldInfo `json:"fields"`
}

func open(ctx context.Context, in ConnectionInput) (*mongo.Client, *mongo.Database, error) {
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return nil, nil, errors.New("host is required")
	}
	port := in.Port
	if port <= 0 {
		port = 27017
	}
	if port > 65535 {
		return nil, nil, errors.New("port must be between 1 and 65535")
	}
	database := strings.TrimSpace(in.Database)
	if database == "" {
		database = "admin"
	}
	authSource := strings.TrimSpace(in.AuthSource)
	if authSource == "" {
		authSource = "admin"
	}
	timeout := 10 * time.Second
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	uri := "mongodb://" + net.JoinHostPort(host, strconv.Itoa(port))
	opts := options.Client().ApplyURI(uri).SetServerSelectionTimeout(timeout)
	if strings.TrimSpace(in.Username) != "" {
		opts.SetAuth(options.Credential{Username: strings.TrimSpace(in.Username), Password: in.Password, AuthSource: authSource})
	}
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, nil, err
	}
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Ping(ctx2, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, err
	}
	return client, client.Database(database), nil
}

func withDB[T any](ctx context.Context, in ConnectionInput, fn func(context.Context, *mongo.Client, *mongo.Database) (T, error)) (T, error) {
	var zero T
	client, db, err := open(ctx, in)
	if err != nil {
		return zero, err
	}
	defer client.Disconnect(context.Background())
	return fn(ctx, client, db)
}

func Health(ctx context.Context, in ConnectionInput) (HealthOutput, error) {
	started := time.Now()
	return withDB(ctx, in, func(ctx context.Context, client *mongo.Client, db *mongo.Database) (HealthOutput, error) {
		var build bson.M
		_ = client.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&build)
		out := HealthOutput{Database: db.Name(), LatencyMS: time.Since(started).Milliseconds()}
		if v, ok := build["version"].(string); ok {
			out.Version = v
		}
		return out, nil
	})
}

func DatabaseInfo(ctx context.Context, in ConnectionInput) (DatabaseInfoOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, client *mongo.Client, db *mongo.Database) (DatabaseInfoOutput, error) {
		out := DatabaseInfoOutput{Database: db.Name(), Build: map[string]any{}, Stats: map[string]any{}}
		if err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&out.Build); err != nil {
			return out, err
		}
		if err := db.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&out.Stats); err != nil {
			return out, err
		}
		return out, nil
	})
}

func Collections(ctx context.Context, in ConnectionInput) (CollectionsOutput, error) {
	return withDB(ctx, in, func(ctx context.Context, _ *mongo.Client, db *mongo.Database) (CollectionsOutput, error) {
		names, err := db.ListCollectionNames(ctx, bson.D{})
		return CollectionsOutput{Collections: names}, err
	})
}

func collectionName(in CollectionInput) (string, error) {
	name := strings.TrimSpace(in.Collection)
	if name == "" {
		return "", errors.New("collection is required")
	}
	if len(name) > 255 || strings.ContainsAny(name, "\x00") {
		return "", errors.New("collection is invalid")
	}
	return name, nil
}

func CollectionStats(ctx context.Context, in ConnectionInput, input CollectionInput) (CollectionStatsOutput, error) {
	name, err := collectionName(input)
	if err != nil {
		return CollectionStatsOutput{}, err
	}
	return withDB(ctx, in, func(ctx context.Context, _ *mongo.Client, db *mongo.Database) (CollectionStatsOutput, error) {
		stats := map[string]any{}
		err := db.RunCommand(ctx, bson.D{{Key: "collStats", Value: name}}).Decode(&stats)
		return CollectionStatsOutput{Collection: name, Stats: stats}, err
	})
}

func Indexes(ctx context.Context, in ConnectionInput, input CollectionInput) (IndexesOutput, error) {
	name, err := collectionName(input)
	if err != nil {
		return IndexesOutput{}, err
	}
	return withDB(ctx, in, func(ctx context.Context, _ *mongo.Client, db *mongo.Database) (IndexesOutput, error) {
		cursor, err := db.Collection(name).Indexes().List(ctx)
		if err != nil {
			return IndexesOutput{}, err
		}
		defer cursor.Close(ctx)
		out := IndexesOutput{Collection: name, Indexes: []IndexInfo{}}
		for cursor.Next(ctx) {
			var raw bson.M
			if err := cursor.Decode(&raw); err != nil {
				return out, err
			}
			item := IndexInfo{Keys: map[string]any{}}
			if v, ok := raw["name"].(string); ok {
				item.Name = v
			}
			if keys, ok := raw["key"].(bson.D); ok {
				for _, e := range keys {
					item.Keys[e.Key] = e.Value
				}
			}
			out.Indexes = append(out.Indexes, item)
			if len(out.Indexes) >= 100 {
				break
			}
		}
		return out, cursor.Err()
	})
}

func CollectionSchema(ctx context.Context, in ConnectionInput, input CollectionInput) (CollectionSchemaOutput, error) {
	name, err := collectionName(input)
	if err != nil {
		return CollectionSchemaOutput{}, err
	}
	return withDB(ctx, in, func(ctx context.Context, _ *mongo.Client, db *mongo.Database) (CollectionSchemaOutput, error) {
		cursor, err := db.Collection(name).Find(ctx, bson.D{}, options.Find().SetLimit(50).SetProjection(bson.D{{Key: "_id", Value: 0}}))
		if err != nil {
			return CollectionSchemaOutput{}, err
		}
		defer cursor.Close(ctx)
		fields := map[string]*FieldInfo{}
		sample := 0
		for cursor.Next(ctx) {
			var raw bson.Raw
			if err := cursor.Decode(&raw); err != nil {
				return CollectionSchemaOutput{}, err
			}
			sample++
			elements, err := raw.Elements()
			if err != nil {
				return CollectionSchemaOutput{}, err
			}
			for _, element := range elements {
				name := element.Key()
				value := element.Value()
				item := fields[name]
				if item == nil {
					item = &FieldInfo{Name: name}
					fields[name] = item
				}
				typeName := value.Type.String()
				found := false
				for _, existing := range item.Types {
					if existing == typeName {
						found = true
						break
					}
				}
				if !found {
					item.Types = append(item.Types, typeName)
				}
				item.Count++
			}
		}
		out := CollectionSchemaOutput{Collection: name, SampleSize: sample, Fields: make([]FieldInfo, 0, len(fields))}
		for _, item := range fields {
			out.Fields = append(out.Fields, *item)
		}
		return out, cursor.Err()
	})
}

func Error(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
