package devin

import (
	"fmt"
	"slices"
	"strings"

	"github.com/WncFht/devin2api/internal/adapter"
)

// supportedModelEfforts 是客户端可选择的推理等级；Devin 的 none/off
// 只有在目录真正返回对应变体时才允许使用，绝不凭空合成。
var supportedModelEfforts = map[string]bool{
	"none": true, "off": true, "minimal": true, "low": true,
	"medium": true, "high": true, "xhigh": true, "max": true,
}

// disallowedModelVariant 拒绝昂贵或不明确的加速/优先级变体，不按
// 后缀位置猜测（priority-high 同样禁止进入目录和请求路由）。
func disallowedModelVariant(uid string) bool {
	for _, part := range strings.Split(strings.ToLower(uid), "-") {
		if part == "fast" || part == "priority" {
			return true
		}
	}
	return false
}

// modelVariant 从末尾识别标准推理等级，其他后缀不参与自动推理映射。
func modelVariant(uid string) (base, effort string, ok bool) {
	if disallowedModelVariant(uid) {
		return "", "", false
	}
	pos := strings.LastIndexByte(uid, '-')
	if pos <= 0 || pos == len(uid)-1 {
		return "", "", false
	}
	effort = uid[pos+1:]
	if !supportedModelEfforts[effort] {
		return "", "", false
	}
	return normalizedModelBase(uid[:pos]), effort, true
}

// normalizedModelBase 把连写的数字版本片段标准化：
// claude-opus-5-5 → claude-opus-v5.5，swe-2 → swe-v2。
func normalizedModelBase(base string) string {
	parts := strings.Split(base, "-")
	for i := 0; i < len(parts); i++ {
		if !numericVersionPart(parts[i]) {
			continue
		}
		j := i + 1
		for j < len(parts) && numericVersionPart(parts[j]) {
			j++
		}
		parts[i] = "v" + strings.Join(parts[i:j], ".")
		parts = slices.Delete(parts, i+1, j)
		break
	}
	return strings.Join(parts, "-")
}

// numericVersionPart 判断一个模型名称片段是否为纯数字版本。
func numericVersionPart(part string) bool {
	if part == "" {
		return false
	}
	for _, char := range part {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// normalizedModelCatalog 从 Devin 原始目录投影出唯一的基础模型列表；
// 原始目录继续留给 router、能力检查和请求映射，不能用投影覆盖它。
func normalizedModelCatalog(raw []adapter.ModelInfo) []adapter.ModelInfo {
	byID := make(map[string]adapter.ModelInfo, len(raw))
	for _, model := range raw {
		base, _, ok := modelVariant(model.ID)
		if !ok {
			continue
		}
		if _, exists := byID[base]; !exists {
			model.ID = base
			model.AliasOf = ""
			byID[base] = model
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	models := make([]adapter.ModelInfo, 0, len(ids))
	for _, id := range ids {
		models = append(models, byID[id])
	}
	return models
}

// resolveReasoningVariant 根据规范化基础模型和显式 reasoning.effort
// 找到实际存在的 Devin UID；原始 UID 保持精确透传，兼容旧客户端。
func resolveReasoningVariant(model, effort string, raw []adapter.ModelInfo) (string, error) {
	if disallowedModelVariant(model) {
		return "", fmt.Errorf("model %q is excluded (fast/priority variant)", model)
	}
	variants := make(map[string]string)
	for _, entry := range raw {
		if entry.ID == model {
			return model, nil
		}
		base, level, ok := modelVariant(entry.ID)
		if ok && base == model {
			if level == "none" {
				level = "off"
			}
			variants[level] = entry.ID
		}
	}
	if len(variants) == 0 {
		if effort != "" || strings.Contains(model, "-v") {
			return "", fmt.Errorf("model %q has no supported reasoning variants in Devin catalog", model)
		}
		return model, nil // 保留既有的非目录模型直传能力。
	}
	if effort == "" {
		// 未指定 effort 时，优先中档，再选较低档；不自行合成变体。
		for _, preferred := range []string{"medium", "low", "minimal", "off", "high", "xhigh", "max"} {
			if uid := variants[preferred]; uid != "" {
				return uid, nil
			}
		}
	}
	if effort == "none" {
		effort = "off"
	}
	if uid := variants[effort]; uid != "" {
		return uid, nil
	}
	return "", fmt.Errorf("model %q does not support reasoning.effort=%q", model, effort)
}
