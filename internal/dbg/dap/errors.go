package dap

type gniDAPError int

const (
	processingErr gniDAPError = iota
	parseErr
	launchErr
	setBreakpointsErr
)

func (e gniDAPError) String() string {
	return []string{"Processing error", "Parse error", "Failed to launch", "Failed to set breakpoints"}[e]
}
