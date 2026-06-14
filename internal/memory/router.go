package memory

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/easp-platform/easp/internal/models"
)

// MemoryRouter 记忆路由器 - 核心组件
// 负责根据上下文（租户/用户/角色/查询）按需加载和注入记忆
type MemoryRouter struct {
	memorySvc *MemoryService
	config    RouterConfig
}

// RouterConfig 路由器配置（可配置，不写死）
type RouterConfig struct {
	// 用户记忆配置
	UserMemoryLimit   int  `json:"user_memory_limit"`   // 最多加载几条用户记忆，默认5
	UserMemoryEnabled bool `json:"user_memory_enabled"` // 是否启用用户记忆

	// 技能记忆配置
	SkillMemoryLimit   int  `json:"skill_memory_limit"`   // 最多加载几条技能记忆，默认3
	SkillMemoryEnabled bool `json:"skill_memory_enabled"` // 是否启用技能记忆

	// 实体记忆配置
	EntityLimit   int  `json:"entity_limit"`   // 最多加载几个实体，默认5
	EntityEnabled bool `json:"entity_enabled"` // 是否启用实体记忆

	// 角色记忆配置
	RoleMemoryEnabled bool `json:"role_memory_enabled"` // 是否启用角色级记忆共享

	// 记忆提取配置
	ExtractEnabled bool `json:"extract_enabled"` // 是否启用记忆提取
}

// DefaultRouterConfig 默认配置
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		UserMemoryLimit:    5,
		UserMemoryEnabled:  true,
		SkillMemoryLimit:   3,
		SkillMemoryEnabled: true,
		EntityLimit:        5,
		EntityEnabled:      true,
		RoleMemoryEnabled:  true,
		ExtractEnabled:     true,
	}
}

// MemoryContext 记忆上下文 - 加载结果
type MemoryContext struct {
	UserMemories  []models.UserMemory  `json:"user_memories"`
	SkillMemories []models.SkillMemory `json:"skill_memories"`
	Entities      []models.Entity      `json:"entities"`
	RoleMemories  []models.UserMemory  `json:"role_memories"` // 同角色其他用户的共享记忆
}

// NewMemoryRouter 创建记忆路由器
func NewMemoryRouter(memorySvc *MemoryService, config RouterConfig) *MemoryRouter {
	return &MemoryRouter{
		memorySvc: memorySvc,
		config:    config,
	}
}

// GetMemoryService 获取记忆服务（供外部使用）
func (r *MemoryRouter) GetMemoryService() *MemoryService {
	return r.memorySvc
}

// LoadMemories 按需加载记忆（核心方法）
// tenantID: 租户隔离
// userID: 用户隔离
// query: 当前查询（用于相关性匹配）
// LoadMemories 按需加载记忆（核心方法）
func (r *MemoryRouter) LoadMemories(tenantID, userID, query string, permissions []string) *MemoryContext {
	ctx := &MemoryContext{}
	settings := r.memorySvc.GetMemorySettings(tenantID, userID)
	if !settings.RecallEnabled {
		log.Printf("MemoryRouter: recall disabled for tenant=%s, user=%s", tenantID, userID)
		return ctx
	}
	log.Printf("MemoryRouter: loading memories for tenant=%s, user=%s, query=%s, hybrid=%v/%s", tenantID, userID, truncate(query, 50), settings.HybridSearchEnabled, settings.HybridSearchMode)

	// 并行加载各类记忆：向量/关键词/实体召回互不阻塞，降低模型前置理解等待时间。
	var wg sync.WaitGroup
	run := func(enabled bool, fn func()) {
		if !enabled {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}
	run(r.config.UserMemoryEnabled, func() { r.loadUserMemories(ctx, tenantID, userID, query) })
	run(r.config.SkillMemoryEnabled, func() { r.loadSkillMemories(ctx, tenantID, query) })
	run(r.config.EntityEnabled, func() { r.loadEntityMemories(ctx, tenantID, query) })
	run(r.config.RoleMemoryEnabled && len(permissions) > 0, func() { r.loadRoleMemories(ctx, tenantID, userID, query, permissions) })
	wg.Wait()

	return ctx
}

// loadUserMemories 加载用户个人记忆
func (r *MemoryRouter) loadUserMemories(ctx *MemoryContext, tenantID, userID, query string) {
	// 先尝试关键词搜索（有查询词时）
	if query != "" {
		memories, err := r.memorySvc.SearchUserMemories(tenantID, userID, query, r.config.UserMemoryLimit)
		if err != nil {
			log.Printf("MemoryRouter: search user memories failed: %v, falling back to recent", err)
		} else if len(memories) > 0 {
			log.Printf("MemoryRouter: found %d user memories via search", len(memories))
			ctx.UserMemories = memories
			return
		} else {
			log.Printf("MemoryRouter: search returned 0 results, falling back to recent memories")
		}
	}

	// 回退：加载最近的记忆
	memories, err := r.memorySvc.GetUserMemories(tenantID, userID, "", r.config.UserMemoryLimit)
	if err != nil {
		log.Printf("MemoryRouter: failed to load user memories: %v", err)
		return
	}
	log.Printf("MemoryRouter: loaded %d user memories (recent)", len(memories))
	ctx.UserMemories = memories
}

// loadSkillMemories 加载技能记忆
func (r *MemoryRouter) loadSkillMemories(ctx *MemoryContext, tenantID, query string) {
	if query == "" {
		return
	}

	memories, err := r.memorySvc.SearchSkillMemories(tenantID, query, r.config.SkillMemoryLimit)
	if err != nil {
		log.Printf("MemoryRouter: failed to search skill memories: %v", err)
		return
	}
	ctx.SkillMemories = memories
}

// loadEntityMemories 加载实体记忆
func (r *MemoryRouter) loadEntityMemories(ctx *MemoryContext, tenantID, query string) {
	if query == "" {
		return
	}

	entities, err := r.memorySvc.SearchEntities(tenantID, query, r.config.EntityLimit)
	if err != nil {
		log.Printf("MemoryRouter: failed to search entities: %v", err)
		return
	}
	ctx.Entities = entities
}

// loadRoleMemories 加载角色共享记忆
// 同一租户下、同一角色的其他用户的记忆（只加载fact类型，不加载preference）
func (r *MemoryRouter) loadRoleMemories(ctx *MemoryContext, tenantID, userID, query string, permissions []string) {
	// 判断是否管理员角色（管理员可以看到更多共享记忆）
	isAdmin := false
	for _, p := range permissions {
		if p == "*" || p == "admin" {
			isAdmin = true
			break
		}
	}

	// 非管理员只加载自己的记忆
	if !isAdmin {
		return
	}

	// 管理员可以看到同租户其他管理员的fact类型记忆
	memories, err := r.memorySvc.GetRoleMemories(tenantID, userID, query, r.config.UserMemoryLimit)
	if err != nil {
		log.Printf("MemoryRouter: failed to load role memories: %v", err)
		return
	}

	ctx.RoleMemories = memories
}

// BuildMemoryPrompt 构建记忆注入的system prompt
func (r *MemoryRouter) BuildMemoryPrompt(basePrompt string, memCtx *MemoryContext) string {
	if memCtx == nil {
		return basePrompt
	}

	var sections []string

	// 1. 用户偏好记忆
	if len(memCtx.UserMemories) > 0 {
		var prefs, facts []string
		for _, m := range memCtx.UserMemories {
			switch m.Type {
			case "preference":
				prefs = append(prefs, "- "+m.Content)
			case "fact":
				facts = append(facts, "- "+m.Content)
			}
		}

		if len(prefs) > 0 {
			sections = append(sections, "## 用户偏好\n"+strings.Join(prefs, "\n"))
		}
		if len(facts) > 0 {
			sections = append(sections, "## 用户背景\n"+strings.Join(facts, "\n"))
		}
	}

	// 2. 角色共享记忆
	if len(memCtx.RoleMemories) > 0 {
		var facts []string
		for _, m := range memCtx.RoleMemories {
			facts = append(facts, "- "+m.Content)
		}
		if len(facts) > 0 {
			sections = append(sections, "## 团队知识\n"+strings.Join(facts, "\n"))
		}
	}

	// 3. 技能/知识记忆
	if len(memCtx.SkillMemories) > 0 {
		var skills []string
		for _, m := range memCtx.SkillMemories {
			desc := ""
			if m.Description != nil {
				desc = *m.Description + ": "
			}
			skills = append(skills, fmt.Sprintf("- %s%s", desc, truncate(m.Content, 200)))
		}
		if len(skills) > 0 {
			sections = append(sections, "## 相关知识\n"+strings.Join(skills, "\n"))
		}
	}

	// 4. 实体记忆
	if len(memCtx.Entities) > 0 {
		var entities []string
		for _, e := range memCtx.Entities {
			entities = append(entities, fmt.Sprintf("- %s (%s)", e.Name, e.Type))
		}
		if len(entities) > 0 {
			sections = append(sections, "## 已知实体\n"+strings.Join(entities, "\n"))
		}
	}

	if len(sections) == 0 {
		return basePrompt
	}

	// 注入到base prompt
	memoryBlock := strings.Join(sections, "\n\n")
	return basePrompt + "\n\n" + `以下是从记忆系统中检索到的上下文信息，请在回答时参考：

` + memoryBlock + `

注意：这些信息来自记忆系统，可能不完全准确。如果用户明确指出信息有误，以用户说法为准。`
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
