package app

import (
	"fmt"
	"strings"
)

type addOptions struct {
	Path                string
	NewBranch           string
	NewBranchProvided   bool
	Detach              bool
	From                string
	FromProvided        bool
	Description         string
	DescriptionProvided bool
	Protected           bool
}

type metaOptions struct {
	Path                string
	Description         string
	DescriptionProvided bool
	Protected           bool
	ProtectedProvided   bool
}

type removeOptions struct {
	Path  string
	Force bool
}

func parseAdd(args []string) (addOptions, error) {
	var options addOptions
	positionals := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-b":
			if options.NewBranchProvided {
				return options, fmt.Errorf("-b may be provided only once")
			}
			value, next, err := optionValue(args, i, "-b")
			if err != nil {
				return options, err
			}
			options.NewBranch, options.NewBranchProvided, i = value, true, next
		case "--detach":
			if options.Detach {
				return options, fmt.Errorf("--detach may be provided only once")
			}
			options.Detach = true
		case "--from":
			if options.FromProvided {
				return options, fmt.Errorf("--from may be provided only once")
			}
			value, next, err := optionValue(args, i, "--from")
			if err != nil {
				return options, err
			}
			options.From, options.FromProvided, i = value, true, next
		case "--description":
			if options.DescriptionProvided {
				return options, fmt.Errorf("--description may be provided only once")
			}
			value, next, err := optionValue(args, i, "--description")
			if err != nil {
				return options, err
			}
			options.Description, options.DescriptionProvided, i = value, true, next
		case "--protected":
			if options.Protected {
				return options, fmt.Errorf("--protected may be provided only once")
			}
			options.Protected = true
		default:
			if strings.HasPrefix(arg, "-") {
				return options, fmt.Errorf("unknown add option %q", arg)
			}
			positionals++
			if positionals > 1 {
				return options, fmt.Errorf("add accepts exactly one path")
			}
			options.Path = arg
		}
	}
	if positionals != 1 {
		return options, fmt.Errorf("add requires one path")
	}
	if options.NewBranchProvided && options.Detach {
		return options, fmt.Errorf("-b and --detach are mutually exclusive")
	}
	if err := validatePath(options.Path); err != nil {
		return options, err
	}
	return options, nil
}

func parseMeta(args []string) (metaOptions, error) {
	var options metaOptions
	positionals := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--description":
			if options.DescriptionProvided {
				return options, fmt.Errorf("--description may be provided only once")
			}
			value, next, err := optionValue(args, i, "--description")
			if err != nil {
				return options, err
			}
			options.Description, options.DescriptionProvided, i = value, true, next
		case "--protected":
			if options.ProtectedProvided {
				return options, fmt.Errorf("--protected may be provided only once")
			}
			value, next, err := optionValue(args, i, "--protected")
			if err != nil {
				return options, err
			}
			switch value {
			case "true":
				options.Protected = true
			case "false":
				options.Protected = false
			default:
				return options, fmt.Errorf("--protected must be true or false")
			}
			options.ProtectedProvided, i = true, next
		default:
			if strings.HasPrefix(arg, "-") {
				return options, fmt.Errorf("unknown meta option %q", arg)
			}
			positionals++
			if positionals > 1 {
				return options, fmt.Errorf("meta accepts exactly one path")
			}
			options.Path = arg
		}
	}
	if positionals != 1 {
		return options, fmt.Errorf("meta requires one path")
	}
	if err := validatePath(options.Path); err != nil {
		return options, err
	}
	return options, nil
}

func parseRemove(args []string) (removeOptions, error) {
	var options removeOptions
	positionals := 0
	for _, arg := range args {
		switch arg {
		case "--force":
			if options.Force {
				return options, fmt.Errorf("--force may be provided only once")
			}
			options.Force = true
		default:
			if strings.HasPrefix(arg, "-") {
				return options, fmt.Errorf("unknown remove option %q", arg)
			}
			positionals++
			if positionals > 1 {
				return options, fmt.Errorf("remove accepts exactly one path")
			}
			options.Path = arg
		}
	}
	if positionals != 1 {
		return options, fmt.Errorf("remove requires one path")
	}
	if err := validatePath(options.Path); err != nil {
		return options, err
	}
	return options, nil
}

func optionValue(args []string, index int, name string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	value := args[index+1]
	if !validText(value) {
		return "", index, fmt.Errorf("%s value must be valid UTF-8 without NUL", name)
	}
	return value, index + 1, nil
}

func validatePath(path string) error {
	if path == "" || !validText(path) {
		return fmt.Errorf("path must be non-empty valid UTF-8 without NUL")
	}
	return nil
}
