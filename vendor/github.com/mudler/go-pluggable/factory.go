// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
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
	"io"
	"io/ioutil"
)

type FactoryPlugin struct {
	EventType     EventType
	PluginHandler PluginHandler
}

func NewPluginFactory(p ...FactoryPlugin) PluginFactory {
	f := make(PluginFactory)
	for _, pp := range p {
		f.Add(pp.EventType, pp.PluginHandler)
	}
	return f
}

// PluginHandler represent a generic plugin which
// talks go-pluggable API
// It receives an event, and is always expected to give a response
type PluginHandler func(*Event) EventResponse

// PluginFactory is a collection of handlers for a given event type.
// a plugin has to respond to multiple events and it always needs to return an
// Event response as result
type PluginFactory map[EventType]PluginHandler

// Run runs the PluginHandler given a event type and a payload
//
// The result is written to the writer provided
// as argument.
func (p PluginFactory) Run(name EventType, payload string, w io.Writer) error {
	ev := &Event{}

	if err := json.Unmarshal([]byte(payload), ev); err != nil {
		return err
	}

	if ev.File != "" {
		c, err := ioutil.ReadFile(ev.File)
		if err != nil {
			return err
		}

		ev.Data = string(c)
	}

	resp := EventResponse{}
	for e, r := range p {
		if name == e {
			resp = r(ev)
		}
	}

	dat, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	_, err = w.Write(dat)
	return err
}

// Add associates an handler to an event type
func (p PluginFactory) Add(ev EventType, ph PluginHandler) {
	p[ev] = ph
}
