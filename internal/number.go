package internal

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Num is a number that may have a comment attached to it
type NumWithComment struct {
	Value   decimal.Decimal
	Comment string
}

// String returns the formatted number with comment if present
func (n NumWithComment) String() string {
	if n.Comment == "" {
		return n.Value.String()
	}
	return fmt.Sprintf("%s # %s", n.Value.String(), n.Comment)
}

// GetValue returns just the numeric value
func (n NumWithComment) GetValue() decimal.Decimal {
	return n.Value
}

// SetComment updates the comment
func (n *NumWithComment) SetComment(comment string) {
	n.Comment = comment
}
