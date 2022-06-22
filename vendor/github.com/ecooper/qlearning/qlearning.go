// Package qlearning is an experimental set of interfaces and helpers to
// implement the Q-learning algorithm in Go.
//
// This is highly experimental and should be considered a toy.
//
// See https://github.com/ecooper/qlearning/tree/master/examples for
// implementation examples.
package qlearning

import (
	"fmt"
	"math/rand"
	"time"
)

// State is an interface wrapping the current state of the model.
type State interface {

	// String returns a string representation of the given state.
	// Implementers should take care to insure that this is a consistent
	// hash for a given state.
	String() string

	// Next provides a slice of possible Actions that could be applied to
	// a state.
	Next() []Action
}

// Action is an interface wrapping an action that can be applied to the
// model's current state.
//
// BUG (ecooper): A state should apply an action, not the other way
// around.
type Action interface {
	String() string
	Apply(State) State
}

// Rewarder is an interface wrapping the ability to provide a reward
// for the execution of an action in a given state.
type Rewarder interface {
	// Reward calculates the reward value for a given action in a given
	// state.
	Reward(action *StateAction) float32
}

// Agent is an interface for a model's agent and is able to learn
// from actions and return the current Q-value of an action at a given state.
type Agent interface {
	// Learn updates the model for a given state and action, using the
	// provided Rewarder implementation.
	Learn(*StateAction, Rewarder)

	// Value returns the current Q-value for a State and Action.
	Value(State, Action) float32

	// Return a string representation of the Agent.
	String() string
}

// StateAction is a struct grouping an action to a given State. Additionally,
// a Value can be associated to StateAction, which is typically the Q-value.
type StateAction struct {
	State  State
	Action Action
	Value  float32
}

// NewStateAction creates a new StateAction for a State and Action.
func NewStateAction(state State, action Action, val float32) *StateAction {
	return &StateAction{
		State:  state,
		Action: action,
		Value:  val,
	}
}

// Next uses an Agent and State to find the highest scored Action.
//
// In the case of Q-value ties for a set of actions, a random
// value is selected.
func Next(agent Agent, state State) *StateAction {
	best := make([]*StateAction, 0)
	bestVal := float32(0.0)

	for _, action := range state.Next() {
		val := agent.Value(state, action)

		if bestVal == float32(0.0) {
			best = append(best, NewStateAction(state, action, val))
			bestVal = val
		} else {
			if val > bestVal {
				best = []*StateAction{NewStateAction(state, action, val)}
				bestVal = val
			} else if val == bestVal {
				best = append(best, NewStateAction(state, action, val))
			}
		}
	}

	return best[rand.Intn(len(best))]
}

// SimpleAgent is an Agent implementation that stores Q-values in a
// map of maps.
type SimpleAgent struct {
	q  map[string]map[string]float32
	lr float32
	d  float32
}

// NewSimpleAgent creates a SimpleAgent with the provided learning rate
// and discount factor.
func NewSimpleAgent(lr, d float32) *SimpleAgent {
	return &SimpleAgent{
		q:  make(map[string]map[string]float32),
		d:  d,
		lr: lr,
	}
}

// getActions returns the current Q-values for a given state.
func (agent *SimpleAgent) getActions(state string) map[string]float32 {
	if _, ok := agent.q[state]; !ok {
		agent.q[state] = make(map[string]float32)
	}

	return agent.q[state]
}

// Learn updates the existing Q-value for the given State and Action
// using the Rewarder.
//
// See https://en.wikipedia.org/wiki/Q-learning#Algorithm
func (agent *SimpleAgent) Learn(action *StateAction, reward Rewarder) {
	current := action.State.String()
	next := action.Action.Apply(action.State).String()

	actions := agent.getActions(current)

	maxNextVal := float32(0.0)
	for _, v := range agent.getActions(next) {
		if v > maxNextVal {
			maxNextVal = v
		}
	}

	currentVal := actions[action.Action.String()]
	actions[action.Action.String()] = currentVal + agent.lr*(reward.Reward(action)+agent.d*maxNextVal-currentVal)
}

// Value gets the current Q-value for a State and Action.
func (agent *SimpleAgent) Value(state State, action Action) float32 {
	return agent.getActions(state.String())[action.String()]
}

// String returns the current Q-value map as a printed string.
//
// BUG (ecooper): This is useless.
func (agent *SimpleAgent) String() string {
	return fmt.Sprintf("%v", agent.q)
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
