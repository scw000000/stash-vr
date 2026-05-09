package static

import "embed"

//go:embed *.gohtml *.html *.png *.js vendor/*.js
var Fs embed.FS
