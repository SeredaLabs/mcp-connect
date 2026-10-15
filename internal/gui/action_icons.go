package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// A single outline style for connection actions, with theme-aware contrast.
func connectionActionIcon(name, paths string) fyne.Resource {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000000" d="` + paths + `"/></svg>`
	return theme.NewThemedResource(fyne.NewStaticResource(name+".svg", []byte(svg)))
}

var (
	testConnectionIcon   = connectionActionIcon("connection-test", "M2 11h4.3l2.8-6.4a1 1 0 0 1 1.9.1l3.2 11.2 1.9-4.3a1 1 0 0 1 .9-.6h5v2h-4.3l-2.8 6.4a1 1 0 0 1-1.9-.1L9.8 8.1l-1.9 4.3a1 1 0 0 1-.9.6H2z")
	connectIcon          = connectionActionIcon("connection-link", "M9 6H7a6 6 0 0 0 0 12h2v-2H7a4 4 0 0 1 0-8h2z M15 6h2a6 6 0 0 1 0 12h-2v-2h2a4 4 0 0 0 0-8h-2z M8 11h8v2H8z")
	disconnectIcon       = connectionActionIcon("connection-unlink", "M7 6H6a6 6 0 0 0 0 12h1v-2H6a4 4 0 0 1 0-8h1z M17 6h1a6 6 0 0 1 0 12h-1v-2h1a4 4 0 0 0 0-8h-1z M13 7l2 1-4 9-2-1z")
	deleteConnectionIcon = connectionActionIcon("connection-delete", "M9 2h6v3h5v2H4V5h5z M11 4v1h2V4z M5 8h2l1 12h8l1-12h2l-1 14H6z M9 9h2v9H9z M13 9h2v9h-2z")
)

var (
	loginPasswordIcon = connectionActionIcon("auth-login-password", "M8 2a4 4 0 1 1 0 8 4 4 0 1 1 0-8 M1 21v-3a7 7 0 0 1 12-4.9v2.5A5 5 0 0 0 3 18v1h8v2z M15 12v-2a3 3 0 0 1 6 0v2h-2v-2a1 1 0 0 0-2 0v2z M13 13h10v9H13z")
	bearerTokenIcon   = connectionActionIcon("auth-bearer-token", "M12 2l9 5v10l-9 5-9-5V7z M12 4.3L5 8.2v7.6l7 3.9 7-3.9V8.2z M12 8a4 4 0 1 1 0 8 4 4 0 1 1 0-8")
	apiKeyIcon        = connectionActionIcon("auth-api-key", "M7 7a5 5 0 1 1 0 10 5 5 0 1 1 0-10 M7 9a3 3 0 1 0 0 6 3 3 0 1 0 0-6 M11 11h12v2h-2v4h-2v-4h-3v3h-2v-3h-3z")
)
