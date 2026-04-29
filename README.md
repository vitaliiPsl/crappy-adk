# Crappy Agent Development Kit

<div align="center">
  <img src="icon.png" alt="crappy-adk" width="260" /><br/><br/>

  [![Go](https://img.shields.io/badge/Go-1.25.6-00ADD8?style=flat&logo=go&logoColor=white)](https://golang.org)
  [![GoDoc](https://pkg.go.dev/badge/github.com/vitaliiPsl/crappy-adk.svg)](https://pkg.go.dev/github.com/vitaliiPsl/crappy-adk)
  [![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
</div>

A toolkit for building AI agents in Go. ReAct loop, providers, tools, hooks, and middleware — pick what you need.

## Motivation

Felt bored while on vacation, so decided to learn more about AI agents and build one myself.

## Contents

- [Install](#install)
- [Quick start](#quick-start)
- [Providers](#providers)
- [Tools](#tools)
- [Streaming](#streaming)
- [Multi-turn conversations](#multi-turn-conversations)
- [Hooks](#hooks)
- [Middleware](#middleware)
- [Limits](#limits)
- [Structured output](#structured-output)
- [Extending](#extending)
- [Subagents](#subagents)
- [Examples](#examples)
- [License](#license)

## Install

```sh
go get github.com/vitaliiPsl/crappy-adk
```

Requires Go 1.25.6.

API documentation: https://pkg.go.dev/github.com/vitaliiPsl/crappy-adk

## Quick start

```go
ctx := context.Background()

model, err := google.New(os.Getenv("GEMINI_API_KEY"), "gemini-2.5-flash")
if err != nil {
    log.Fatal(err)
}

a, err := agent.New(model,
    agent.WithSystemPrompt("You are a helpful assistant."),
    agent.WithTools(filesystem.NewReadFile(), filesystem.NewListDirectory()),
)
if err != nil {
    log.Fatal(err)
}

result, err := a.Run(ctx, []kit.Message{
    kit.NewUserMessage(kit.NewTextPart("What does this project do?")),
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Output.Text)
fmt.Printf("messages produced: %d\n", len(result.Messages))
fmt.Printf("tokens used: in=%d out=%d\n", result.Usage.InputTokens, result.Usage.OutputTokens)
```

## Providers

Each provider package exposes a `New(...)` constructor for simple model IDs and a `NewWithConfig(...)` constructor when you want to attach static model metadata such as token limits, capabilities, pricing, or release dates. The returned `kit.Model` handles both blocking (`Generate`) and streaming (`GenerateStream`) calls — the agent doesn't know or care which provider is behind it.

| Provider | Package | API | Models |
|---|---|---|---|
| Anthropic | `providers/anthropic` | Anthropic Messages API | claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5, ... |
| OpenAI | `providers/openai` | OpenAI Responses API | gpt-5.4, gpt-5.4-pro, gpt-5.4-mini, ... |
| Google Gemini | `providers/google` | Gemini GenerateContent API | gemini-2.5-pro, gemini-2.5-flash, ... |

All first-party providers support extended thinking where the underlying model offers it.
Anthropic, OpenAI, and Google also support structured final output via `agent.WithResponseSchema(...)`
or the typed helper `agent.WithResponseSchemaFor[T]()`.

These adapters represent API dialects rather than vendor names. Each provider can be pointed at a compatible endpoint with `WithBaseURL(...)`.

`providers/openai` targets the OpenAI Responses API and can also point at OpenAI-compatible backends:

```go
model, err := openai.New("ollama", "gemma4",
    openai.WithBaseURL("http://localhost:11434/v1"),
)

```

`providers/anthropic` targets the Anthropic Messages API:

```go
model, err := anthropic.New(apiKey, "claude-compatible",
    anthropic.WithBaseURL("https://your-anthropic-compatible-gateway.example.com"),
)
```

`providers/google` targets the Gemini GenerateContent API:

```go
model, err := google.New(apiKey, "gemini-compatible",
    google.WithBaseURL("https://your-gemini-compatible-gateway.example.com"),
)
```

## Tools

Tools are the actions the agent can take during the ReAct loop. Each tool has a name, a description the model uses to decide when to call it, a JSON schema for its arguments, and an execute function.

### Built-in

| Tool | Package | What it does |
|---|---|---|
| `bash` | `tools/bash` | Run a shell command with a configurable timeout |
| `read_file` | `tools/fs` | Read a file with optional line range |
| `write_file` | `tools/fs` | Write or overwrite a file |
| `edit_file` | `tools/fs` | Replace an exact string within a file |
| `list` | `tools/fs` | List directory contents with a configurable limit |

### Custom tools

`FunctionTool[T]` in `x/tool` wraps a typed Go function as a tool. The JSON schema for arguments is generated automatically from `T` — no manual schema definition needed. Arguments are validated against the schema before the handler is called.

```go
type GetTimeInput struct {
    Timezone string `json:"timezone" jsonschema:"IANA timezone name, e.g. America/New_York"`
}

getTime, err := tool.NewFunction(
    "get_time",
    "Get the current time in a given IANA timezone.",
    func(_ context.Context, args GetTimeInput) (string, error) {
        loc, err := time.LoadLocation(args.Timezone)
        if err != nil {
            return "", fmt.Errorf("unknown timezone: %s", args.Timezone)
        }
        return time.Now().In(loc).Format(time.RFC3339), nil
    },
)
```

## Streaming

`Stream` returns a lazy single-consumption iterator that yields fine-grained events as they arrive.
Consume it once with `range stream.Iter()`, then check `stream.Result()` for terminal errors. If you call `stream.Result()` before
iteration starts, it drains the stream for you. If you stop iterating early,
`stream.Result()` returns the partial result accumulated so far and does not resume.

The stream emits unified content-part lifecycle events (`content_part_started`,
`content_part_delta`, `content_part_done`) for every kind of content — text,
thinking, tool calls, and tool results — plus higher-level `message` and
`compaction_done` events once those items are fully assembled.

```go
stream, err := a.Stream(ctx, messages)
if err != nil {
    log.Fatal(err)
}

for event := range stream.Iter() {
    switch event.Type {
    case kit.EventContentPartStarted:
        switch event.ContentPartType {
        case kit.ContentTypeThinking:
            fmt.Print("[thinking] ")
        case kit.ContentTypeText:
            fmt.Print("[assistant] ")
        }
    case kit.EventContentPartDelta:
        fmt.Print(event.Text)
    case kit.EventContentPartDone:
        if event.ContentPart == nil {
            break
        }
        switch event.ContentPart.Type {
        case kit.ContentTypeThinking, kit.ContentTypeText:
            fmt.Print("\n")
        case kit.ContentTypeToolCall:
            fmt.Printf("[tool %s] requested\n", event.ContentPart.Name)
        case kit.ContentTypeToolResult:
            fmt.Printf("[tool %s] done\n", event.ContentPart.Name)
        }
    case kit.EventMessage:
        fmt.Printf("[message %s complete]\n", event.Message.Role)
    }
}
if err := stream.Err(); err != nil {
    log.Fatal(err)
}

result, err := stream.Result()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("final text: %s\n", result.Output.Text)
fmt.Printf("messages produced: %d\n", len(result.Messages))
```

## Multi-turn conversations

The agent is stateless between runs. To continue a conversation, pass the messages from the previous result back on the next call.

```go
result, err := a.Run(ctx, messages)

// Continue the conversation
messages = append(messages, result.Messages...)
messages = append(messages, kit.NewUserMessage(kit.NewTextPart("follow-up question")))

result, err = a.Run(ctx, messages)
```

## Hooks

Eight hooks cover every stage of the ReAct loop. Return a modified value to replace the original, or an error to abort. Tool hooks are the exception: an error becomes a tool result and the loop continues.

**`WithOnRunStart`** — once before the loop begins. Returned messages replace the originals.
```go
func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error)
```

**`WithOnRunEnd`** — once after the loop completes. `err` is non-nil if the run failed.
```go
func(ctx context.Context, result kit.Result, err error) (context.Context, error)
```

**`WithOnTurnStart`** — start of each turn, before the model is called.
```go
func(ctx context.Context, messages []kit.Message) (context.Context, []kit.Message, error)
```

**`WithOnTurnEnd`** — end of each turn, after all tools complete.
```go
func(ctx context.Context, messages []kit.Message) (context.Context, error)
```

**`WithOnModelRequest`** — before each model call. Returned request replaces the original.
```go
func(ctx context.Context, req kit.ModelRequest) (context.Context, kit.ModelRequest, error)
```

**`WithOnModelResponse`** — after each model call. Returned response replaces the original.
```go
func(ctx context.Context, resp kit.ModelResponse) (context.Context, kit.ModelResponse, error)
```

**`WithOnToolCall`** — before each tool execution. Return an error to block the call; the error is sent back to the model as the tool result.
```go
func(ctx context.Context, call kit.ToolCall) (context.Context, kit.ToolCall, error)
```

**`WithOnToolResult`** — after each tool execution.
```go
func(ctx context.Context, result kit.ToolResult) (context.Context, kit.ToolResult, error)
```

## Middleware

Middleware in `x/middleware` wraps the model and intercepts every `Generate` and `GenerateStream` call. Multiple middlewares can be chained.

```go
a, err := agent.New(model,
    agent.WithModelMiddleware(middleware.NewRetry(
        middleware.WithMaxAttempts(5),
        middleware.WithBaseDelay(300*time.Millisecond),
        middleware.WithMaxDelay(15*time.Second),
    )),
)
```

## Limits

The guards in `x/limits` limit how much work one agent run can do.
Each guard is an agent option that uses hooks to observe turns, model usage, or
tool calls.

```go
a, err := agent.New(model,
    limits.WithMaxTurns(10),
    limits.WithMaxToolCalls(25),
    limits.WithMaxUsage(limits.UsageLimits{
        OutputTokens: 20_000,
    }),
    limits.WithToolLoopDetection(3, 15),
)
```

Usage is accumulated after each model response and is checked along with tool-call limits before starting the next turn. Tool loop detection stops repeated identical tool calls within the configured turn window.

## Structured output

Use structured output when you want the final agent answer to be valid JSON that
matches a schema. The provider may enforce the schema natively, and the ADK
always validates the final JSON locally before returning it on `result.StructuredOutput`.

For most cases, the typed helper is the nicest API:

```go
type ReleaseNotes struct {
    Title      string   `json:"title" jsonschema:"Short release note title"`
    Highlights []string `json:"highlights" jsonschema:"Short highlights for the release note"`
    Breaking   bool     `json:"breaking" jsonschema:"Whether this update is breaking"`
}

a, err := agent.New(model,
    agent.WithSystemPrompt("Extract release notes into JSON. Include 2 or 3 highlights."),
    agent.WithResponseSchemaFor[ReleaseNotes](),
)
if err != nil {
    log.Fatal(err)
}

result, err := a.Run(ctx, []kit.Message{
    kit.NewUserMessage(kit.NewTextPart("Summarize this release update.")),
})
if err != nil {
    log.Fatal(err)
}

var notes ReleaseNotes
if err := json.Unmarshal(result.StructuredOutput.JSON, &notes); err != nil {
    log.Fatal(err)
}
```

If you need full manual control, use `agent.WithResponseSchema(schema)` with a
`*jsonschema.Schema` instead.

## Extending

Everything is an interface. If something doesn't fit, replace it.

- **Model** — `kit.Model`. Point at any inference backend via a provider package or implement your own.
- **Tool** — `kit.Tool`, or use `tool.NewFunction[T]` from `x/tool` for auto-schema from a Go struct.
- **Middleware** — `func(Model) Model`. Built-in middleware lives in `x/middleware`.
- **System prompt** — a static prompt set with `agent.WithSystemPrompt(...)`.
- **Instruction** — `func(ctx) (string, error)`. Dynamic prompt source evaluated fresh each run; built-in instruction sources live in `x/instructions`.
- **Compactor** — `kit.Compactor`. Built-in compactor implementations live in `x/compactors`.
- **Extension** — `agent.WithExtension([]Option)` bundles options into a reusable capability.

## Subagents

`WithSubAgents` in `extensions/subagents` registers an `agent` delegation tool on the parent agent. When called, it runs the selected subagent's full ReAct loop and returns its output.

```go
researcher, err := agent.New(model,
    agent.WithName("researcher"),
    agent.WithDescription("Explores the codebase and answers factual questions."),
    agent.WithSystemPrompt("You are a code researcher."),
)

writer, err := agent.New(model,
    agent.WithName("writer"),
    agent.WithDescription("Turns findings into clear markdown documentation."),
    agent.WithSystemPrompt("You are a technical writer."),
)

orchestrator, err := agent.New(model,
    agent.WithSystemPrompt("You are an orchestrator. Always delegate — never answer directly."),
    agent.WithExtension(subagents.WithSubAgents(researcher, writer)),
)
```

## Examples

| Example | What it shows |
|---|---|
| `examples/01-basic` | `Run()`, no tools |
| `examples/02-stream` | `Stream()`, events in real time |
| `examples/03-tools` | `FunctionTool[T]` with a custom typed tool |
| `examples/04-providers` | Anthropic, OpenAI, and Google side by side |
| `examples/05-local-model` | `providers/openai` against a local Ollama server |
| `examples/06-multiturn` | Stateless multi-turn conversation pattern |
| `examples/07-hooks` | Token logging and tool timing |
| `examples/08-middleware` | Retry middleware with custom backoff |
| `examples/09-limits` | Run limits with limit guards |
| `examples/10-structured-output` | Typed structured final output with `WithResponseSchemaFor[T]()` |
| `examples/11-subagents` | Orchestrator with researcher and writer subagents |

Run any example from the repo root:

```sh
GEMINI_API_KEY=... go run ./examples/01-basic
```

## License

MIT — see [LICENSE](LICENSE).
