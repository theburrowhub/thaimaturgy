// Package imagefmt registers extra image decoders (TIFF, WebP, BMP) with the
// standard image registry so images extracted from PDFs — which pdfcpu often
// writes as TIFF — render in the GUI and editor. Import it for its side effects:
//
//	import _ "github.com/theburrowhub/thaimaturgy/internal/imagefmt"
//
// TIFF decoding uses github.com/hhrutter/tiff (a pdfcpu dependency) rather than
// golang.org/x/image/tiff: pdfcpu commonly extracts page scans as CMYK TIFF,
// which x/image/tiff cannot decode ("unsupported feature: color model") but
// hhrutter/tiff can. Do NOT also import x/image/tiff here — both register the
// same "tiff" magic, and whichever registers first wins the sniff; letting
// x/image/tiff win would fail on the CMYK images we most need to render.
package imagefmt

import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "github.com/hhrutter/tiff" // TIFF, incl. CMYK

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)
