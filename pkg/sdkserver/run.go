package sdkserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gptscript-ai/gptscript/pkg/gptscript"
	"github.com/gptscript-ai/gptscript/pkg/loader"
	"github.com/gptscript-ai/gptscript/pkg/mvl"
	"github.com/gptscript-ai/gptscript/pkg/runner"
	"github.com/gptscript-ai/gptscript/pkg/sdkserver/threads"
	"github.com/gptscript-ai/gptscript/pkg/types"
)

type loaderFunc func(context.Context, string, string, ...loader.Options) (types.Program, error)

func loaderWithLocation(f loaderFunc, loc string) loaderFunc {
	return func(ctx context.Context, s string, s2 string, options ...loader.Options) (types.Program, error) {
		return f(ctx, s, s2, append(options, loader.Options{
			Location: loc,
		})...)
	}
}

func (s *server) execAndStream(ctx context.Context, programLoader loaderFunc, logger mvl.Logger, w http.ResponseWriter, opts gptscript.Options, threadID, id uint64, input, subTool string, chatState any, toolDef fmt.Stringer) {
	g, err := gptscript.New(ctx, s.gptscriptOpts, opts)
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to initialize gptscript: %w", err))
		return
	}
	defer g.Close(false)

	prg, err := programLoader(ctx, toolDef.String(), subTool, loader.Options{Cache: g.Cache})
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to load program: %w", err))
		return
	}

	errChan := make(chan error)
	programOutput := make(chan *runner.ChatResponse)
	events := s.events.Subscribe()
	defer events.Close()

	go func() {
		run, err := g.Chat(ctx, chatState, prg, opts.Env, input)
		if err != nil {
			errChan <- err
		} else {
			programOutput <- &run
		}
		close(errChan)
		close(programOutput)
	}()

	processEventStreamOutput(ctx, s.threadsStore, logger, w, threadID, id, events.C, programOutput, errChan)
}

// processEventStreamOutput will stream the events of the tool to the response as server sent events.
// If an error occurs, then an event with the error will also be sent.
func processEventStreamOutput(ctx context.Context, s *threads.Store, logger mvl.Logger, w http.ResponseWriter, threadID, runID uint64, events <-chan threads.GPTScriptEvent, output <-chan *runner.ChatResponse, errChan chan error) {
	run := newRun(runID)
	setStreamingHeaders(w)

	runFinishEvent := streamEvents(ctx, logger, w, s, runID, run, events)

	var out *runner.ChatResponse
	select {
	case <-ctx.Done():
	case out = <-output:
		processStdout(run, out)

		writeServerSentEvent(logger, w, map[string]any{
			"stdout": out,
		})
	case err := <-errChan:
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to run file: %w", err))
	}

	if threadID == 0 && out != nil && !out.Done {
		// If this run wasn't put into a thread, and it is not "done" then create a thread for it.
		thread, err := s.CreateThread(ctx)
		if err != nil {
			logger.Warnf("failed to create thread: %v", err)
		}

		if thread != nil {
			threadID = thread.ID
		}
	}

	run.ThreadID = threadID

	if runFinishEvent != nil {
		run.ThreadID = threadID
		writeServerSentEvent(logger, w, process(run, *runFinishEvent))
	}

	// Now that we have received all events, send the DONE event.
	_, err := w.Write([]byte("data: [DONE]\n\n"))
	if err == nil {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	if out != nil && !out.Done {
		run.State = threads.Continue
	}

	if _, err := s.FinishRun(ctx, threadID, runID, run); err != nil {
		logger.Warnf("failed to finish run: %v", err)
	}

	logger.Debugf("wrote DONE event")
}

// streamEvents will stream the events of the tool to the response as server sent events.
func streamEvents(ctx context.Context, logger mvl.Logger, w http.ResponseWriter, s *threads.Store, runID uint64, run *threads.RunInfo, events <-chan threads.GPTScriptEvent) *threads.GPTScriptEvent {
	logger.Debugf("receiving events")

	defer func() {
		logger.Debugf("done receiving events")
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Debugf("context canceled while receiving events")
			go func() {
				//nolint:revive
				for range events {
				}
			}()
			return nil
		case e, ok := <-events:
			if ok && e.RunID != run.ID {
				continue
			}

			if !ok {
				return nil
			}

			if _, err := s.CreateEvent(ctx, runID, e); err != nil {
				logger.Warnf("failed to store event: %v", err)
			}

			if e.Type == runner.EventTypeRunFinish {
				return &e
			}

			writeServerSentEvent(logger, w, process(run, e))
		}
	}
}

func writeResponse(logger mvl.Logger, w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to marshal response: %w", err))
		return
	}

	_, _ = w.Write(b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeError(logger mvl.Logger, w http.ResponseWriter, code int, err error) {
	logger.Debugf("Writing error response with code %d: %v", code, err)

	w.WriteHeader(code)
	resp := map[string]any{
		"stderr": err.Error(),
	}

	b, err := json.Marshal(resp)
	if err != nil {
		_, _ = w.Write([]byte(fmt.Sprintf(`{"stderr": "%s"}`, err.Error())))
		return
	}

	_, _ = w.Write(b)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeServerSentEvent(logger mvl.Logger, w http.ResponseWriter, event any) {
	ev, err := json.Marshal(event)
	if err != nil {
		logger.Warnf("failed to marshal event: %v", err)
		return
	}

	_, err = w.Write([]byte(fmt.Sprintf("data: %s\n\n", ev)))
	if err == nil {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	logger.Debugf("wrote event: %v", string(ev))
}

func setStreamingHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
}
