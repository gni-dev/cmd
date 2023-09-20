package dap

type gniDAPError int

const (
	processingErr gniDAPError = iota
)

func (e gniDAPError) String() string {
	return []string{"Processing error"}[e]
}
