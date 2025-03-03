package core

import "encoding/json"

type NodeID string
type PortName string
type Payload map[string]any

// Node interface — implementations live in pkg/nodes/
type Node interface {
	ID() NodeID
	OnEvent(port PortName, in Payload) map[PortName][]Payload
	Metadata() any
}

// factory registry for JSON round-trip
type factory func(json.RawMessage) (Node, error)

var registry = map[string]factory{}

func Register(kind string, f factory) { registry[kind] = f }

