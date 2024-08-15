package server

import (
	"context"
	"fmt"

	"github.com/gptscript-ai/gptscript/pkg/runner"
	"github.com/gptscript-ai/gptscript/pkg/types"
)

type Event struct {
	runner.Event `json:",inline"`
	RunID        string         `json:"runID,omitempty"`
	Program      *types.Program `json:"program,omitempty"`
	Input        string         `json:"input,omitempty"`
	Output       string         `json:"output,omitempty"`
	Err          string         `json:"err,omitempty"`
}

type execKey struct{}

func ContextWithNewRunID(ctx context.Context, runID uint64) context.Context {
	return context.WithValue(ctx, execKey{}, fmt.Sprint(runID))
}

func RunIDFromContext(ctx context.Context) string {
	runID, _ := ctx.Value(execKey{}).(string)
	return runID
}
