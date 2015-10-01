// Copyright 2015 Apcera Inc. All rights reserved.

package taskrenderer

const (
	progressCurrent = "progress"
	progressTotal   = "total"
)

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
	// or "link_job_1_and_2".
	Thread string `json:""`

	// Stage indicates a logical grouping of subtasks.
	// A stage could be "creating_job", or "downloading_packages".
	Stage string `json:"stage"`

	// Subtask provides a description of the subtask that
	// this TaskEvent describes.
	Subtask string `json:"subtask"`

	// Index is the index of this subtask in the total
	// number of subtasks for the current stage.
	Index int32 `json:"index"`

	// Total is the total number of subtasks for a current stage.
	Total int32 `json:"total"`

	// Tags provide a hint as to what is being tracked.
	Tags []string `json:"tags"`
	// CurrentProgress indicates the current state of progress for
	// this subtask.
	CurrentProgress uint64 `json:""`

	// TotalProgress indicates the total size of progress for this
	// subtask.
	TotalProgress uint64 `json:""`

	// Data is extra information about this TaskEvent.
	Data map[string]interface{} `json:"data"`
}
