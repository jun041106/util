// Copyright 2015 Apcera Inc. All rights reserved.

package taskrenderer

import (
	"fmt"
	"io"
	"time"
)

// Renderer holds all of the state necessary to gather and output TaskEvents.
type Renderer struct {
	out io.Writer
	err io.Writer

	eventCh chan *TaskEvent

	options *FormatOptions
}

// FormatOptions provides a way to customize the output of TaskEvents.
type FormatOptions struct {
	showTime bool
}

// New instantiates and returns a new Renderer object.
func New(out io.Writer, err io.Writer, showTime bool) *Renderer {
	return &Renderer{
		out: out,
		err: err,
		options: &FormatOptions{
			showTime: showTime,
		},
	}
}

// RenderEvents reads every event sent on the given channel and renders it.
// The channel can be closed by the caller at any time to stop rendering.
func (r *Renderer) RenderEvents(eventCh chan *TaskEvent) {
	for event := range eventCh {
		r.renderEvent(event)
	}
}

// renderEvent varies output depending on the information provided
// by the current taskEvent.
func (r *Renderer) renderEvent(event *TaskEvent) {
	s := ""

	if r.options.showTime {
		s += fmt.Sprintf("[%s] ", time.Unix(0, event.Time).Format(time.UnixDate))
	}

	if event.Thread != "" {
		s += fmt.Sprintf("Thread: %s ", event.Thread)
	}

	s += fmt.Sprintf("Stage: %s ", event.Stage)

	s += fmt.Sprintf("Subtask (%d/%d): %s ", event.Index, event.Total, event.Subtask)

	if event.TotalProgress != 0 {
		s += fmt.Sprintf("Progress (%d/%d)", event.CurrentProgress, event.TotalProgress)
	}

	fmt.Fprintf(r.out, "%s\n", s)
}
