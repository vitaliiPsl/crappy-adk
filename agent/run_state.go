package agent

import "github.com/vitaliiPsl/crappy-adk/kit"

type runState struct {
	transcript []kit.Message
	produced   kit.Result
}

func newRunState(messages []kit.Message) *runState {
	return &runState{transcript: messages}
}

func (s *runState) result() kit.Result {
	return s.produced
}

func (s *runState) messages() []kit.Message {
	return s.transcript
}

func (s *runState) setMessages(messages []kit.Message) {
	s.transcript = messages
}

func (s *runState) recordModelResponse(resp kit.ModelResponse) {
	s.transcript = append(s.transcript, resp.Message)
	s.produced.Messages = append(s.produced.Messages, resp.Message)
	s.produced.Usage.Add(resp.Usage)
	s.produced.LastUsage = resp.Usage
}

func (s *runState) recordFinalResponse(resp kit.ModelResponse) {
	assistantMsg := resp.Message
	s.produced.Output = assistantMsg.Output()
	s.produced.StructuredOutput = resp.StructuredOutput
}

func (s *runState) recordToolMessages(messages []kit.Message) {
	s.transcript = append(s.transcript, messages...)
	s.produced.Messages = append(s.produced.Messages, messages...)
}

func (s *runState) recordCompaction(compacted []kit.Message, summary *kit.Message) {
	s.transcript = compacted
	if summary != nil {
		s.produced.Messages = append(s.produced.Messages, *summary)
	}
}
