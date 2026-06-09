// This file is part of rpn, a simple and useful CLI RPN calculator.
// For further information, check https://github.com/marcopaganini/rpn
//
// (C) Sep/2024 by Marco Paganini <paganini AT paganini DOT net>
package main

import (
	"errors"
	"fmt"

	"github.com/ericlagergren/decimal"
)

type (
	// variablesType holds a map of variable names to decimal values.
	variablesType struct {
		vars map[string]*decimal.Big
	}
)

// newVariablesType creates a new variables store.
func newVariablesType() *variablesType {
	return &variablesType{
		vars: make(map[string]*decimal.Big),
	}
}

// set stores a value in a variable.
func (v *variablesType) set(name string, value *decimal.Big) error {
	if name == "" {
		return errors.New("variable name cannot be empty")
	}
	// Make a copy of the value to avoid external modifications.
	v.vars[name] = big().Copy(value)
	return nil
}

// get retrieves a value from a variable.
func (v *variablesType) get(name string) (*decimal.Big, error) {
	if value, ok := v.vars[name]; ok {
		// Return a copy to avoid external modifications.
		return big().Copy(value), nil
	}
	return nil, fmt.Errorf("variable %q not found", name)
}

// exists checks if a variable exists.
func (v *variablesType) exists(name string) bool {
	_, ok := v.vars[name]
	return ok
}

// delete removes a variable.
func (v *variablesType) delete(name string) error {
	if !v.exists(name) {
		return fmt.Errorf("variable %q not found", name)
	}
	delete(v.vars, name)
	return nil
}

// list returns all variable names sorted.
func (v *variablesType) list() map[string]*decimal.Big {
	result := make(map[string]*decimal.Big)
	for name, value := range v.vars {
		result[name] = big().Copy(value)
	}
	return result
}

// clear removes all variables.
func (v *variablesType) clear() {
	v.vars = make(map[string]*decimal.Big)
}
