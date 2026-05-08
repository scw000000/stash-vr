package static

import "embed"

//go:embed *.gohtml *.html *.png vendor/*.js
var Fs embed.FS
