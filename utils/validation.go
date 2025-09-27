package utils

import "errors"

func EnumValidator(allowed ...string) func(string) error {
	set := map[string]struct{}{}
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return func(s string) error {
		if _, ok := set[s]; ok {
			return nil
		}
		return errors.New("validation failed")
	}
}
