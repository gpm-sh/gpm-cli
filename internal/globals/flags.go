package globals

var (
	Verbose    bool
	Debug      bool
	Quiet      bool
	JSONOutput bool
)

func SetFlags(verbose, debug, quiet, json bool) {
	Verbose = verbose
	Debug = debug
	Quiet = quiet
	JSONOutput = json
}

func IsVerbose() bool {
	return Verbose
}

func IsDebug() bool {
	return Debug
}

func IsQuiet() bool {
	return Quiet
}

func IsJSONOutput() bool {
	return JSONOutput
}
