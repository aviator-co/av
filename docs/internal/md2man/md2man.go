package md2man

import (
	"regexp"

	"github.com/russross/blackfriday/v2"
)

var returns = regexp.MustCompile(`\n+`)

func RenderToRoff(text []byte, section int, version, source, volume string) []byte {
	renderer := NewRoffRenderer(section, version, source, volume)
	bs := blackfriday.Run(text,
		[]blackfriday.Option{
			blackfriday.WithRenderer(renderer),
			blackfriday.WithExtensions(renderer.GetExtensions()),
		}...,
	)
	return []byte(returns.ReplaceAllString(string(bs), "\n"))
}
