package threads

import (
	"maps"
	"time"

	"github.com/gptscript-ai/gptscript/pkg/engine"
	"github.com/gptscript-ai/gptscript/pkg/runner"
	gserver "github.com/gptscript-ai/gptscript/pkg/server"
	"github.com/gptscript-ai/gptscript/pkg/types"
	"gorm.io/datatypes"
)

type RunState string

const (
	Creating RunState = "creating"
	Running  RunState = "running"
	Continue RunState = "continue"
	Finished RunState = "finished"
	Error    RunState = "error"

	CallConfirm runner.EventType = "callConfirm"
	Prompt      runner.EventType = "prompt"
)

type Thread struct {
	ID         uint64    `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	FirstRunID uint64    `json:"firstRunID"`
}

type Run struct {
	ID             uint64                              `json:"id"`
	PreviousRunID  uint64                              `json:"previousRunID"`
	StartedAt      time.Time                           `json:"startedAt"`
	FinishedAt     time.Time                           `json:"finishedAt"`
	ThreadID       uint64                              `json:"threadID"`
	Input          string                              `json:"input"`
	Output         string                              `json:"output"`
	ChatStateAfter any                                 `json:"chatStateAfter" gorm:"serializer:json"`
	Run            datatypes.JSONType[*RunInfo]        `json:"run"`
	Calls          datatypes.JSONType[map[string]Call] `json:"calls"`
}

type Event struct {
	ID        uint64                             `json:"id"`
	RunID     uint64                             `json:"runID"`
	CreatedAt time.Time                          `json:"createdAt"`
	Event     datatypes.JSONType[GPTScriptEvent] `json:"event"`
}

type RunInfo struct {
	Calls     map[string]Call      `json:"-"`
	ID        string               `json:"id"`
	ThreadID  uint64               `json:"threadID"`
	Program   types.Program        `json:"program"`
	Input     string               `json:"input"`
	Output    string               `json:"output"`
	RawOutput *runner.ChatResponse `json:"rawOutput"`
	Error     string               `json:"error"`
	Start     time.Time            `json:"start"`
	End       time.Time            `json:"end"`
	State     RunState             `json:"state"`
	ChatState any                  `json:"chatState"`
}

type GPTScriptEvent struct {
	gserver.Event `json:",inline"`
	types.Prompt  `json:",inline"`
	Done          bool `json:"done"`
}

type Call struct {
	engine.CallContext `json:",inline"`

	Type        runner.EventType `json:"type"`
	Start       time.Time        `json:"start"`
	End         time.Time        `json:"end"`
	Input       string           `json:"input"`
	Output      []Output         `json:"output"`
	Usage       types.Usage      `json:"usage"`
	LLMRequest  any              `json:"llmRequest"`
	LLMResponse any              `json:"llmResponse"`
}

func (c *Call) SetSubCalls(subCalls map[string]engine.Call) {
	c.Output = append(c.Output, Output{
		SubCalls: maps.Clone(subCalls),
	})
}

func (c *Call) SetOutput(o string) {
	if len(c.Output) == 0 || len(c.Output[len(c.Output)-1].SubCalls) > 0 {
		c.Output = append(c.Output, Output{
			Content: o,
		})
	} else {
		c.Output[len(c.Output)-1].Content = o
	}
}

type Output struct {
	Content  string                 `json:"content"`
	SubCalls map[string]engine.Call `json:"subCalls"`
}
