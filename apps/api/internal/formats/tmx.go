package formats

// TMX (Translation Memory eXchange) 1.4 is a BILINGUAL interchange format, so it
// does not fit the monolingual key/value Format interface — it's a standalone
// codec over translation-memory segments (source/target text + BCP-47 langs).

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// TMXUnit is one bilingual translation-memory segment (a source<->target pair).
type TMXUnit struct {
	SourceLang string
	SourceText string
	TargetLang string
	TargetText string
	// Flattened is true when either side's <seg> contained inline formatting
	// tags (bpt/ept/ph/it/hi) that were reduced to plain text, so the caller can
	// warn that placeholders/formatting may have been lost.
	Flattened bool
}

type tmxRoot struct {
	XMLName xml.Name  `xml:"tmx"`
	Version string    `xml:"version,attr"`
	Header  tmxHeader `xml:"header"`
	Body    []tmxTU   `xml:"body>tu"`
}

type tmxHeader struct {
	SrcLang             string `xml:"srclang,attr"`
	Datatype            string `xml:"datatype,attr"`
	SegType             string `xml:"segtype,attr"`
	CreationTool        string `xml:"creationtool,attr"`
	CreationToolVersion string `xml:"creationtoolversion,attr"`
	AdminLang           string `xml:"adminlang,attr"`
	OTmf                string `xml:"o-tmf,attr"`
}

type tmxTU struct {
	Variants []tmxTUV `xml:"tuv"`
}

type tmxTUV struct {
	// xml:lang is namespaced; the full-URI tag makes encoding/xml emit "xml:lang"
	// on marshal and match it on unmarshal. LangBare also matches a bare "lang"
	// attribute that some tools emit; lang() prefers the namespaced one.
	Lang     string `xml:"http://www.w3.org/XML/1998/namespace lang,attr"`
	LangBare string `xml:"lang,attr,omitempty"`
	Seg      tmxSeg `xml:"seg"`
}

// tmxSeg captures the segment's flattened text (Text, the direct character data)
// and its raw inner XML (Inner) so we can detect — and warn about — inline
// formatting tags that the plain-text TM can't represent.
type tmxSeg struct {
	Text  string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

func (v tmxTUV) lang() string {
	if v.Lang != "" {
		return strings.TrimSpace(v.Lang)
	}
	return strings.TrimSpace(v.LangBare)
}

// text is the segment value: the direct chardata, trimmed so stored text matches
// the (trim-normalized) source hash and dedups against human-recorded segments.
func (v tmxTUV) text() string { return strings.TrimSpace(v.Seg.Text) }

// hasInline reports whether the <seg> wrapped its text in inline elements
// (bpt/ept/ph/...), whose contents are dropped by text().
func (v tmxTUV) hasInline() bool { return strings.Contains(v.Seg.Inner, "<") }

// MarshalTMX renders segments as a TMX 1.4 document. srcLang sets the header
// srclang (use "*all*" — the TMX sentinel — when segments mix source languages).
func MarshalTMX(srcLang string, units []TMXUnit) ([]byte, error) {
	if srcLang == "" {
		srcLang = "*all*"
	}
	root := tmxRoot{
		Version: "1.4",
		Header: tmxHeader{
			SrcLang: srcLang, Datatype: "plaintext", SegType: "sentence",
			CreationTool: "hijau", CreationToolVersion: "0.1.0", AdminLang: "en", OTmf: "hijau",
		},
	}
	for _, u := range units {
		if strings.TrimSpace(u.SourceText) == "" || strings.TrimSpace(u.TargetText) == "" {
			continue // never emit a semantically-empty TU
		}
		root.Body = append(root.Body, tmxTU{Variants: []tmxTUV{
			{Lang: u.SourceLang, Seg: tmxSeg{Text: u.SourceText}},
			{Lang: u.TargetLang, Seg: tmxSeg{Text: u.TargetText}},
		}})
	}
	b, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(b, '\n')...), nil
}

// UnmarshalTMX parses a TMX document into bilingual units. Multilingual TUs are
// expanded into one unit per (source, target) pair: the source is the tuv whose
// xml:lang matches the header srclang, else (when no srclang is declared) the
// first tuv. skipped counts TUs dropped because they had fewer than two variants
// or — when a srclang is declared — no variant in that source language.
func UnmarshalTMX(data []byte) (units []TMXUnit, skipped int, err error) {
	var root tmxRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, 0, fmt.Errorf("invalid TMX: %w", err)
	}
	srcLang := strings.TrimSpace(root.Header.SrcLang)
	srcDeclared := srcLang != "" && srcLang != "*all*"
	for _, tu := range root.Body {
		if len(tu.Variants) < 2 {
			skipped++
			continue // need at least a source + a target
		}
		srcIdx := 0 // convention: the first tuv is the source
		if srcDeclared {
			srcIdx = -1
			for i, v := range tu.Variants {
				if strings.EqualFold(v.lang(), srcLang) {
					srcIdx = i
					break
				}
			}
			if srcIdx == -1 {
				skipped++
				continue // declared source language absent — don't mislabel another tuv
			}
		}
		src := tu.Variants[srcIdx]
		for i, v := range tu.Variants {
			if i == srcIdx {
				continue
			}
			units = append(units, TMXUnit{
				SourceLang: src.lang(), SourceText: src.text(),
				TargetLang: v.lang(), TargetText: v.text(),
				Flattened: src.hasInline() || v.hasInline(),
			})
		}
	}
	return units, skipped, nil
}
