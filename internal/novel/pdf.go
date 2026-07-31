package novel

import (
	"bytes"
	"strings"

	"github.com/go-pdf/fpdf"
)

// MarkdownToPDF lays out the novelization Markdown as a book-like PDF (A5, serif
// body, a title page, chapters starting on their own page, and page numbers). It
// understands a small Markdown subset: "# " book title, "## " chapters, "### "
// subheadings, and blank-line-separated paragraphs; inline * _ ` markers are
// stripped for clean prose.
func MarkdownToPDF(title, subtitle, md string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A5", "")
	pdf.SetMargins(18, 20, 18)
	pdf.SetAutoPageBreak(true, 20)

	// Page numbers (skip the title page).
	pdf.SetFooterFunc(func() {
		if pdf.PageNo() <= 1 {
			return
		}
		pdf.SetY(-15)
		pdf.SetFont("Times", "I", 9)
		pdf.CellFormat(0, 10, itoa(pdf.PageNo()-1), "", 0, "C", false, 0, "")
	})

	// Title page.
	pdf.AddPage()
	pdf.Ln(40)
	pdf.SetFont("Times", "B", 26)
	pdf.MultiCell(0, 12, tr(pdf, firstLineTitle(md, title)), "", "C", false)
	if subtitle != "" {
		pdf.Ln(6)
		pdf.SetFont("Times", "I", 13)
		pdf.MultiCell(0, 8, tr(pdf, subtitle), "", "C", false)
	}

	bodyPageOpen := false
	ensureBody := func() {
		if !bodyPageOpen {
			pdf.AddPage()
			bodyPageOpen = true
		}
	}

	para := &strings.Builder{}
	flush := func() {
		text := strings.TrimSpace(para.String())
		para.Reset()
		if text == "" {
			return
		}
		ensureBody()
		pdf.SetFont("Times", "", 12)
		pdf.MultiCell(0, 6.2, tr(pdf, stripInline(text)), "", "J", false)
		pdf.Ln(3)
	}

	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimRight(raw, " \t")
		switch {
		case strings.HasPrefix(line, "# "):
			// Book title already on the title page; ignore further top-level titles.
			flush()
		case strings.HasPrefix(line, "## "):
			flush()
			pdf.AddPage()
			bodyPageOpen = true
			pdf.Ln(10)
			pdf.SetFont("Times", "B", 18)
			pdf.MultiCell(0, 10, tr(pdf, stripInline(line[3:])), "", "C", false)
			pdf.Ln(6)
		case strings.HasPrefix(line, "### "):
			flush()
			ensureBody()
			pdf.SetFont("Times", "B", 13)
			pdf.MultiCell(0, 8, tr(pdf, stripInline(line[4:])), "", "L", false)
			pdf.Ln(2)
		case strings.TrimSpace(line) == "":
			flush()
		default:
			if para.Len() > 0 {
				para.WriteByte(' ')
			}
			para.WriteString(strings.TrimSpace(line))
		}
	}
	flush()

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// firstLineTitle returns the "# " title from the markdown, or the fallback.
func firstLineTitle(md, fallback string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# ") {
			return stripInline(strings.TrimSpace(line[2:]))
		}
	}
	return fallback
}

// stripInline removes basic Markdown emphasis markers for clean printed prose.
func stripInline(s string) string {
	r := strings.NewReplacer("**", "", "__", "", "*", "", "`", "")
	return r.Replace(s)
}

// tr converts a UTF-8 string to the encoding fpdf's core fonts expect (cp1252),
// so accented characters (á, ñ, …) render correctly.
func tr(pdf *fpdf.Fpdf, s string) string {
	return pdf.UnicodeTranslatorFromDescriptor("")(s)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
