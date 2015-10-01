package taskrenderer

import (
	"fmt"
	"io"
	"time"
)

type Renderer struct {
	out io.Writer
	err io.Writer

	eventCh chan *TaskEvent
	quitCh  chan struct{}

	options *FormatOptions
}

type FormatOptions struct {
	showTime bool
}

func New(out io.Writer, err io.Writer, showTime bool) *Renderer {
	return &Renderer{
		out: out,
		err: err,
		options: &FormatOptions{
			showTime: showTime,
		},
	}
}

func (r *Renderer) RenderEvents(eventCh chan *TaskEvent, quitCh chan struct{}) error {

	for {
		select {
		case event := <-eventCh:
			r.renderEvent(event)
		case <-quitCh:
			return nil
		}
	}
}

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

	if event.Data != nil {
		if progress, ok := event.Data[progressKey]; ok {
			switch t := progress.(type) {
			case string:
				s += fmt.Sprintf("Progress: %s", t)
			case int:
				s += fmt.Sprintf("Progress: %d", t)
			case float32:
			case float64:
				i := int(t)
				s += fmt.Sprintf("Progress: %d", i)
			default:
				// do nothing, just being explicit
			}
		}
	}
	fmt.Fprintf(r.out, "%s\n", s)
}
