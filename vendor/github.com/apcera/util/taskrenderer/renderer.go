// Copyright 2015 Apcera Inc. All rights reserved.

package taskrenderer

import (
	"fmt"
	"time"
)

// Renderer holds all of the state necessary to gather and output TaskEvents.
type Renderer struct {
	options *FormatOptions
}

// FormatOptions provides a way to customize the output of TaskEvents.
type FormatOptions struct {
	showTime bool
}

// New instantiates and returns a new Renderer object.
func New(showTime bool) *Renderer {
	return &Renderer{
		options: &FormatOptions{
			showTime: showTime,
		},
	}
}

// RenderEvents reads every event sent on the given channel and renders it.
// The channel can be closed by the caller at any time to stop rendering.
func (r *Renderer) RenderEvents(eventCh <-chan *TaskEvent) {
	for event := range eventCh {
		r.RenderEvent(event)
	}
}

// renderEvent varies output depending on the information provided
// by the current taskEvent.
func (r *Renderer) RenderEvent(event *TaskEvent) string {
	switch event.Type {
	case "eos":
		return ""
	}

	s := ""

	if r.options.showTime {
		s += fmt.Sprintf("(%s) ", time.Unix(0, event.Time).Format(time.UnixDate))
	}

	if event.Thread != "" {
		s += fmt.Sprintf("[%s] -- ", event.Thread)
	}

	s += fmt.Sprintf("%s", event.Stage)

	if event.Subtask.Name != "" {
		s += " -- "
		if event.Subtask.Total != 0 {
			s += fmt.Sprintf("(%d/%d): ", event.Subtask.Index, event.Subtask.Total)
		}

		s += fmt.Sprintf("%s", event.Subtask.Name)

		if event.Subtask.Progress.Total != 0 {
			s += fmt.Sprintf(" ... %d%%",
				(event.Subtask.Progress.Current/event.Subtask.Progress.Total)*100,
			)
		}
	}

	return s
}
