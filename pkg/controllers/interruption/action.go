package interruption

import "fmt"

type Action byte

const (
	_ Action = iota
	CordonAndDrain
	NoAction
)

func (a Action) String() string {
	switch a {
	case CordonAndDrain:
		return "CordonAndDrain"
	case NoAction:
		return "NoAction"
	default:
		return fmt.Sprintf("Unsupported Action %d", a)
	}
}
