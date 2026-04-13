package middleware

import (
	"context"

	"github.com/vitaliiPsl/crappy-adk/kit"
)

// BaseModel is a pass-through implementation of [kit.Model], meant to be embedded in middleware structs.
type BaseModel struct {
	// The next model in the chain.
	Next kit.Model
}

func (b *BaseModel) Config() kit.ModelConfig {
	return b.Next.Config()
}

func (b *BaseModel) Generate(ctx context.Context, req kit.ModelRequest) (kit.ModelResponse, error) {
	return b.Next.Generate(ctx, req)
}

func (b *BaseModel) GenerateStream(ctx context.Context, req kit.ModelRequest) (*kit.Stream[kit.ModelEvent, kit.ModelResponse], error) {
	return b.Next.GenerateStream(ctx, req)
}

// Chain composes multiple middleware functions into one, applying them in order.
func Chain(middlewares ...func(kit.Model) kit.Model) func(kit.Model) kit.Model {
	return func(llm kit.Model) kit.Model {
		for i := len(middlewares) - 1; i >= 0; i-- {
			llm = middlewares[i](llm)
		}

		return llm
	}
}
