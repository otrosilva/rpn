// This file is part of rpn, a simple and useful CLI RPN calculator.
// For further information, check https://github.com/marcopaganini/rpn
//
// (C) Sep/2024 by Marco Paganini <paganini AT paganini DOT net>
package main

import (
	"errors"
	"fmt"
)

type (
	// wordType represents a user-defined word (function)
	wordType struct {
		name  string   // name of the word
		ops   []string // sequence of operations/tokens to execute
	}

	// wordsType holds a map of word names to word definitions
	wordsType struct {
		words map[string]*wordType
	}
)

// newWordsType creates a new words store.
func newWordsType() *wordsType {
	return &wordsType{
		words: make(map[string]*wordType),
	}
}

// define creates a new word with the given name and operations.
func (w *wordsType) define(name string, ops []string) error {
	if name == "" {
		return errors.New("word name cannot be empty")
	}
	if !isValidVariableName(name) {
		return fmt.Errorf("invalid word name: %q. Must start with a letter", name)
	}
	
	if len(ops) == 0 {
		return errors.New("word definition cannot be empty")
	}

	w.words[name] = &wordType{
		name: name,
		ops:  ops,
	}
	return nil
}

// get retrieves a word definition.
func (w *wordsType) get(name string) (*wordType, error) {
	if word, ok := w.words[name]; ok {
		return word, nil
	}
	return nil, fmt.Errorf("word %q not defined", name)
}

// exists checks if a word is defined.
func (w *wordsType) exists(name string) bool {
	_, ok := w.words[name]
	return ok
}

// delete removes a word definition.
func (w *wordsType) delete(name string) error {
	if !w.exists(name) {
		return fmt.Errorf("word %q not defined", name)
	}
	delete(w.words, name)
	return nil
}

// list returns all word names and their definitions.
func (w *wordsType) list() map[string]*wordType {
	result := make(map[string]*wordType)
	for name, word := range w.words {
		// Create a copy
		newWord := &wordType{
			name: word.name,
			ops:  append([]string{}, word.ops...),
		}
		result[name] = newWord
	}
	return result
}

// clear removes all word definitions.
func (w *wordsType) clear() {
	w.words = make(map[string]*wordType)
}
