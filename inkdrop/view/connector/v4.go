package viewBackend

import (
	"inkdrop/view/template"
	"io"
)

// V4Shell renders the InkDrop 4.0 four-panel layout.
func V4Shell(w io.Writer, p FrontEndParams) error {
	return template.V4Shell.Execute(w, p)
}