package connector

import (
	"context"
	"fmt"
	"time"
	mt "opsseek/backend/tool/mongodb"
)

type MongoDBDraftInput struct{ mt.ConnectionInput }
type MongoDBDraftCheck struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	LatencyMS int64  `json:"latency_ms"`
}

func (s *Service) TestMongoDBDraft(ctx context.Context, in MongoDBDraftInput) (MongoDBDraftCheck, error) {
	started := time.Now()
	out := MongoDBDraftCheck{Status: "failed"}
	_, err := mt.Health(ctx, in.ConnectionInput)
	out.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		out.Message = fmt.Sprintf("MongoDB 连接测试失败：%s", err)
		return out, nil
	}
	out.Status = "succeeded"
	out.Message = "MongoDB 连接测试通过"
	return out, nil
}
