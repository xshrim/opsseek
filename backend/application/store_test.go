package application

import (
	"context"
	"strings"
	"testing"

	"opskeeper/backend/authorization"
)

func TestApplicationUpdateQueryFiltersBeforeReturning(t *testing.T) {
	for _, projectID := range []string{"", "project-id"} {
		for _, restricted := range []bool{false, true} {
			for _, scopes := range [][]string{nil, {"scope-id"}} {
				ctx := context.Background()
				if restricted {
					ctx = authorization.WithScopeFilter(ctx, authorization.ScopeFilter{ScopeIDs: scopes})
				}
				query, args := applicationUpdateQuery(ctx, projectID, make([]any, 12))
				if !strings.HasSuffix(query, " RETURNING a.id") || !strings.Contains(query, "FROM projects p") || !strings.Contains(query, "a.project_id=p.id") {
					t.Fatalf("invalid update query: %s", query)
				}
				wantArgs := 12
				if projectID != "" {
					wantArgs++
					if !strings.Contains(query, "AND a.project_id=$13::uuid") {
						t.Fatalf("missing project filter: %s", query)
					}
				}
				if restricted {
					if len(scopes) == 0 {
						if !strings.Contains(query, " AND FALSE RETURNING") {
							t.Fatalf("missing deny-all filter: %s", query)
						}
					} else {
						wantArgs++
						if !strings.Contains(query, " AND p.scope_id = ANY($") {
							t.Fatalf("missing scope filter: %s", query)
						}
					}
				}
				if len(args) != wantArgs {
					t.Fatalf("got %d arguments, want %d", len(args), wantArgs)
				}
			}
		}
	}
}
