package dap

type gniDAPError int

const (
	processingErr gniDAPError = iota
	parseErr
	launchErr
)

func (e gniDAPError) String() string {
	return []string{"Processing error", "Parse error", "Failed to launch"}[e]
}
