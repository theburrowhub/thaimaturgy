// Package bookpdf lays out a book-style Markdown document as a print-ready PDF
// (A5, serif body, title page, chapters on their own page, page numbers). It is
// shared by the session novelization and the DM sourcebook exporters.
package bookpdf

import (
	"bytes"
	"strings"

	"github.com/go-pdf/fpdf"
)

// FromMarkdown renders a book-like PDF from a Markdown subset: "# " book title,
// "## " chapters (new page), "### "/"#### " subheadings, "- "/"* " bullet lists,
// "> " block quotes, and blank-line-separated paragraphs. Inline * _ ` markers are
// stripped for clean printed prose.
func FromMarkdown(title, subtitle, md string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A5", "")
	pdf.SetMargins(18, 20, 18)
	pdf.SetAutoPageBreak(true, 20)

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

	bodyOpen := false
	ensureBody := func() {
		if !bodyOpen {
			pdf.AddPage()
			bodyOpen = true
		}
	}

	para := &strings.Builder{}
	flushPara := func() {
		text := strings.TrimSpace(para.String())
		para.Reset()
		if text == "" {
			return
		}
		ensureBody()
		pdf.SetFont("Times", "", 12)
		pdf.MultiCell(0, 6.2, tr(pdf, stripInline(text)), "", "J", false)
		pdf.Ln(2.5)
	}

	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "# "):
			flushPara() // book title is on the title page; ignore repeats
		case strings.HasPrefix(line, "## "):
			flushPara()
			pdf.AddPage()
			bodyOpen = true
			pdf.Ln(8)
			pdf.SetFont("Times", "B", 18)
			pdf.MultiCell(0, 10, tr(pdf, stripInline(line[3:])), "", "C", false)
			pdf.Ln(5)
		case strings.HasPrefix(line, "### "):
			flushPara()
			ensureBody()
			pdf.Ln(1)
			pdf.SetFont("Times", "B", 14)
			pdf.MultiCell(0, 8, tr(pdf, stripInline(line[4:])), "", "L", false)
			pdf.Ln(1)
		case strings.HasPrefix(line, "#### "):
			flushPara()
			ensureBody()
			pdf.SetFont("Times", "B", 12)
			pdf.MultiCell(0, 7, tr(pdf, stripInline(line[5:])), "", "L", false)
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			flushPara()
			ensureBody()
			pdf.SetFont("Times", "", 12)
			// Hanging bullet via a left indent.
			left, _, _, _ := pdf.GetMargins()
			pdf.SetLeftMargin(left + 5)
			pdf.MultiCell(0, 6, tr(pdf, "• "+stripInline(strings.TrimSpace(trimmed[2:]))), "", "L", false)
			pdf.SetLeftMargin(left)
		case strings.HasPrefix(line, "> "):
			flushPara()
			ensureBody()
			pdf.SetFont("Times", "I", 12)
			left, _, _, _ := pdf.GetMargins()
			pdf.SetLeftMargin(left + 5)
			pdf.MultiCell(0, 6.2, tr(pdf, stripInline(line[2:])), "", "L", false)
			pdf.SetLeftMargin(left)
			pdf.Ln(1)
		case trimmed == "":
			flushPara()
		default:
			if para.Len() > 0 {
				para.WriteByte(' ')
			}
			para.WriteString(trimmed)
		}
	}
	flushPara()

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstLineTitle(md, fallback string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# ") {
			return stripInline(strings.TrimSpace(line[2:]))
		}
	}
	return fallback
}

func stripInline(s string) string {
	return strings.NewReplacer("**", "", "__", "", "*", "", "`", "").Replace(s)
}

// tr converts UTF-8 to the cp1252 encoding fpdf's core fonts expect, so accented
// characters render correctly.
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
