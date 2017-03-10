// Copyright 2015 Apcera Inc. All rights reserved.

package taskrenderer

// A TaskEvent carries structured information about events that occur during
// processing. This information will be persisted, and then forwarded to
// clients. The structure allows for the client to render TaskEvents in
// a predictable manner.
type TaskEvent struct {
	// TaskUUID is the UUID of the Task that stores this event.
	// This may be redundant and removable.
	TaskUUID string `json:"task_uuid"`

	// Type is the type of message this TaskEvent carries.
	// Can be a plain TASK_EVENT, TASK_EVENT_CLIENT_ERROR,
	// TASK_EVENT_SERVER_ERROR, TASK_EVENT_CANCEL, or TASK_EVENT_EOS.
	Type string `json:"task_event_type"`

	// Time will preferably be in unix nanosecond time, and should be the time
	// immediately before the TaskEvent gets announced on NATS
	// for persistence, this way the order of TaskEvents can
	// be reconstructed regardless of latency between components.
	Time int64 `json:"time"`

	// Thread represents a logically independent procedure within
	// a Task. For instance, a thread could be "job1" or "job2"
	// or "Link job1 and job2".
	Thread string `json:"thread"`

	// Stage indicates a logical grouping of subtasks.
	// In Continuum, a stage could be "Creating Job",
	// or "Downloading Packages".
	Stage string `json:"stage"`

	// Subtask provides a description of the subtask that
	// this TaskEvent describes.
	Subtask Subtask `json:"subtask"`

	// Tags provide a hint as to what is being tracked.
	Tags []string `json:"tags"`

	// Payload is extra information about this TaskEvent.
	Payload map[string]interface{} `json:"payload"`
}

// Subtask is any discrete piece of work belonging to a stage.
// A stage "Creating Package" could contain subtasks of
// "Checking for Package Existence", "Creating Package", and
// "Uploading Tarball".
type Subtask struct {
	// Name is the human-readable description of this subtask.
	Name string `json:"name"`

	// Index is the index of this subtask among all subtasks.
	Index int `json:"index"`

	// Total is the total number of subtasks in the current stage.
	Total int `json:"total"`

	// Progress indicates how far along this unit of work is.
	Progress Progress `json:"progress"`
}

// Progress indicates how far along in processing the current subtask is.
type Progress struct {
	// Current is the current progress.
	Current uint64 `json:"current"`

	// Total is the total amount of work to be done.
	Total uint64 `json:"total"`
}
