package connector

import (
	"context"
	"errors"
	"strings"

	mt "opsseek/backend/tool/mongodb"
)

type mongodbAdapter struct{ connection mt.ConnectionInput }

func newMongoDBAdapter(target Target, _ Limits) (Adapter, error) {
	in := mt.ConnectionInput{
		Host:           stringValue(target.Resource.Config, "host"),
		Port:           configPort(target.Resource.Config, 27017),
		Database:       stringValue(target.Resource.Config, "database"),
		AuthSource:     stringValue(target.Resource.Config, "auth_source"),
		TimeoutSeconds: configInt(target.Resource.Config, "timeout_seconds"),
	}
	secret := secretFields(target.Secret)
	in.Username, in.Password = strings.TrimSpace(secret["username"]), secret["password"]
	if in.Host == "" {
		return nil, connectorError(CategoryConfiguration, "configure MongoDB", false, errors.New("host is required"))
	}
	if in.Username == "" && in.Password != "" {
		return nil, connectorError(CategoryConfiguration, "configure MongoDB", false, errors.New("credential username is required when a password is configured"))
	}
	return &mongodbAdapter{connection: in}, nil
}

func (a *mongodbAdapter) Kind() string               { return "MongoDB" }
func (a *mongodbAdapter) Capabilities() []Capability { return []Capability{CapabilityMongoDBInspect} }
func (a *mongodbAdapter) Test(ctx context.Context) error {
	_, err := mt.Health(ctx, a.connection)
	return mongodbError("test MongoDB connection", err)
}

func mongodbError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connectorError(CategoryTimeout, operation, true, err)
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "authentication") || strings.Contains(low, "unauthorized") || strings.Contains(low, "auth") {
		return connectorError(CategoryAuthentication, operation, false, err)
	}
	return connectorError(CategoryUpstream, operation, true, err)
}
