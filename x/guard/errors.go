package guard

import "errors"

var (
	// ErrMaxTurnsExceeded indicates that an agent run exceeded its configured model turn limit.
	ErrMaxTurnsExceeded = errors.New("guard: max turns exceeded")

	// ErrMaxToolCallsExceeded indicates that an agent run exceeded its configured tool call limit.
	ErrMaxToolCallsExceeded = errors.New("guard: max tool calls exceeded")

	// ErrToolLoopDetected indicates that an agent repeatedly requested the same tool call.
	ErrToolLoopDetected = errors.New("guard: tool loop detected")

	// ErrBudgetExceeded indicates that an agent run exceeded its configured token budget.
	ErrBudgetExceeded = errors.New("guard: budget exceeded")
)
