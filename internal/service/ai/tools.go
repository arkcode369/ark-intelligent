package ai

import (
	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
)

// ToolConfig manages tier-based server tool access.
type ToolConfig struct {
	tierTools map[domain.UserRole][]ports.ServerTool
}

// NewToolConfig creates the default tool configuration.
// Only server-managed tools (web_search, web_fetch, code_execution) are included.
// memory_20250818 is excluded — it requires client-side tool_use/tool_result
// round-trips which we don't support. Conversation persistence is handled by
// our own BadgerDB conversation repository instead.
func NewToolConfig() *ToolConfig {
	return &ToolConfig{
		tierTools: map[domain.UserRole][]ports.ServerTool{
			domain.RoleFree: {
				// No server tools for free tier (keeps costs low)
			},
			domain.RoleMember: {
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 3},
			},
			domain.RoleAdmin: {
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 5},
				{Type: "web_fetch_20260309", Name: "web_fetch", MaxUses: 3},
			},
			domain.RoleOwner: {
				{Type: "web_search_20250305", Name: "web_search", MaxUses: 5},
				{Type: "web_fetch_20260309", Name: "web_fetch", MaxUses: 5},
				{Type: "code_execution_20260120", Name: "code_execution"},
			},
			domain.RoleBanned: {}, // no tools
		},
	}
}

// ToolsForRole returns the allowed server tools for the given user role.
func (tc *ToolConfig) ToolsForRole(role domain.UserRole) []ports.ServerTool {
	if tools, ok := tc.tierTools[role]; ok {
		return tools
	}
	// Default to Free tier
	return tc.tierTools[domain.RoleFree]
}
