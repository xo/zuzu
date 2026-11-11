package icon

// stroke opens the group of a built-in icon: lines of 1.5 with round ends,
// in the look of the outlines of zuzu.
const stroke = `<g fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">`

// builtins are the icons that zuzu draws with no pack: the five icons of
// the architecture diagram of Mermaid, in the look of zuzu (D31).
var builtins = map[string]string{
	"cloud":    `<path d="M7 18.5h10.5a4 4 0 0 0 .6-7.96A6 6 0 0 0 6.4 9.6A4.5 4.5 0 0 0 7 18.5z"/>`,
	"database": `<ellipse cx="12" cy="5.5" rx="7" ry="2.5"/><path d="M5 5.5v13c0 1.38 3.13 2.5 7 2.5s7-1.12 7-2.5v-13"/><path d="M5 12c0 1.38 3.13 2.5 7 2.5s7-1.12 7-2.5"/>`,
	"disk":     `<rect x="3.5" y="4" width="17" height="16" rx="2"/><circle cx="12" cy="11" r="4"/><path d="M12 11h.01M7 17h.01M17 17h-3"/>`,
	"internet": `<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3c2.4 2.5 3.6 5.5 3.6 9s-1.2 6.5-3.6 9c-2.4-2.5-3.6-5.5-3.6-9S9.6 5.5 12 3z"/>`,
	"server":   `<rect x="4" y="3.5" width="16" height="7.5" rx="2"/><rect x="4" y="13" width="16" height="7.5" rx="2"/><path d="M8 7.25h.01M8 16.75h.01M12 7.25h4M12 16.75h4"/>`,
}

// missing is the icon of a pack or an icon that does not exist: a box with
// a question mark, as in Mermaid.
const missing = `<rect x="3.5" y="3.5" width="17" height="17" rx="3"/><path d="M9.6 9.6a2.4 2.4 0 1 1 3.4 2.2c-.6.3-1 .9-1 1.5v.5M12 16.75h.01"/>`

// Builtin returns a built-in icon by its name, cloud, database, disk,
// internet or server.
func Builtin(name string) (*Icon, bool) {
	body, ok := builtins[name]
	if !ok {
		return nil, false
	}
	return &Icon{Body: stroke + body + "</g>", Width: 24, Height: 24}, true
}

// Missing returns the icon that draws in place of a missing icon.
func Missing() *Icon {
	return &Icon{Body: stroke + missing + "</g>", Width: 24, Height: 24}
}
