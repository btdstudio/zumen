package icm

import "strings"

// Check validates the embedded scaffold and returns a list of problems
// (empty when clean). It guards against template syntax leaking into the
// literal scaffold output.
func Check() []string {
	var problems []string
	_ = walkScaffold(func(rel string, data []byte) error {
		if strings.Contains(string(data), "{{") {
			problems = append(problems, rel+": contains template syntax")
		}
		return nil
	})
	return problems
}
