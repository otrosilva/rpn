// This file is part of rpn, a simple and useful CLI RPN calculator.
// For further information, check https://github.com/marcopaganini/rpn
//
// (C) Sep/2024 by Marco Paganini <paganini AT paganini DOT net>
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/ericlagergren/decimal"
	"github.com/fatih/color"
)

var (
	// Build is filled by go build -ldflags during build.
	Build        string
	programTitle = "rpn - a simple CLI RPN calculator"

	// These are functions to be used to print in color.
	errorMsg = color.New(color.FgRed).SprintFunc()
	warnMsg  = color.New(color.FgMagenta).SprintFunc()
	bold     = color.New(color.Bold).SprintFunc()
)

// atof takes a string as an argument and return a decimal object representing
// that string. Strings starting in 0x or 0X are treated as hex strings.
// Strings starting in o or 0 are treated as octal strings. Non decimal strings
// are converted to a uint64 intermediate representation and thus limited to
// how much a uint64 can hold.
func atof(s string) (*decimal.Big, error) {
	base := 10
	switch {
	case (strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B")) && len(s) > 2:
		s = s[2:]
		base = 2
	case (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) && len(s) > 2:
		s = s[2:]
		base = 16
	// Numbers starting with 0 must account for 0.xx fractional numbers not
	// being octal numbers.
	case (strings.HasPrefix(s, "0") || strings.HasPrefix(s, "o")) && !strings.HasPrefix(s, "0.") && len(s) > 1:
		s = s[1:]
		base = 8
	}

	if base == 10 {
		var d decimal.Big
		if _, ok := d.SetString(s); !ok || d.IsNaN(0) {
			return nil, errors.New("unable to convert number")
		}
		return &d, nil
	}

	// Non-base 10 numbers are limited to uint64 sizes.
	ret, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return nil, err
	}
	return bigUint(ret), nil
}

// isValidVariableName checks if a string is a valid variable name.
// Variables must start with a letter and contain only alphanumeric characters and underscores.
func isValidVariableName(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must start with a letter
	if !isLetter(rune(s[0])) {
		return false
	}
	// Rest can be letters, digits, or underscores
	for _, c := range s {
		if !isAlphanumericOrUnderscore(c) {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isAlphanumericOrUnderscore(r rune) bool {
	return isLetter(r) || (r >= '0' && r <= '9') || r == '_'
}

// maxWordDepth is the maximum recursion depth for word (function) calls.
const maxWordDepth = 100

// executeWord runs a user-defined word by name, resolving nested word calls
// recursively up to maxWordDepth levels deep.
func executeWord(name string, depth int, stack *stackType, opmap opmapType, vars *variablesType, words *wordsType) error {
	if depth > maxWordDepth {
		return fmt.Errorf("maximum word call depth (%d) exceeded — possible infinite recursion", maxWordDepth)
	}

	word, err := words.get(name)
	if err != nil {
		return err
	}

	for _, op := range word.ops {
		// Try as a built-in operator.
		if h, ok := opmap[op]; ok {
			if _, _, err := operation(h, stack); err != nil {
				return fmt.Errorf("in word %q: %w", name, err)
			}
			continue
		}

		// Try as a variable.
		if val, err := vars.get(op); err == nil {
			stack.push(val)
			continue
		}

		// Try as another word (recursive call).
		if words.exists(op) {
			if err := executeWord(op, depth+1, stack, opmap, vars, words); err != nil {
				return fmt.Errorf("in word %q: %w", name, err)
			}
			continue
		}

		// Try as a number.
		if n, err := atof(op); err == nil {
			stack.push(n)
			continue
		}

		return fmt.Errorf("in word %q: unknown operation %q", name, op)
	}
	return nil
}

// calc contains the bulk of the calculator code. It takes a stack and an
// optional string argument. If string the string is not empty, it executes the
// oeprations in the string and returns. If the string is empty, it enters a
// readline loop accepting commands from the user.
func calc(stack *stackType, cmd string) error {
	// Wait for entry until Ctrl-D or q is issued.
	var (
		line string
		err  error
		rl   *readline.Instance
	)

	ctx := decimal.Context128

	// Single command execution?
	single := (cmd != "")

	// Variables storage
	vars := newVariablesType()

	// Words (user-defined functions) storage
	words := newWordsType()

	// Operations
	ops := newOpsType(ctx, stack, vars, words)
	opmap := ops.opmap()

	if !single {
		rl, err = readline.New("> ")
		if err != nil {
			log.Fatal(err)
		}
		defer rl.Close()
	}

	// Remove all extraneous characters from the input. This will silently
	// remove undesirable formatting characters, making cut/paste operations
	// simpler. If you add a new operation as a single special character, make
	// sure it's represented here.
	cleanRe := regexp.MustCompile(`[^-+./*%^=:;[:alnum:]_\s]`)

	for {
		// Save a copy of the stack so we can restore it to the previous state
		// before this line was processed (in case of errors.)
		stack.save()

		if ops.debug {
			stack.print(ctx, ops.base, ops.decimals)
		}

		// By default, use the passed command. If no command, initialize readline.
		line = cmd
		if !single {
			line, err = rl.Readline()
			if err != nil { // io.EOF
				break
			}
		}
		// Comment?
		if strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(line)
		line = cleanRe.ReplaceAllString(line, "")

		tokens := strings.Fields(line)

		// Check for word definition: func <name> ... ;
		if len(tokens) >= 3 && (tokens[0] == "func" || tokens[0] == "FUNC") {
			semiIdx := -1
			for i, t := range tokens {
				if t == ";" {
					semiIdx = i
					break
				}
			}
			if semiIdx <= 1 {
				fmt.Printf(errorMsg("ERROR: Invalid word definition. Expected: func <name> <operations> ;\n"))
				stack.restore()
				continue
			}
			wordName := tokens[1]
			if !isValidVariableName(wordName) {
				fmt.Printf(errorMsg("ERROR: Invalid word name: %q. Must start with a letter.\n"), wordName)
				stack.restore()
				continue
			}
			wordOps := tokens[2:semiIdx]
			if len(wordOps) == 0 {
				fmt.Printf(errorMsg("ERROR: Word definition cannot be empty.\n"))
				stack.restore()
				continue
			}
			if err := words.define(wordName, wordOps); err != nil {
				fmt.Printf(errorMsg("ERROR: %v\n"), err)
				stack.restore()
			} else {
				fmt.Printf(warnMsg("Word %q defined with %d operations\n"), wordName, len(wordOps))
			}
			continue
		}

		// Check for variable assignment: VAR <name>
		if len(tokens) >= 2 && (tokens[0] == "var" || tokens[0] == "VAR") {
			if len(stack.list) == 0 {
				fmt.Printf(errorMsg("ERROR: Stack is empty. Cannot assign variable.\n"))
				stack.restore()
				continue
			}
			varName := tokens[1]
			if !isValidVariableName(varName) {
				fmt.Printf(errorMsg("ERROR: Invalid variable name: %q. Must start with a letter.\n"), varName)
				stack.restore()
				continue
			}
			value := stack.top()
			if err := vars.set(varName, value); err != nil {
				fmt.Printf(errorMsg("ERROR: %v\n"), err)
				stack.restore()
			} else {
				fmt.Printf(warnMsg("Variable %q set to %s\n"), varName, formatNumber(ctx, value, ops.base, ops.decimals))
			}
			continue
		}

		// Split into fields and process
		autoprint := false
		for _, token := range tokens {
			// Check operator map.
			handler, ok := opmap[token]
			if ok {
				results, remove, err := operation(handler, stack)
				if err != nil {
					if single {
						return err
					}
					fmt.Printf(errorMsg("ERROR: %v\n"), err)
					stack.restore()
					break
				}
				// If the particular handler does not ignore results from the
				// function, set autoprint to true. This will cause the top of
				// the stack results to be printed.
				autoprint = (len(results) > 0 || remove > 0)

				if !single {
					// Set readline prompt based on base and degrees/radian mode.
					switch {
					case ops.degmode:
						rl.SetPrompt("deg> ")
					case ops.base == 10:
						rl.SetPrompt("> ")
					case ops.base == 8:
						rl.SetPrompt("oct> ")
					case ops.base == 16:
						rl.SetPrompt("hex> ")
					case ops.base == 2:
						rl.SetPrompt("bin> ")
					}
				}
				continue
			}

			// Check if it's a user-defined word (supports nested calls).
			if words.exists(token) {
				if err := executeWord(token, 0, stack, opmap, vars, words); err != nil {
					if single {
						return err
					}
					fmt.Printf(errorMsg("ERROR: %v\n"), err)
					stack.restore()
					break
				}
				autoprint = true
				continue
			}

			// Help
			if token == "help" || token == "h" || token == "?" {
				if err := ops.help(); err != nil {
					fmt.Println(errorMsg(err))
				}
				continue
			}

			if token == "quit" || token == "exit" || token == "q" {
				fmt.Printf("Bye.\n")
				os.Exit(0)
			}

			// Check if it's a valid variable name (retrieve variable)
			if isValidVariableName(token) {
				if value, err := vars.get(token); err == nil {
					stack.push(value)
					autoprint = true
					continue
				}
				// If not found as a variable, fall through to number parsing
			}

			// At this point, it's either a number or not recognized.
			// If anything fails, restore stack and stop token processing.
			n, err := atof(token)
			if err != nil {
				fmt.Printf(errorMsg("Not a number or operator: %q.\n"), token)
				fmt.Println(errorMsg("Use \"help\" for online help."))
				stack.restore()
				break
			}
			// Valid number
			stack.push(n)
			continue
		}

		if autoprint {
			if single {
				fmt.Println(stack.top()) // plain print to stdout
			} else {
				stack.printTop(ctx, ops.base, ops.decimals) // pretty print to terminal
			}
		}

		// Break after the first iteration if a command is passed.
		if single {
			break
		}
	}
	return nil
}

func main() {
	stack := &stackType{}

	if err := calc(stack, strings.Join(os.Args[1:], " ")); err != nil {
		log.Fatal(err)
	}
}
// This file is part of rpn, a simple and useful CLI RPN calculator.
// For further information, check https://github.com/marcopaganini/rpn
//
// (C) Sep/2024 by Marco Paganini <paganini AT paganini DOT net>
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/ericlagergren/decimal"
	"github.com/fatih/color"
)

var (
	// Build is filled by go build -ldflags during build.
	Build        string
	programTitle = "rpn - a simple CLI RPN calculator"

	// These are functions to be used to print in color.
	errorMsg = color.New(color.FgRed).SprintFunc()
	warnMsg  = color.New(color.FgMagenta).SprintFunc()
	bold     = color.New(color.Bold).SprintFunc()
)

// atof takes a string as an argument and return a decimal object representing
// that string. Strings starting in 0x or 0X are treated as hex strings.
// Strings starting in o or 0 are treated as octal strings. Non decimal strings
// are converted to a uint64 intermediate representation and thus limited to
// how much a uint64 can hold.
func atof(s string) (*decimal.Big, error) {
	base := 10
	switch {
	case (strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B")) && len(s) > 2:
		s = s[2:]
		base = 2
	case (strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X")) && len(s) > 2:
		s = s[2:]
		base = 16
	// Numbers starting with 0 must account for 0.xx fractional numbers not
	// being octal numbers.
	case (strings.HasPrefix(s, "0") || strings.HasPrefix(s, "o")) && !strings.HasPrefix(s, "0.") && len(s) > 1:
		s = s[1:]
		base = 8
	}

	if base == 10 {
		var d decimal.Big
		if _, ok := d.SetString(s); !ok || d.IsNaN(0) {
			return nil, errors.New("unable to convert number")
		}
		return &d, nil
	}

	// Non-base 10 numbers are limited to uint64 sizes.
	ret, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		return nil, err
	}
	return bigUint(ret), nil
}

// isValidVariableName checks if a string is a valid variable name.
// Variables must start with a letter and contain only alphanumeric characters and underscores.
func isValidVariableName(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Must start with a letter
	if !isLetter(rune(s[0])) {
		return false
	}
	// Rest can be letters, digits, or underscores
	for _, c := range s {
		if !isAlphanumericOrUnderscore(c) {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isAlphanumericOrUnderscore(r rune) bool {
	return isLetter(r) || (r >= '0' && r <= '9') || r == '_'
}

// calc contains the bulk of the calculator code. It takes a stack and an
// optional string argument. If string the string is not empty, it executes the
// oeprations in the string and returns. If the string is empty, it enters a
// readline loop accepting commands from the user.
func calc(stack *stackType, cmd string) error {
	// Wait for entry until Ctrl-D or q is issued.
	var (
		line string
		err  error
		rl   *readline.Instance
	)

	ctx := decimal.Context128

	// Single command execution?
	single := (cmd != "")

	// Variables storage
	vars := newVariablesType()

	// Words (user-defined functions) storage
	words := newWordsType()

	// Operations
	ops := newOpsType(ctx, stack, vars, words)
	opmap := ops.opmap()

	if !single {
		rl, err = readline.New("> ")
		if err != nil {
			log.Fatal(err)
		}
		defer rl.Close()
	}

	// Remove all extraneous characters from the input. This will silently
	// remove undesirable formatting characters, making cut/paste operations
	// simpler. If you add a new operation as a single special character, make
	// sure it's represented here.
	cleanRe := regexp.MustCompile(`[^-+./*%^=:;[:alnum:]_\s]`)

	for {
		// Save a copy of the stack so we can restore it to the previous state
		// before this line was processed (in case of errors.)
		stack.save()

		if ops.debug {
			stack.print(ctx, ops.base, ops.decimals)
		}

		// By default, use the passed command. If no command, initialize readline.
		line = cmd
		if !single {
			line, err = rl.Readline()
			if err != nil { // io.EOF
				break
			}
		}
		// Comment?
		if strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(line)
		line = cleanRe.ReplaceAllString(line, "")

		tokens := strings.Fields(line)

		// Check for word definition: func <name> ... ;
		if len(tokens) >= 3 && (tokens[0] == "func" || tokens[0] == "FUNC") {
			semiIdx := -1
			for i, t := range tokens {
				if t == ";" {
					semiIdx = i
					break
				}
			}
			if semiIdx <= 1 {
				fmt.Printf(errorMsg("ERROR: Invalid word definition. Expected: func <name> <operations> ;\n"))
				stack.restore()
				continue
			}
			wordName := tokens[1]
			if !isValidVariableName(wordName) {
				fmt.Printf(errorMsg("ERROR: Invalid word name: %q. Must start with a letter.\n"), wordName)
				stack.restore()
				continue
			}
			wordOps := tokens[2:semiIdx]
			if len(wordOps) == 0 {
				fmt.Printf(errorMsg("ERROR: Word definition cannot be empty.\n"))
				stack.restore()
				continue
			}
			if err := words.define(wordName, wordOps); err != nil {
				fmt.Printf(errorMsg("ERROR: %v\n"), err)
				stack.restore()
			} else {
				fmt.Printf(warnMsg("Word %q defined with %d operations\n"), wordName, len(wordOps))
			}
			continue
		}

		// Check for variable assignment: VAR <name>
		if len(tokens) >= 2 && (tokens[0] == "var" || tokens[0] == "VAR") {
			if len(stack.list) == 0 {
				fmt.Printf(errorMsg("ERROR: Stack is empty. Cannot assign variable.\n"))
				stack.restore()
				continue
			}
			varName := tokens[1]
			if !isValidVariableName(varName) {
				fmt.Printf(errorMsg("ERROR: Invalid variable name: %q. Must start with a letter.\n"), varName)
				stack.restore()
				continue
			}
			value := stack.top()
			if err := vars.set(varName, value); err != nil {
				fmt.Printf(errorMsg("ERROR: %v\n"), err)
				stack.restore()
			} else {
				fmt.Printf(warnMsg("Variable %q set to %s\n"), varName, formatNumber(ctx, value, ops.base, ops.decimals))
			}
			continue
		}

		// Split into fields and process
		autoprint := false
		for _, token := range tokens {
			// Check operator map.
			handler, ok := opmap[token]
			if ok {
				results, remove, err := operation(handler, stack)
				if err != nil {
					if single {
						return err
					}
					fmt.Printf(errorMsg("ERROR: %v\n"), err)
					stack.restore()
					break
				}
				// If the particular handler does not ignore results from the
				// function, set autoprint to true. This will cause the top of
				// the stack results to be printed.
				autoprint = (len(results) > 0 || remove > 0)

				if !single {
					// Set readline prompt based on base and degrees/radian mode.
					switch {
					case ops.degmode:
						rl.SetPrompt("deg> ")
					case ops.base == 10:
						rl.SetPrompt("> ")
					case ops.base == 8:
						rl.SetPrompt("oct> ")
					case ops.base == 16:
						rl.SetPrompt("hex> ")
					case ops.base == 2:
						rl.SetPrompt("bin> ")
					}
				}
				continue
			}

			// Check if it's a user-defined word
			if word, err := words.get(token); err == nil {
				// Execute the word's operations
				for _, op := range word.ops {
					// Try as operator first
					if h, ok := opmap[op]; ok {
						_, _, err := operation(h, stack)
						if err != nil {
							fmt.Printf(errorMsg("ERROR in word %q: %v\n"), token, err)
							stack.restore()
							break
						}
						continue
					}

					// Try as variable
					if val, err := vars.get(op); err == nil {
						stack.push(val)
						continue
					}

					// Try as number
					n, err := atof(op)
					if err == nil {
						stack.push(n)
						continue
					}

					fmt.Printf(errorMsg("ERROR in word %q: unknown operation %q\n"), token, op)
					stack.restore()
					break
				}
				autoprint = true
				continue
			}

			// Help
			if token == "help" || token == "h" || token == "?" {
				if err := ops.help(); err != nil {
					fmt.Println(errorMsg(err))
				}
				continue
			}

			if token == "quit" || token == "exit" || token == "q" {
				fmt.Printf("Bye.\n")
				os.Exit(0)
			}

			// Check if it's a valid variable name (retrieve variable)
			if isValidVariableName(token) {
				if value, err := vars.get(token); err == nil {
					stack.push(value)
					autoprint = true
					continue
				}
				// If not found as a variable, fall through to number parsing
			}

			// At this point, it's either a number or not recognized.
			// If anything fails, restore stack and stop token processing.
			n, err := atof(token)
			if err != nil {
				fmt.Printf(errorMsg("Not a number or operator: %q.\n"), token)
				fmt.Println(errorMsg("Use \"help\" for online help."))
				stack.restore()
				break
			}
			// Valid number
			stack.push(n)
			continue
		}

		if autoprint {
			if single {
				fmt.Println(stack.top()) // plain print to stdout
			} else {
				stack.printTop(ctx, ops.base, ops.decimals) // pretty print to terminal
			}
		}

		// Break after the first iteration if a command is passed.
		if single {
			break
		}
	}
	return nil
}

func main() {
	stack := &stackType{}

	if err := calc(stack, strings.Join(os.Args[1:], " ")); err != nil {
		log.Fatal(err)
	}
}
