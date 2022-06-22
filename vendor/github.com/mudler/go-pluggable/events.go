// Copyright © 2020 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package pluggable

import (
	"encoding/json"
	"fmt"
)

// EventType describes an event type
type EventType string

// Event describes the event structure.
// Contains a Name field and a Data field which
// is marshalled in JSON
type Event struct {
	Name EventType `json:"name"`
	Data string    `json:"data"`
	File string    `json:"file"` // If Data >> 10K write content to file instead
}

// EventResponse describes the event response structure
// It represent the JSON response from plugins
type EventResponse struct {
	State string `json:"state"`
	Data  string `json:"data"`
	Error string `json:"error"`
}

// JSON returns the stringified JSON of the Event
func (e Event) JSON() (string, error) {
	dat, err := json.Marshal(e)
	return string(dat), err
}

// Copy returns a copy of Event
func (e Event) Copy() *Event {
	copy := &e
	return copy
}

func (e Event) ResponseEventName(s string) EventType {
	return EventType(fmt.Sprintf("%s-%s", e.Name, s))
}

// Unmarshal decodes the json payload in the given parameteer
func (r EventResponse) Unmarshal(i interface{}) error {
	return json.Unmarshal([]byte(r.Data), i)
}

// Errored returns true if the response contains an error
func (r EventResponse) Errored() bool {
	return len(r.Error) != 0
}

// NewEvent returns a new event which can be used for publishing
// the obj gets automatically serialized in json.
func NewEvent(name EventType, obj interface{}) (*Event, error) {
	dat, err := json.Marshal(obj)
	return &Event{Name: name, Data: string(dat)}, err
}
