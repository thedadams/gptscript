package sdkserver

import (
	"fmt"
	"strings"
	"time"

	"github.com/gptscript-ai/gptscript/pkg/cache"
	"github.com/gptscript-ai/gptscript/pkg/openai"
	"github.com/gptscript-ai/gptscript/pkg/parser"
	"github.com/gptscript-ai/gptscript/pkg/runner"
	"github.com/gptscript-ai/gptscript/pkg/sdkserver/threads"
	"github.com/gptscript-ai/gptscript/pkg/types"
)

type toolDefs []types.ToolDef

func (t toolDefs) String() string {
	s := new(strings.Builder)
	for i, tool := range t {
		s.WriteString(tool.String())
		if i != len(t)-1 {
			s.WriteString("\n\n---\n\n")
		}
	}

	return s.String()
}

type (
	cacheOptions  cache.Options
	openAIOptions openai.Options
)

type toolOrFileRequest struct {
	content       `json:",inline"`
	file          `json:",inline"`
	cacheOptions  `json:",inline"`
	openAIOptions `json:",inline"`

	ThreadID             uint64   `json:"threadID"`
	PreviousRunID        uint64   `json:"previousRunID"`
	ToolDefs             toolDefs `json:"toolDefs,inline"`
	SubTool              string   `json:"subTool"`
	Input                string   `json:"input"`
	ChatState            any      `json:"chatState"`
	Workspace            string   `json:"workspace"`
	Env                  []string `json:"env"`
	CredentialContext    string   `json:"credentialContext"`
	CredentialOverrides  []string `json:"credentialOverrides"`
	Confirm              bool     `json:"confirm"`
	Location             string   `json:"location,omitempty"`
	ForceSequential      bool     `json:"forceSequential"`
	DefaultModelProvider string   `json:"DefaultModelProvider,omitempty"`
}

type content struct {
	Content string `json:"content"`
}

func (c *content) String() string {
	return c.Content
}

type file struct {
	File string `json:"file"`
}

func (f *file) String() string {
	return f.File
}

type loadRequest struct {
	content `json:",inline"`

	ToolDefs     toolDefs `json:"toolDefs,inline"`
	DisableCache bool     `json:"disableCache"`
	SubTool      string   `json:"subTool,omitempty"`
	File         string   `json:"file"`
}

type parseRequest struct {
	parser.Options `json:",inline"`
	content        `json:",inline"`

	DisableCache bool   `json:"disableCache"`
	File         string `json:"file"`
}

type modelsRequest struct {
	Providers []string `json:"providers"`
}

func newRun(id uint64) *threads.RunInfo {
	return &threads.RunInfo{
		ID:    fmt.Sprint(id),
		State: threads.Creating,
		Calls: make(map[string]threads.Call),
	}
}

func process(r *threads.RunInfo, e threads.GPTScriptEvent) map[string]any {
	switch e.Type {
	case threads.Prompt:
		return map[string]any{"prompt": prompt{
			Prompt: e.Prompt,
			ID:     e.RunID,
			Type:   e.Type,
			Time:   e.Time,
		}}
	case runner.EventTypeRunStart:
		r.Start = e.Time
		r.Program = *e.Program
		r.State = threads.Running
	case runner.EventTypeRunFinish:
		r.End = e.Time
		r.Output = e.Output
		r.Error = e.Err
		if r.Error != "" {
			r.State = threads.Error
		} else if !e.Done {
			r.State = threads.Continue
		} else {
			r.State = threads.Finished
		}
	}

	if e.CallContext == nil || e.CallContext.ID == "" {
		return map[string]any{"run": runEvent{
			RunInfo: *r,
			Type:    e.Type,
		}}
	}

	call := r.Calls[e.CallContext.ID]
	call.CallContext = *e.CallContext
	call.Type = e.Type

	switch e.Type {
	case runner.EventTypeCallStart:
		call.Start = e.Time
		call.Input = e.Content

	case runner.EventTypeCallSubCalls:
		call.SetSubCalls(e.ToolSubCalls)

	case runner.EventTypeCallProgress:
		call.SetOutput(e.Content)

	case runner.EventTypeCallFinish:
		call.End = e.Time
		call.SetOutput(e.Content)

	case runner.EventTypeChat:
		if e.ChatRequest != nil {
			call.LLMRequest = e.ChatRequest
		}
		if e.ChatResponse != nil {
			call.LLMResponse = e.ChatResponse
		}
	}

	r.Calls[e.CallContext.ID] = call
	return map[string]any{"call": call}
}

func processStdout(r *threads.RunInfo, cs *runner.ChatResponse) {
	if cs.Done {
		r.State = threads.Finished
	} else {
		r.State = threads.Continue
	}

	r.RawOutput = cs
	r.ChatState = cs.State
	r.Output = cs.Content
}

type runEvent struct {
	threads.RunInfo `json:",inline"`
	Type            runner.EventType `json:"type"`
}

type prompt struct {
	types.Prompt `json:",inline"`
	ID           string           `json:"id,omitempty"`
	Type         runner.EventType `json:"type,omitempty"`
	Time         time.Time        `json:"time,omitempty"`
}
