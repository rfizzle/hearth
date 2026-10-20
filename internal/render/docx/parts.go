package docx

import (
	"fmt"
	"strings"
)

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const (
	nsW   = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsWP  = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsA   = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsPic = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	relNS = "http://schemas.openxmlformats.org/package/2006/relationships"
)

const contentTypes = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Default Extension="png" ContentType="image/png"/>` +
	`<Default Extension="jpg" ContentType="image/jpeg"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>` +
	`</Types>`

const packageRels = xmlHeader + `<Relationships xmlns="` + relNS + `">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`</Relationships>`

// firstImage is the number of the first image's relationship id; rId1 is the
// styles and rId2 the numbering.
const firstImage = 3

// buildStyles is word/styles.xml: the built-in styles a document uses, named as
// Word names them so Word, Pages and Google Docs recognise and restyle them.

func buildStyles() string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<w:styles xmlns:w="` + nsW + `">`)
	b.WriteString(`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:eastAsia="Calibri" w:cs="Calibri"/>` +
		`<w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="en-US"/></w:rPr></w:rPrDefault>` +
		`<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="276" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>`)
	b.WriteString(`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>`)
	b.WriteString(`<w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/><w:uiPriority w:val="1"/><w:semiHidden/></w:style>`)
	b.WriteString(`<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/><w:uiPriority w:val="99"/><w:semiHidden/>` +
		`<w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>`)
	b.WriteString(`<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:keepNext/><w:spacing w:before="0" w:after="240" w:line="240" w:lineRule="auto"/></w:pPr><w:rPr><w:b/><w:sz w:val="52"/><w:szCs w:val="52"/></w:rPr></w:style>`)
	sizes := [6]int{40, 32, 28, 24, 22, 22}
	for i, sz := range sizes {
		fmt.Fprintf(&b, `<w:style w:type="paragraph" w:styleId="Heading%[1]d"><w:name w:val="heading %[1]d"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>`+
			`<w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="320" w:after="120" w:line="240" w:lineRule="auto"/><w:outlineLvl w:val="%[2]d"/></w:pPr>`+
			`<w:rPr><w:b/><w:sz w:val="%[3]d"/><w:szCs w:val="%[3]d"/></w:rPr></w:style>`, i+1, i, sz)
	}
	b.WriteString(`<w:style w:type="paragraph" w:styleId="ListBullet"><w:name w:val="List Bullet"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="60"/></w:pPr></w:style>`)
	b.WriteString(`<w:style w:type="paragraph" w:styleId="ListNumber"><w:name w:val="List Number"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:after="60"/></w:pPr></w:style>`)
	b.WriteString(`<w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:qFormat/>` +
		`<w:pPr><w:pBdr><w:left w:val="single" w:sz="12" w:space="10" w:color="BFBFBF"/></w:pBdr><w:ind w:left="720"/></w:pPr><w:rPr><w:i/><w:color w:val="595959"/></w:rPr></w:style>`)
	b.WriteString(`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/><w:basedOn w:val="Normal"/>` +
		`<w:pPr><w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr>` +
		`<w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:eastAsia="Courier New" w:cs="Courier New"/><w:sz w:val="19"/><w:szCs w:val="19"/></w:rPr></w:style>`)
	b.WriteString(`<w:style w:type="character" w:styleId="CodeChar"><w:name w:val="Code Char"/>` +
		`<w:rPr><w:rFonts w:ascii="Courier New" w:hAnsi="Courier New" w:eastAsia="Courier New" w:cs="Courier New"/><w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr></w:style>`)
	b.WriteString(`<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:basedOn w:val="TableNormal"/><w:uiPriority w:val="39"/>` +
		`<w:tblPr><w:tblBorders>`)
	for _, edge := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(&b, `<w:%s w:val="single" w:sz="4" w:space="0" w:color="BFBFBF"/>`, edge)
	}
	b.WriteString(`</w:tblBorders></w:tblPr></w:style></w:styles>`)
	return b.String()
}

// maxLevels is the list levels numbering.xml defines; a deeper list sits on
// the last.
const maxLevels = 9

// buildNumberingHead is word/numbering.xml up to the numbered lists' own entries:
// abstract list 0 is bullets and 1 is decimals, each over nine levels.

func buildNumberingHead() string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<w:numbering xmlns:w="` + nsW + `">`)
	for id, kind := range []struct{ format, text string }{{"bullet", "•"}, {"decimal", "%d."}} {
		fmt.Fprintf(&b, `<w:abstractNum w:abstractNumId="%d"><w:multiLevelType w:val="hybridMultilevel"/>`, id)
		for lvl := range maxLevels {
			text := kind.text
			if kind.format == "decimal" {
				text = fmt.Sprintf(text, lvl+1)
			}
			fmt.Fprintf(&b, `<w:lvl w:ilvl="%d"><w:start w:val="1"/><w:numFmt w:val="%s"/><w:lvlText w:val="%s"/><w:lvlJc w:val="left"/>`+
				`<w:pPr><w:ind w:left="%d" w:hanging="360"/></w:pPr></w:lvl>`, lvl, kind.format, text, 720*(lvl+1))
		}
		b.WriteString(`</w:abstractNum>`)
	}
	return b.String()
}
