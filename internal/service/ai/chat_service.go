package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/arkcode369/ark-intelligent/internal/domain"
	"github.com/arkcode369/ark-intelligent/internal/ports"
	"github.com/arkcode369/ark-intelligent/pkg/logger"
)

var chatLog = logger.Component("chat-service")

// ChatService orchestrates the chatbot pipeline:
// 1. Load conversation history
// 2. Build context-aware system prompt
// 3. Call Claude (primary) → Gemini (fallback) → template (last resort)
// 4. Persist conversation
type ChatService struct {
	claude         ports.ChatEngine
	gemini         *GeminiClient // may be nil
	convRepo       ports.ConversationRepository
	contextBuilder *ContextBuilder
	toolConfig     *ToolConfig
}

// NewChatService creates a ChatService with the given dependencies.
// gemini may be nil (no Gemini fallback available).
func NewChatService(
	claude ports.ChatEngine,
	gemini *GeminiClient,
	convRepo ports.ConversationRepository,
	contextBuilder *ContextBuilder,
	toolConfig *ToolConfig,
) *ChatService {
	return &ChatService{
		claude:         claude,
		gemini:         gemini,
		convRepo:       convRepo,
		contextBuilder: contextBuilder,
		toolConfig:     toolConfig,
	}
}

// HandleMessage processes a free-text user message through the AI pipeline.
// contentBlocks is non-nil when the message contains media (images, documents).
// Returns the assistant's response text.
func (cs *ChatService) HandleMessage(ctx context.Context, userID int64, text string, role domain.UserRole, contentBlocks []ports.ContentBlock) (string, error) {
	// 1. Load conversation history (last 20 messages for context window)
	history, err := cs.convRepo.GetHistory(ctx, userID, 20)
	if err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to load conversation history")
		history = nil // non-fatal — proceed without history
	}

	// Resolve the effective text for context building and history.
	// For multimodal messages without caption, extract text from content blocks
	// or generate a descriptive placeholder.
	effectiveText := text
	if effectiveText == "" && len(contentBlocks) > 0 {
		// Try to extract text from content blocks
		for _, b := range contentBlocks {
			if b.Type == "text" && b.Text != "" {
				effectiveText = b.Text
				break
			}
		}
		// If still empty, generate a descriptive label for context
		if effectiveText == "" {
			effectiveText = describeContentBlocks(contentBlocks)
		}
	}

	// 2. Build system prompt with market data injection
	systemPrompt := cs.contextBuilder.BuildSystemPrompt(ctx, effectiveText)

	// 3. Build messages array (history + current message)
	messages := make([]ports.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)

	// Build the current user message (multimodal or text-only)
	currentMsg := ports.ChatMessage{Role: "user"}
	if len(contentBlocks) > 0 {
		currentMsg.ContentBlocks = contentBlocks
	} else {
		currentMsg.Content = text
	}
	messages = append(messages, currentMsg)

	// 4. Resolve tools for user's tier
	tools := cs.toolConfig.ToolsForRole(role)

	// 5. Try Claude (primary)
	req := ports.ChatRequest{
		UserID:       userID,
		Messages:     messages,
		SystemPrompt: systemPrompt,
		Tools:        tools,
	}

	resp, err := cs.claude.Chat(ctx, req)
	if err == nil && resp.Content != "" {
		// Success — persist conversation (use effectiveText for multimodal description)
		cs.saveConversation(ctx, userID, effectiveText, resp.Content)

		if len(resp.ToolsUsed) > 0 {
			chatLog.Info().
				Int64("user_id", userID).
				Strs("tools_used", resp.ToolsUsed).
				Int("input_tokens", resp.InputTokens).
				Int("output_tokens", resp.OutputTokens).
				Msg("Claude response with tools")
		}

		return resp.Content, nil
	}

	// Claude failed — log and attempt fallback
	if err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("Claude failed, attempting Gemini fallback")
	} else {
		chatLog.Warn().Int64("user_id", userID).Msg("Claude returned empty response, attempting Gemini fallback")
	}

	// 6. Try Gemini (fallback) — single-turn only (no history or multimodal support)
	if cs.gemini != nil && effectiveText != "" {
		geminiResp, geminiErr := cs.gemini.GenerateWithSystem(ctx, systemPrompt, effectiveText)
		if geminiErr == nil && geminiResp != "" {
			// Prefix with fallback notice
			fallbackResponse := fmt.Sprintf(
				"<i>[⚠️ Claude endpoint unreachable — response via Gemini fallback]</i>\n\n%s",
				geminiResp,
			)

			// Save to history (but note it's a fallback response)
			cs.saveConversation(ctx, userID, effectiveText, geminiResp)

			chatLog.Info().Int64("user_id", userID).Msg("Gemini fallback succeeded")
			return fallbackResponse, nil
		}

		if geminiErr != nil {
			chatLog.Error().Err(geminiErr).Int64("user_id", userID).Msg("Gemini fallback also failed")
		}
	}

	// 7. Template fallback (last resort)
	chatLog.Error().Int64("user_id", userID).Msg("all AI services unavailable — using template fallback")
	return templateFallback(), nil
}

// ClearHistory wipes conversation history for a user.
func (cs *ChatService) ClearHistory(ctx context.Context, userID int64) error {
	return cs.convRepo.ClearHistory(ctx, userID)
}

// IsAvailable returns true if the primary chat engine (Claude) is configured.
func (cs *ChatService) IsAvailable(ctx context.Context) bool {
	return cs.claude != nil && cs.claude.IsAvailable(ctx)
}

// saveConversation persists both user message and assistant response.
func (cs *ChatService) saveConversation(ctx context.Context, userID int64, userMsg, assistantMsg string) {
	if err := cs.convRepo.AppendMessage(ctx, userID, ports.ChatMessage{
		Role:    "user",
		Content: userMsg,
	}); err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to save user message")
	}

	if err := cs.convRepo.AppendMessage(ctx, userID, ports.ChatMessage{
		Role:    "assistant",
		Content: assistantMsg,
	}); err != nil {
		chatLog.Warn().Err(err).Int64("user_id", userID).Msg("failed to save assistant message")
	}
}

// templateFallback returns a helpful message when all AI services are down.
func templateFallback() string {
	var b strings.Builder
	b.WriteString("<b>⚠️ AI services temporarily unavailable</b>\n\n")
	b.WriteString("Both Claude and Gemini are currently unreachable.\n")
	b.WriteString("You can still use these commands:\n\n")
	b.WriteString("/cot — COT positioning overview\n")
	b.WriteString("/outlook — Weekly market outlook\n")
	b.WriteString("/calendar — Economic calendar\n")
	b.WriteString("/macro — FRED macro dashboard\n")
	b.WriteString("/signals — Active trading signals\n")
	b.WriteString("/rank — Currency strength ranking\n\n")
	b.WriteString("<i>Please try again later for AI chat features.</i>")
	return b.String()
}

// describeContentBlocks generates a descriptive label for multimodal content
// when no text was provided. Used for conversation history and context building.
func describeContentBlocks(blocks []ports.ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "image":
			parts = append(parts, "[Image]")
		case "document":
			if b.FileName != "" {
				parts = append(parts, fmt.Sprintf("[Document: %s]", b.FileName))
			} else {
				parts = append(parts, "[Document]")
			}
		}
	}
	if len(parts) == 0 {
		return "[Media message]"
	}
	return strings.Join(parts, " ")
}
