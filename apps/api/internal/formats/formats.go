// Package formats converts between Hijau translations and common localization
// file formats via a small intermediate representation (a flat list of
// key/value Entries for one language). Each Format both marshals (export) and
// unmarshals (import).
package formats

import "sort"

// Entry is one translatable string. Key is dotted (e.g. "cart.checkout").
type Entry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Format marshals/unmarshals one language's entries.
type Format interface {
	ID() string
	Ext() string
	ContentType() string
	Marshal(entries []Entry) ([]byte, error)
	Unmarshal(data []byte) ([]Entry, error)
}

var registry = map[string]Format{}

func register(f Format) { registry[f.ID()] = f }

// Get returns the format adapter for id.
func Get(id string) (Format, bool) {
	f, ok := registry[id]
	return f, ok
}

// IDs lists the registered format ids, sorted.
func IDs() []string {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func init() {
	register(jsonFlat{})
	register(jsonNested{})
	register(csvFormat{})
	register(androidXML{})
	register(appleStrings{})
}

// sortedByKey returns entries ordered by key, for deterministic output.
func sortedByKey(e []Entry) []Entry {
	out := append([]Entry(nil), e...)
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
