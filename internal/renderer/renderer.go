package renderer

import (
	"fmt"
	"os"
	"os/exec"
)

// ImageRenderer renders an image to a string of terminal escape sequences.
type ImageRenderer interface {
	Render(imagePath string, width, height int) (string, error)
}

// detectFormat picks the best chafa --format value based on environment variables.
// chafa's --format=auto only inspects $TERM, missing terminals like WezTerm that
// advertise themselves via $TERM_PROGRAM instead.
func detectFormat() string {
	switch os.Getenv("TERM_PROGRAM") {
	case "WezTerm":
		return "kitty"
	case "iTerm.app":
		return "iterm"
	}
	switch os.Getenv("TERM") {
	case "xterm-kitty":
		return "kitty"
	}
	return "auto"
}

// ChafaRenderer renders images using the chafa CLI tool.
type ChafaRenderer struct{}

func (r *ChafaRenderer) Render(imagePath string, width, height int) (string, error) {
	format := detectFormat()
	cmd := exec.Command(
		"chafa",
		"--format="+format,
		"--size", fmt.Sprintf("%dx%d", width, height),
		"--stretch",
		imagePath,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("chafa: %w", err)
	}

	return string(out), nil
}

// IsChafaAvailable checks whether chafa is on PATH.
func IsChafaAvailable() bool {
	_, err := exec.LookPath("chafa")
	return err == nil
}

// FallbackRenderer renders a simple placeholder when chafa is unavailable.
type FallbackRenderer struct{}

func (r *FallbackRenderer) Render(imagePath string, width, height int) (string, error) {
	line := "+" + repeatStr("-", width-2) + "+"
	mid := "|" + centerStr("NO PREVIEW", width-2) + "|"

	out := line + "\n"
	for i := 0; i < height-2; i++ {
		if i == (height-2)/2 {
			out += mid + "\n"
		} else {
			out += "|" + repeatStr(" ", width-2) + "|\n"
		}
	}
	out += line
	return out, nil
}

func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

func centerStr(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	pad := (width - len(s)) / 2
	return repeatStr(" ", pad) + s + repeatStr(" ", width-len(s)-pad)
}

// ensure FallbackRenderer satisfies the interface
var _ ImageRenderer = (*FallbackRenderer)(nil)
var _ ImageRenderer = (*ChafaRenderer)(nil)

