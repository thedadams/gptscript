package sdkserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	gcontext "github.com/gptscript-ai/gptscript/pkg/context"
	"github.com/gptscript-ai/gptscript/pkg/sdkserver/threads"
	"gorm.io/gorm"
)

func (s *server) createThread(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())

	thread, err := s.threadsStore.CreateThread(r.Context())
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, err)
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": thread})
}

type nameThreadRequest struct {
	Name string `json:"name"`
}

func (s *server) nameThread(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())

	threadID := r.PathValue("id")
	if threadID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	id, err := strconv.ParseUint(threadID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	reqObject := new(nameThreadRequest)
	if err = json.NewDecoder(r.Body).Decode(reqObject); err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("failed to decode request body: %w", err))
		return
	}

	thread, err := s.threadsStore.NameThread(r.Context(), id, reqObject.Name)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}

		writeError(logger, w, status, err)
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": thread})
}

func (s *server) listThreads(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())
	threads, err := s.threadsStore.ListThreads(r.Context())
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to list threads: %w", err))
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": map[string]any{"items": threads}})
}

func (s *server) getThread(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())
	threadID := r.PathValue("id")
	if threadID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	id, err := strconv.ParseUint(threadID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	thread, err := s.threadsStore.GetThread(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}

		writeError(logger, w, status, fmt.Errorf("failed to get thread: %w", err))
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": thread})
}

func (s *server) deleteThread(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())

	threadID := r.PathValue("id")
	if threadID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	id, err := strconv.ParseUint(threadID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	if err = s.threadsStore.DeleteThread(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}

		writeError(logger, w, status, fmt.Errorf("failed to delete thread: %w", err))
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": map[string]any{"deleted": true}})
}

func (s *server) listRuns(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())
	threadID := r.PathValue("thread_id")
	if threadID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("thread_id is required"))
		return
	}

	id, err := strconv.ParseUint(threadID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid thread_id: %w", err))
		return
	}

	runs, err := s.threadsStore.ListRuns(r.Context(), id)
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to get runs: %w", err))
		return
	}

	compiledRuns := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		compiledRuns = append(compiledRuns, compileRun(&run))
	}

	writeResponse(logger, w, map[string]any{"stdout": map[string]any{"items": compiledRuns}})
}

func (s *server) getRun(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())

	threadID := r.PathValue("thread_id")
	if threadID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("thread_id is required"))
		return
	}

	tID, err := strconv.ParseUint(threadID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid thread_id: %w", err))
		return
	}

	runID := r.PathValue("id")
	if runID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	id, err := strconv.ParseUint(runID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	run, err := s.threadsStore.GetRun(r.Context(), tID, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}

		writeError(logger, w, status, fmt.Errorf("failed to get run: %w", err))
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": compileRun(run)})
}

func (s *server) listEvents(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())

	if r.PathValue("thread_id") == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("thread_id is required"))
		return
	}

	runID := r.PathValue("id")
	if runID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	rID, err := strconv.ParseUint(runID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	events, err := s.threadsStore.ListEvents(r.Context(), rID)
	if err != nil {
		writeError(logger, w, http.StatusInternalServerError, fmt.Errorf("failed to get events: %w", err))
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": events})
}

func (s *server) getEvent(w http.ResponseWriter, r *http.Request) {
	logger := gcontext.GetLogger(r.Context())

	if r.PathValue("thread_id") == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("thread_id is required"))
		return
	}

	runID := r.PathValue("id")
	if runID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("id is required"))
		return
	}

	rID, err := strconv.ParseUint(runID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	eventID := r.PathValue("event_id")
	if eventID == "" {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("event_id is required"))
		return
	}

	eID, err := strconv.ParseUint(eventID, 10, 64)
	if err != nil {
		writeError(logger, w, http.StatusBadRequest, fmt.Errorf("invalid event_id: %w", err))
		return
	}

	event, err := s.threadsStore.GetEvent(r.Context(), rID, eID)
	status := http.StatusInternalServerError
	if errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusNotFound
	}

	if err != nil {
		writeError(logger, w, status, fmt.Errorf("failed to get event: %w", err))
		return
	}

	writeResponse(logger, w, map[string]any{"stdout": event})
}

func compileRun(run *threads.Run) map[string]any {
	var (
		runInfo           *threads.RunInfo
		parentCallFrameID string
		calls             map[string]threads.Call
	)
	if run != nil {
		runInfo = run.Run.Data()
		calls = run.Calls.Data()
		for _, callFrame := range calls {
			if callFrame.ParentID == "" {
				parentCallFrameID = callFrame.ID
				break
			}
		}
	}

	return map[string]any{
		"runFrame":          runInfo,
		"parentCallFrameID": parentCallFrameID,
		"calls":             calls,
	}
}
