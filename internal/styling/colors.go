package styling

import (
	"fmt"
	"os"
)

const (
	Reset      = "\033[0m"
	Bold       = "\033[1m"
	Dim        = "\033[2m"
	Italic     = "\033[3m"
	Underline  = "\033[4m"
	Blink      = "\033[5m"
	Reverse    = "\033[7m"
	Hidden     = "\033[8m"
	Strikethru = "\033[9m"
)

const (
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

const (
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

const (
	BgBlack   = "\033[40m"
	BgRed     = "\033[41m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"
	BgWhite   = "\033[47m"
)

const (
	BgBrightBlack   = "\033[100m"
	BgBrightRed     = "\033[101m"
	BgBrightGreen   = "\033[102m"
	BgBrightYellow  = "\033[103m"
	BgBrightBlue    = "\033[104m"
	BgBrightMagenta = "\033[105m"
	BgBrightCyan    = "\033[106m"
	BgBrightWhite   = "\033[107m"
)

var (
	NoColor = false
)

func init() {
	NoColor = os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"
}

func Colorize(color, text string) string {
	if NoColor {
		return text
	}
	return color + text + Reset
}

func MakeBold(text string) string {
	return Colorize(Bold, text)
}

func MakeDim(text string) string {
	return Colorize(Dim, text)
}

func MakeItalic(text string) string {
	return Colorize(Italic, text)
}

func MakeUnderline(text string) string {
	return Colorize(Underline, text)
}

func Success(text string) string {
	return Colorize(BrightGreen, text)
}

func Error(text string) string {
	return Colorize(BrightRed, text)
}

func Warning(text string) string {
	return Colorize(BrightYellow, text)
}

func Info(text string) string {
	return Colorize(BrightBlue, text)
}

func Highlight(text string) string {
	return Colorize(BrightCyan, text)
}

func Muted(text string) string {
	return Colorize(Dim, text)
}

func Accent(text string) string {
	return Colorize(BrightMagenta, text)
}

func Package(text string) string {
	return Colorize(BrightGreen, text)
}

func Version(text string) string {
	return Colorize(BrightYellow, text)
}

func File(text string) string {
	return Colorize(BrightBlue, text)
}

func Size(text string) string {
	return Colorize(BrightCyan, text)
}

func Hash(text string) string {
	return Colorize(BrightMagenta, text)
}

func Separator() string {
	return Colorize(Dim, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func Header(text string) string {
	return Colorize(Bold+BrightCyan, text)
}

func SubHeader(text string) string {
	return Colorize(BrightBlue, text)
}

func Label(text string) string {
	return Colorize(Bold+White, text)
}

func Value(text string) string {
	return Colorize(BrightWhite, text)
}

func Command(text string) string {
	return Colorize(BrightGreen, text)
}

func URL(text string) string {
	return Colorize(Underline+BrightBlue, text)
}

func Status(text string, isSuccess bool) string {
	if isSuccess {
		return Colorize(BrightGreen, "✓ "+text)
	}
	return Colorize(BrightRed, "✗ "+text)
}

func Progress(current, total int) string {
	percentage := float64(current) / float64(total) * 100
	bar := "["
	for i := 0; i < 20; i++ {
		if float64(i)/20*100 <= percentage {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	return Colorize(BrightCyan, fmt.Sprintf("%s %.1f%%", bar, percentage))
}

func Hint(text string) string {
	return Colorize(Dim+Italic, fmt.Sprintf("💡 %s", text))
}
