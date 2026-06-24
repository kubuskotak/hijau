package formats

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// --- JSON (flat: {"a.b": "v"}) ---

type jsonFlat struct{}

func (jsonFlat) ID() string          { return "json" }
func (jsonFlat) Ext() string         { return "json" }
func (jsonFlat) ContentType() string { return "application/json" }

func (jsonFlat) Marshal(entries []Entry) ([]byte, error) {
	m := map[string]string{}
	for _, e := range entries {
		m[e.Key] = e.Value
	}
	b, err := json.MarshalIndent(m, "", "  ") // json sorts map keys
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func (jsonFlat) Unmarshal(data []byte) ([]Entry, error) {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid flat JSON: %w", err)
	}
	out := make([]Entry, 0, len(m))
	for k, v := range m {
		out = append(out, Entry{Key: k, Value: v})
	}
	return sortedByKey(out), nil
}

// --- JSON (nested / i18next: {"a": {"b": "v"}}) ---

type jsonNested struct{}

func (jsonNested) ID() string          { return "json-nested" }
func (jsonNested) Ext() string         { return "json" }
func (jsonNested) ContentType() string { return "application/json" }

func (jsonNested) Marshal(entries []Entry) ([]byte, error) {
	root := map[string]any{}
	for _, e := range sortedByKey(entries) {
		parts := strings.Split(e.Key, ".")
		cur := root
		for i, p := range parts {
			if i == len(parts)-1 {
				cur[p] = e.Value
				break
			}
			next, ok := cur[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				cur[p] = next
			}
			cur = next
		}
	}
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func (jsonNested) Unmarshal(data []byte) ([]Entry, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid nested JSON: %w", err)
	}
	var out []Entry
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, v := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			switch t := v.(type) {
			case string:
				out = append(out, Entry{Key: key, Value: t})
			case map[string]any:
				walk(key, t)
			default:
				out = append(out, Entry{Key: key, Value: fmt.Sprintf("%v", t)})
			}
		}
	}
	walk("", root)
	return sortedByKey(out), nil
}

// --- CSV (key,value with header) ---

type csvFormat struct{}

func (csvFormat) ID() string          { return "csv" }
func (csvFormat) Ext() string         { return "csv" }
func (csvFormat) ContentType() string { return "text/csv" }

func (csvFormat) Marshal(entries []Entry) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"key", "value"})
	for _, e := range sortedByKey(entries) {
		if err := w.Write([]string{e.Key, e.Value}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (csvFormat) Unmarshal(data []byte) ([]Entry, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}
	out := make([]Entry, 0, len(rows))
	for i, row := range rows {
		if len(row) < 2 {
			continue
		}
		if i == 0 && strings.EqualFold(strings.TrimSpace(row[0]), "key") {
			continue // header
		}
		out = append(out, Entry{Key: row[0], Value: row[1]})
	}
	return sortedByKey(out), nil
}

// --- Android strings.xml ---

type androidXML struct{}

func (androidXML) ID() string          { return "android" }
func (androidXML) Ext() string         { return "xml" }
func (androidXML) ContentType() string { return "application/xml" }

type androidResources struct {
	XMLName xml.Name        `xml:"resources"`
	Strings []androidString `xml:"string"`
}

type androidString struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

func (androidXML) Marshal(entries []Entry) ([]byte, error) {
	res := androidResources{}
	for _, e := range sortedByKey(entries) {
		res.Strings = append(res.Strings, androidString{Name: e.Key, Value: e.Value})
	}
	b, err := xml.MarshalIndent(res, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(b, '\n')...), nil
}

func (androidXML) Unmarshal(data []byte) ([]Entry, error) {
	var res androidResources
	if err := xml.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("invalid Android XML: %w", err)
	}
	out := make([]Entry, 0, len(res.Strings))
	for _, s := range res.Strings {
		out = append(out, Entry{Key: s.Name, Value: s.Value})
	}
	return sortedByKey(out), nil
}

// --- Apple .strings ("key" = "value";) ---

type appleStrings struct{}

func (appleStrings) ID() string          { return "apple" }
func (appleStrings) Ext() string         { return "strings" }
func (appleStrings) ContentType() string { return "text/plain" }

func (appleStrings) Marshal(entries []Entry) ([]byte, error) {
	var buf bytes.Buffer
	for _, e := range sortedByKey(entries) {
		fmt.Fprintf(&buf, "%q = %q;\n", e.Key, e.Value)
	}
	return buf.Bytes(), nil
}

var appleLine = regexp.MustCompile(`("(?:[^"\\]|\\.)*")\s*=\s*("(?:[^"\\]|\\.)*")\s*;`)

func (appleStrings) Unmarshal(data []byte) ([]Entry, error) {
	var out []Entry
	for _, m := range appleLine.FindAllStringSubmatch(string(data), -1) {
		k, err := strconv.Unquote(m[1])
		if err != nil {
			continue
		}
		v, err := strconv.Unquote(m[2])
		if err != nil {
			continue
		}
		out = append(out, Entry{Key: k, Value: v})
	}
	return sortedByKey(out), nil
}

// --- XLIFF 1.2 ---
// Single-language export: the value goes in <target>; <source> carries the key
// (we don't track per-entry source text). On import the target wins, falling
// back to source.

type xliffFormat struct{}

func (xliffFormat) ID() string          { return "xliff" }
func (xliffFormat) Ext() string         { return "xlf" }
func (xliffFormat) ContentType() string { return "application/xml" }

type xliffRoot struct {
	XMLName xml.Name  `xml:"xliff"`
	Version string    `xml:"version,attr"`
	File    xliffFile `xml:"file"`
}
type xliffFile struct {
	Datatype string      `xml:"datatype,attr"`
	Original string      `xml:"original,attr"`
	Body     []xliffUnit `xml:"body>trans-unit"`
}
type xliffUnit struct {
	ID     string `xml:"id,attr"`
	Source string `xml:"source"`
	Target string `xml:"target"`
}

func (xliffFormat) Marshal(entries []Entry) ([]byte, error) {
	root := xliffRoot{Version: "1.2", File: xliffFile{Datatype: "plaintext", Original: "messages"}}
	for _, e := range sortedByKey(entries) {
		root.File.Body = append(root.File.Body, xliffUnit{ID: e.Key, Source: e.Key, Target: e.Value})
	}
	b, err := xml.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(b, '\n')...), nil
}

func (xliffFormat) Unmarshal(data []byte) ([]Entry, error) {
	var root xliffRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid XLIFF: %w", err)
	}
	out := make([]Entry, 0, len(root.File.Body))
	for _, u := range root.File.Body {
		v := u.Target
		if v == "" {
			v = u.Source
		}
		out = append(out, Entry{Key: u.ID, Value: v})
	}
	return sortedByKey(out), nil
}

// --- PO / gettext ---

type poFormat struct{}

func (poFormat) ID() string          { return "po" }
func (poFormat) Ext() string         { return "po" }
func (poFormat) ContentType() string { return "text/plain" }

func (poFormat) Marshal(entries []Entry) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("msgid \"\"\nmsgstr \"\"\n\n") // minimal header
	for _, e := range sortedByKey(entries) {
		fmt.Fprintf(&buf, "msgid %q\nmsgstr %q\n\n", e.Key, e.Value)
	}
	return buf.Bytes(), nil
}

func (poFormat) Unmarshal(data []byte) ([]Entry, error) {
	var out []Entry
	var key string
	var haveKey bool
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "msgid "):
			if s, err := strconv.Unquote(strings.TrimSpace(line[len("msgid "):])); err == nil {
				key, haveKey = s, true
			}
		case strings.HasPrefix(line, "msgstr "):
			if !haveKey {
				continue
			}
			if s, err := strconv.Unquote(strings.TrimSpace(line[len("msgstr "):])); err == nil && key != "" {
				out = append(out, Entry{Key: key, Value: s}) // empty msgid = header, skipped
			}
			key, haveKey = "", false
		}
	}
	return sortedByKey(out), nil
}
