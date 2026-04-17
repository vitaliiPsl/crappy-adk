package memory

import (
	"context"
	"fmt"

	"github.com/vitaliiPsl/crappy-adk/agent"
	"github.com/vitaliiPsl/crappy-adk/kit"
)

const baseInstructionTemplate = `You have access to durable memory%s.

Use memory to preserve stable information that should remain useful in future conversations.
Saved memory is automatically included in your instructions.

Memory should be used only for durable user preferences about how you should behave or work.
Good examples include response style preferences, review preferences, and stable workflow preferences.
%s
Do not store temporary task details, workspace facts, or one-off requests as memory.`

type config struct {
	maxMemories      int
	enableWriteTools bool
}

type state struct {
	store Store
	cfg   config
}

func (s *state) hook() kit.OnModelRequest {
	return func(ctx context.Context, req kit.ModelRequest) (context.Context, kit.ModelRequest, error) {
		memories, err := s.store.List(ctx)
		if err != nil {
			return ctx, req, err
		}

		rendered := renderInstruction(memories, s.cfg.maxMemories)
		if rendered == "" {
			return ctx, req, nil
		}

		req.Instruction += "\n\n" + rendered

		return ctx, req, nil
	}
}

type Option func(*config)

// WithMaxInjected limits how many memories are injected into each model request.
func WithMaxInjected(n int) Option {
	return func(cfg *config) {
		cfg.maxMemories = n
	}
}

// WithReadOnly disables write and delete tools, exposing only read access.
func WithReadOnly() Option {
	return func(cfg *config) {
		cfg.enableWriteTools = false
	}
}

func defaultConfig() config {
	return config{
		maxMemories:      10,
		enableWriteTools: true,
	}
}

func (cfg config) instruction() string {
	toolText := ""
	writeText := ""

	if cfg.enableWriteTools {
		toolText = " and two related tools: write_memory and delete_memory"
		writeText = "Only write memory when the user clearly expresses a lasting preference that should carry into future sessions.\n"
	}

	return fmt.Sprintf(baseInstructionTemplate, toolText, writeText)
}

// WithMemory registers memory tools and injects saved memories into model requests.
func WithMemory(store Store, opts ...Option) []agent.Option {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	state := &state{
		store: store,
		cfg:   cfg,
	}

	options := []agent.Option{
		agent.WithInstructions(func(_ context.Context) (string, error) { return cfg.instruction(), nil }),
		agent.WithOnModelRequest(state.hook()),
	}

	if cfg.enableWriteTools {
		options = append(options, agent.WithTools(state.newWriteTool(), state.newDeleteTool()))
	}

	return options
}
