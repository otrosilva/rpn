package internal

import (
	"errors"
	"fmt"
	"slices"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

//
// The calculator, which has a stack, a history and can run commands.
//

type StackItem struct {
	Value   decimal.Decimal
	Comment string
}

type Calculator struct {
	stack   []StackItem
	history []string
	undo    [][]StackItem
}

func NewCalculator() *Calculator {
	return &Calculator{}
}

//
// accessors
//

func (c *Calculator) GetStack() []Num {
	result := make([]Num, len(c.stack))
	for i, item := range c.stack {
		result[i] = item.Value
	}
	return result
}

func (c *Calculator) GetStackString() []string {
	return MapV(c.GetStack(), func(x Num) string { return x.String() })
}

func (c *Calculator) GetHistory() []string {
	return c.history
}

func (c *Calculator) SetStack(stack []Num) {
	c.stack = make([]StackItem, len(stack))
	for i, val := range stack {
		c.stack[i] = StackItem{Value: val, Comment: ""}
	}
}

func (c *Calculator) SetStackString(stack []string) {
	c.SetStack(MapV(stack, decimal.RequireFromString))
}

func (c *Calculator) SetHistory(history []string) {
	c.history = history
}

func (c *Calculator) GetUndo() [][]StackItem {
	return c.undo
}

// returns the 8 visible lines of the stack
func (c *Calculator) GetDisplay() []string {
	result := make([]string, StackSize)
	for ii := range StackSize {
		var s = fmt.Sprintf("%d: ", StackSize-ii)
		si := c.Len() - (StackSize - ii)
		if si >= 0 {
			item := c.stack[si]
			if item.Comment != "" {
				s += fmt.Sprintf("%s # %s", item.Value.String(), item.Comment)
			} else {
				s += item.Value.String()
			}
		}
		result[ii] = s
	}
	return result
}

//
// undo
//

func (c *Calculator) snapshotForUndo() {
	c.undo = TruncateStart(Push(c.undo, slices.Clone(c.stack)), UndoSize)
}

func (c *Calculator) Undo() {
	c.stack, c.undo = Pop(c.undo)
}

//
// accessors
//

func (c *Calculator) Clear() {
	c.stack = nil
	c.history = nil
}

func (c *Calculator) History() []string {
	return c.history
}

func (c *Calculator) Enter(value Num, explicit bool) {
	if explicit {
		c.snapshotForUndo()
	}
	c.Push(value)
}

// AddCommentToTop adds a comment to the top stack item and updates history if it has a previous operation
func (c *Calculator) AddCommentToTop(comment string) error {
	if c.Len() == 0 {
		return errors.New("stack is empty")
	}
	topIndex := len(c.stack) - 1
	c.stack[topIndex].Comment = comment
	
	// If there's history, append the comment to the last history entry
	if len(c.history) > 0 {
		lastHistory := c.history[len(c.history)-1]
		// Append comment to the last history entry
		c.history[len(c.history)-1] = fmt.Sprintf("%s # %s", lastHistory, comment)
	}
	
	return nil
}

//
// stack operations
//

func (c *Calculator) Empty() bool {
	return c.Len() == 0
}

func (c *Calculator) Len() int {
	return len(c.stack)
}

func (c *Calculator) Push(values ...Num) {
	var normalized = MapV(values, Normalize)
	stackItems := make([]StackItem, len(normalized))
	for i, val := range normalized {
		stackItems[i] = StackItem{Value: val, Comment: ""}
	}
	c.stack = TruncateStart(Push(c.stack, stackItems...), MaxArraySize)
}

func (c *Calculator) Pop() Num {
	if len(c.stack) == 0 {
		return decimal.Zero
	}
	var x StackItem
	x, c.stack = Pop(c.stack)
	return x.Value
}

func (c *Calculator) Peek() Num {
	return lo.Must(lo.Last(c.stack)).Value
}

//
// these are handy
//

func (c *Calculator) PushInt(values ...int) {
	c.Push(MapV(values, func(x int) Num { return decimal.NewFromInt(int64(x)) })...)
}

func (c *Calculator) PushFloat64(values ...float64) {
	c.Push(MapV(values, decimal.NewFromFloat)...)
}

func (c *Calculator) PopInt() int {
	return int(c.Pop().IntPart())
}

func (c *Calculator) PopFloat64() float64 {
	return c.Pop().InexactFloat64()
}

func (c *Calculator) PeekInt() int {
	return int(c.Peek().IntPart())
}

func (c *Calculator) PeekFloat() float64 {
	return c.Peek().InexactFloat64()
}

//
// history operations
//

func (c *Calculator) AddHistory(s string) {
	c.history = TruncateStart(Push(c.history, s), MaxArraySize)
}

//
// Run a command by name
//

func (c *Calculator) Run(name string) error {
	cmd, ok := CommandsByName[name]
	if !ok {
		panic("unknown command " + name)
	}

	//
	// do we have enough on the stack to run this command?
	//

	switch cmd.fn.(type) {
	case func(*Calculator), func(*Calculator) Num:
		// nop
	case func(*Calculator, Num), func(*Calculator, Num) Num:
		if c.Len() < 1 {
			return errors.New("stack is empty")
		}
	case func(*Calculator, Num, Num), func(*Calculator, Num, Num) Num:
		if c.Len() < 2 {
			return errors.New("too few arguments")
		}
	default:
		panic("unknown command fn sig " + name)
	}

	//
	// is the cmd ready to go? for example, can't ADD without at least two
	// values on the stack
	//

	if cmd.valid != nil {
		if err := cmd.valid(c); err != nil {
			return err
		}
	}
	if cmd.Name != "UNDO" {
		c.snapshotForUndo()
	}

	//
	// excellent! call fn and generate history
	//

	var history string

	switch fn := cmd.fn.(type) {
	case func(*Calculator):
		fn(c)
	case func(*Calculator, Num):
		fn(c, c.Pop())
	case func(*Calculator) Num:
		c.Push(fn(c))
	case func(*Calculator, Num) Num:
		a := c.Pop()
		c.Push(fn(c, a))
		if cmd.fmt != "" {
			history = fmt.Sprintf(cmd.fmt, a, c.Peek())
		}
	case func(*Calculator, Num, Num):
		b, a := c.Pop(), c.Pop()
		fn(c, a, b)
		if cmd.fmt != "" {
			history = fmt.Sprintf(cmd.fmt, a, b)
		}
	case func(*Calculator, Num, Num) Num:
		b, a := c.Pop(), c.Pop()
		c.Push(fn(c, a, b))
		if cmd.fmt != "" {
			history = fmt.Sprintf(cmd.fmt, a, b, c.Peek())
		}
	default:
		panic("unknown command fn sig " + name)
	}

	//
	// append to history
	//

	if history != "" {
		c.AddHistory(history)
	}

	return nil
}
