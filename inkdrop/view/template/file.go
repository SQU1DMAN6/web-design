package template

var (
	// Frontend
	LoginMain                 = Parse("themes/login/login.html")
	RegisterMain              = Parse("themes/register/register.html")
	RenderSuccessfulRegister  = ParseBackEndMessage("themes/register/successregister.html")
	IndexMain                 = Parse("themes/index/index.html")
	IndexMainBrowseRepository = Parse("themes/index/browse.html", "themes/index/template.html")
	LiveEditTextFile          = Parse("themes/live-edit/livetext.html")
	DocumentEditFile          = Parse("themes/doc-edit/document.html")

	// InkDrop 4.0
	V4Shell = Parse("themes/v4/shell.html")
)
