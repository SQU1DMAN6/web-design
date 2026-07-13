package event

import (
	"encoding/json"
	"inkdrop/config"
	"inkdrop/model"
	"sync"
)

// Event represents a system event.
type Event struct {
	Type      string                 `json:"type"`
	DropID    string                 `json:"drop_id"`
	UserID    int64                  `json:"user_id"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventListener is a callback for event notifications.
type EventListener func(Event)

// Dispatcher manages event distribution.
type Dispatcher struct {
	mu        sync.RWMutex
	listeners []EventListener
}

// GlobalDispatcher is the application-wide event dispatcher.
var GlobalDispatcher = NewDispatcher()

// NewDispatcher creates a new event dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

// Subscribe adds a listener for all events.
func (d *Dispatcher) Subscribe(listener EventListener) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners = append(d.listeners, listener)
}

// Dispatch sends an event to all listeners and persists it.
func (d *Dispatcher) Dispatch(event Event) {
	// Persist to activity log
	db := config.GetDB()
	if db != nil {
		metaJSON := ""
		if len(event.Data) > 0 {
			b, err := json.Marshal(event.Data)
			if err == nil {
				metaJSON = string(b)
			}
		}
		model.LogActivity(db, &model.ActivityLog{
			DropID:    event.DropID,
			UserID:    event.UserID,
			EventType: event.Type,
			Metadata:  metaJSON,
		})
	}

	// Notify listeners
	d.mu.RLock()
	listeners := make([]EventListener, len(d.listeners))
	copy(listeners, d.listeners)
	d.mu.RUnlock()

	for _, listener := range listeners {
		listener(event)
	}
}

// DispatchEvent creates and dispatches an event in one call.
func DispatchEvent(eventType, dropID string, userID int64, data map[string]interface{}) {
	GlobalDispatcher.Dispatch(Event{
		Type:   eventType,
		DropID: dropID,
		UserID: userID,
		Data:   data,
	})
}
