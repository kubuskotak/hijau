package formats

import (
	"strings"
	"testing"
)

func TestTMXRoundTrip(t *testing.T) {
	units := []TMXUnit{
		{SourceLang: "en", SourceText: "Hello", TargetLang: "fr", TargetText: "Bonjour"},
		{SourceLang: "en", SourceText: `Quote " amp & lt < gt > newline
end`, TargetLang: "fr", TargetText: "Spécial éàü"},
		{SourceLang: "en", SourceText: "You have {count} messages", TargetLang: "fr", TargetText: "Vous avez {count} messages"},
	}

	out, err := MarshalTMX("en", units)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `xml:lang="en"`) || !strings.Contains(s, `xml:lang="fr"`) {
		t.Fatalf("expected xml:lang attributes in output, got:\n%s", s)
	}
	if !strings.Contains(s, `<tmx version="1.4">`) {
		t.Fatalf("expected tmx 1.4 root, got:\n%s", s)
	}

	got, skipped, err := UnmarshalTMX(out)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("expected 0 skipped, got %d", skipped)
	}
	if len(got) != len(units) {
		t.Fatalf("round-trip count: got %d, want %d", len(got), len(units))
	}
	for i, u := range units {
		if got[i] != u {
			t.Errorf("unit %d round-trip mismatch:\n got  %+v\n want %+v", i, got[i], u)
		}
	}
}

// A bare `lang` attribute (no xml: namespace) must still parse.
func TestTMXUnmarshalBareLang(t *testing.T) {
	in := `<?xml version="1.0"?>
<tmx version="1.4"><header srclang="en"/><body>
<tu><tuv lang="en"><seg>Cat</seg></tuv><tuv lang="de"><seg>Katze</seg></tuv></tu>
</body></tmx>`
	got, _, err := UnmarshalTMX([]byte(in))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].SourceLang != "en" || got[0].TargetLang != "de" || got[0].TargetText != "Katze" {
		t.Fatalf("bare-lang parse wrong: %+v", got)
	}
}

// A multilingual TU (>2 tuvs) expands into one unit per non-source target.
func TestTMXUnmarshalMultilingual(t *testing.T) {
	in := `<?xml version="1.0"?>
<tmx version="1.4"><header srclang="en"/><body>
<tu>
  <tuv xml:lang="en"><seg>Dog</seg></tuv>
  <tuv xml:lang="fr"><seg>Chien</seg></tuv>
  <tuv xml:lang="es"><seg>Perro</seg></tuv>
</tu></body></tmx>`
	got, _, err := UnmarshalTMX([]byte(in))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 pairs from a 3-lang TU, got %d: %+v", len(got), got)
	}
	for _, u := range got {
		if u.SourceLang != "en" || u.SourceText != "Dog" {
			t.Errorf("source should be en/Dog, got %+v", u)
		}
	}
}

// Inline formatting tags are flattened to text and the unit is flagged; leading/
// trailing whitespace is trimmed so it matches the source hash + dedups cleanly.
func TestTMXInlineTagsAndWhitespace(t *testing.T) {
	in := `<?xml version="1.0"?>
<tmx version="1.4"><header srclang="en"/><body>
<tu><tuv xml:lang="en"><seg>Click <bpt i="1">&lt;b&gt;</bpt>here<ept i="1">&lt;/b&gt;</ept></seg></tuv><tuv xml:lang="fr"><seg>  Cliquez ici  </seg></tuv></tu>
</body></tmx>`
	got, _, err := UnmarshalTMX([]byte(in))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(got))
	}
	u := got[0]
	if !u.Flattened {
		t.Errorf("expected Flattened=true for a segment with inline tags")
	}
	if u.SourceText != "Click here" { // bpt/ept contents dropped; text trimmed
		t.Errorf("source flatten wrong: %q", u.SourceText)
	}
	if u.TargetText != "Cliquez ici" { // surrounding whitespace trimmed
		t.Errorf("target trim wrong: %q", u.TargetText)
	}
}

// A declared srclang that's absent from a TU drops it rather than mislabeling.
func TestTMXUnmarshalSrclangAbsent(t *testing.T) {
	in := `<?xml version="1.0"?>
<tmx version="1.4"><header srclang="en"/><body>
<tu><tuv xml:lang="fr"><seg>Chien</seg></tuv><tuv xml:lang="es"><seg>Perro</seg></tuv></tu>
</body></tmx>`
	got, skipped, err := UnmarshalTMX([]byte(in))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 || skipped != 1 {
		t.Fatalf("expected 0 units + 1 skipped (no en source), got %d units, %d skipped: %+v", len(got), skipped, got)
	}
}
