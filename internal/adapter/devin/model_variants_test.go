package devin

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/WncFht/devin2api/internal/adapter"
)

// TestListModelsShowsNormalizedNames 验证真正对外的 ListModels 不泄露变体 UID。
func TestListModelsShowsNormalizedNames(t *testing.T) {
	a := &Adapter{
		models: []adapter.ModelInfo{
			{ID: "claude-opus-5-5-high"},
			{ID: "claude-opus-5-5-high-fast"},
			{ID: "swe-2-max"},
		},
		modelsExpiry: time.Now().Add(time.Minute),
	}
	got, err := a.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "claude-opus-v5.5" || got[1].ID != "swe-v2" {
		t.Fatalf("public model IDs = %#v", got)
	}
	if len(a.models) != 3 || a.models[0].ID != "claude-opus-5-5-high" {
		t.Fatal("raw upstream catalog was modified")
	}
}

// TestNormalizedModelCatalog 验证目录仅展示基础模型与标准推理等级。
func TestNormalizedModelCatalog(t *testing.T) {
	raw := []adapter.ModelInfo{
		{ID: "claude-opus-5-5-low", SupportsThinking: true},
		{ID: "claude-opus-5-5-high", SupportsThinking: true},
		{ID: "claude-opus-5-5-high-fast"},
		{ID: "claude-opus-5-5-priority-max"},
		{ID: "claude-opus-5-5-max"},
		{ID: "swe-2-high"},
		{ID: "swe-2-max"},
		{ID: "swe-2-flash-medium"},
		{ID: "unrecognized-model"},
	}
	got := normalizedModelCatalog(raw)
	ids := make([]string, 0, len(got))
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	want := []string{"claude-opus-v5.5", "swe-v2", "swe-v2-flash"}
	if !slices.Equal(ids, want) {
		t.Fatalf("catalog = %v, want %v", ids, want)
	}
}

// TestResolveReasoningVariant 验证显式等级严格匹配，不会偷偷退回其他档位。
func TestResolveReasoningVariant(t *testing.T) {
	catalog := []adapter.ModelInfo{
		{ID: "claude-opus-5-5-low"},
		{ID: "claude-opus-5-5-high"},
		{ID: "claude-opus-5-5-high-fast"},
		{ID: "claude-opus-5-5-max"},
		{ID: "swe-2-high"},
		{ID: "swe-2-max"},
	}
	for _, tc := range []struct {
		name, effort, want string
		fail               bool
	}{
		{"claude-opus-v5.5", "high", "claude-opus-5-5-high", false},
		{"claude-opus-v5.5", "low", "claude-opus-5-5-low", false},
		{"claude-opus-v5.5", "max", "claude-opus-5-5-max", false},
		{"claude-opus-v5.5", "medium", "", true},
		{"claude-opus-v5.5", "off", "", true},
		{"claude-opus-v5.5", "", "claude-opus-5-5-low", false},
		{"swe-v2", "high", "swe-2-high", false},
		{"swe-v2", "minimal", "", true},
		{"claude-opus-v5.6", "", "", true},
		{"claude-opus-5-5-high-fast", "", "", true},
		{"claude-opus-5-5-high", "", "claude-opus-5-5-high", false},
	} {
		got, err := resolveReasoningVariant(tc.name, tc.effort, catalog)
		if (err != nil) != tc.fail || (!tc.fail && got != tc.want) {
			t.Errorf("resolveReasoningVariant(%q,%q) = (%q,%v), want %q fail=%t", tc.name, tc.effort, got, err, tc.want, tc.fail)
		}
	}
}
