package color

import "fmt"

func color(s string, code int) string {
	return fmt.Sprintf("\033[0;%dm%s\033[0m", code, s)
}

// Red change output color to red.
func Red(s string) string {
	return color(s, 31)
}

// Green change output color to green.
func Green(s string) string {
	return color(s, 32)
}

// Yellow change output color to yellow.
func Yellow(s string) string {
	return color(s, 33)
}
