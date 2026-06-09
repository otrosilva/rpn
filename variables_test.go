// This file is part of rpn, a simple and useful CLI RPN calculator.
// For further information, check https://github.com/marcopaganini/rpn
//
// (C) Sep/2024 by Marco Paganini <paganini AT paganini DOT net>
package main

import (
	"testing"

	"github.com/ericlagergren/decimal"
)

func TestVariablesSet(t *testing.T) {
	vars := newVariablesType()
	val := bigFloat("72.5")
	
	err := vars.set("RUBUSD", val)
	if err != nil {
		t.Fatalf("Failed to set variable: %v", err)
	}
	
	retrieved, err := vars.get("RUBUSD")
	if err != nil {
		t.Fatalf("Failed to get variable: %v", err)
	}
	
	if retrieved.Cmp(val) != 0 {
		t.Errorf("Variable value mismatch. Expected %v, got %v", val, retrieved)
	}
}

func TestVariablesGet(t *testing.T) {
	vars := newVariablesType()
	val := bigFloat("100")
	vars.set("TEST", val)
	
	retrieved, err := vars.get("TEST")
	if err != nil {
		t.Fatalf("Failed to get variable: %v", err)
	}
	
	if retrieved.Cmp(val) != 0 {
		t.Errorf("Variable value mismatch. Expected %v, got %v", val, retrieved)
	}
}

func TestVariablesGetNonexistent(t *testing.T) {
	vars := newVariablesType()
	
	_, err := vars.get("NONEXISTENT")
	if err == nil {
		t.Error("Expected error for nonexistent variable, but got none")
	}
}

func TestVariablesExists(t *testing.T) {
	vars := newVariablesType()
	val := bigFloat("50")
	vars.set("EXISTS", val)
	
	if !vars.exists("EXISTS") {
		t.Error("Variable should exist")
	}
	
	if vars.exists("NONEXISTENT") {
		t.Error("Variable should not exist")
	}
}

func TestVariablesDelete(t *testing.T) {
	vars := newVariablesType()
	val := bigFloat("100")
	vars.set("TODELETE", val)
	
	if !vars.exists("TODELETE") {
		t.Error("Variable should exist before deletion")
	}
	
	err := vars.delete("TODELETE")
	if err != nil {
		t.Fatalf("Failed to delete variable: %v", err)
	}
	
	if vars.exists("TODELETE") {
		t.Error("Variable should not exist after deletion")
	}
}

func TestVariablesClear(t *testing.T) {
	vars := newVariablesType()
	vars.set("VAR1", bigFloat("1"))
	vars.set("VAR2", bigFloat("2"))
	vars.set("VAR3", bigFloat("3"))
	
	if len(vars.list()) != 3 {
		t.Errorf("Expected 3 variables, got %d", len(vars.list()))
	}
	
	vars.clear()
	
	if len(vars.list()) != 0 {
		t.Errorf("Expected 0 variables after clear, got %d", len(vars.list()))
	}
}

func TestVariablesList(t *testing.T) {
	vars := newVariablesType()
	vars.set("A", bigFloat("10"))
	vars.set("B", bigFloat("20"))
	vars.set("C", bigFloat("30"))
	
	list := vars.list()
	
	if len(list) != 3 {
		t.Errorf("Expected 3 variables in list, got %d", len(list))
	}
	
	expectedValues := map[string]string{
		"A": "10",
		"B": "20",
		"C": "30",
	}
	
	for name, expectedVal := range expectedValues {
		if val, ok := list[name]; !ok {
			t.Errorf("Variable %q not found in list", name)
		} else {
			expected := bigFloat(expectedVal)
			if val.Cmp(expected) != 0 {
				t.Errorf("Variable %q value mismatch. Expected %v, got %v", name, expected, val)
			}
		}
	}
}

func TestIsValidVariableName(t *testing.T) {
	testCases := []struct {
		name      string
		valid     bool
	}{
		{"RUBUSD", true},
		{"Test", true},
		{"_invalid", false},
		{"123invalid", false},
		{"valid_name", true},
		{"", false},
		{"VAR_123", true},
		{"2VAR", false},
	}
	
	for _, tc := range testCases {
		result := isValidVariableName(tc.name)
		if result != tc.valid {
			t.Errorf("isValidVariableName(%q) = %v, expected %v", tc.name, result, tc.valid)
		}
	}
}

// Helper function to create decimal.Big from float string
func bigFloat(s string) *decimal.Big {
	var d decimal.Big
	d.SetString(s)
	return &d
}
