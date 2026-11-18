package inspection

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"opsseek/backend/expertgroup"
)

func TestNormalizePolicyRequiresBoundedAuthorizedIntent(t *testing.T) {
	base := Policy{ScopeID: "scope-1", Name: "API health", Cron: "0 * * * *", Timezone: "Asia/Shanghai", TargetResourceIDs: []string{"target-1", "target-1"}, Timeout: time.Minute, MaxConcurrent: 1, MaxToolCalls: 12, MaxTokens: 20000}
	policy, err := normalizePolicy(base)
	if err != nil {
		t.Fatalf("normalizePolicy() error = %v", err)
	}
	if policy.Status != PolicyActive || len(policy.TargetResourceIDs) != 1 {
		t.Fatalf("normalized policy = %#v", policy)
	}
	base.TargetResourceIDs, base.TargetLabels = nil, nil
	if _, err := normalizePolicy(base); !errors.As(err, new(*ValidationError)) {
		t.Fatalf("missing target error = %v", err)
	}
}

func TestNormalizePolicyAllowsApplicationTargets(t *testing.T) {
	policy, err := normalizePolicy(Policy{
		ScopeID: "scope-1", Name: "application health", Cron: "0 * * * *", Timezone: "UTC",
		TargetApplicationIDs: []string{"app-1", "app-1"}, Timeout: time.Minute,
		MaxConcurrent: 1, MaxToolCalls: 12, MaxTokens: 20000,
	})
	if err != nil {
		t.Fatalf("normalizePolicy() error = %v", err)
	}
	if len(policy.TargetApplicationIDs) != 1 || policy.TargetApplicationIDs[0] != "app-1" {
		t.Fatalf("application targets = %v, want one distinct app", policy.TargetApplicationIDs)
	}
}

func TestNormalizePolicyRejectsPersonaAndExpertGroupTogether(t *testing.T) {
	_, err := normalizePolicy(Policy{
		ScopeID: "scope", Name: "health", Cron: "0 * * * *", Timezone: "UTC",
		TargetResourceIDs: []string{"res"}, PersonaID: "persona-1", ExpertGroupID: "group-1",
		Timeout: time.Minute, MaxConcurrent: 1, MaxToolCalls: 1, MaxTokens: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("err = %v, want mutual exclusion rejection", err)
	}
}

type fakeExpertGroupCatalog struct {
	group expertgroup.Group
}

func (f fakeExpertGroupCatalog) GetGroup(context.Context, string) (expertgroup.Group, error) {
	return f.group, nil
}
func (f fakeExpertGroupCatalog) ResolveGroup(context.Context, string, string, string) (expertgroup.ResolvedGroup, error) {
	return expertgroup.ResolvedGroup{}, nil
}
func (f fakeExpertGroupCatalog) ResolveEffective(context.Context, string, string, string, string) (expertgroup.ResolvedGroup, bool, error) {
	return expertgroup.ResolvedGroup{}, false, nil
}

func TestCreatePolicyValidatesExpertGroup(t *testing.T) {
	store := &memoryStore{}
	resources := policyResourceReader{"target": {ID: "target", ScopeID: "scope"}}
	service := NewService(store, resources).WithExpertGroups(fakeExpertGroupCatalog{group: expertgroup.Group{ID: "group-1", ScopeID: "scope", Status: "disabled"}})
	_, err := service.CreatePolicy(context.Background(), Policy{
		ScopeID: "scope", Name: "health", Cron: "0 * * * *", Timezone: "UTC",
		TargetResourceIDs: []string{"target"}, ExpertGroupID: "group-1",
		Timeout: time.Minute, MaxConcurrent: 1, MaxToolCalls: 1, MaxTokens: 1,
	}, "actor")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("err = %v, want disabled group rejection", err)
	}
}
