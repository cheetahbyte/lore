package identity

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"github:owner/repo", false},
		{"npm:react", false},
		{"url:https://docs.example.com/x", false},
		{"nope", true},
		{"bogus:thing", true},
		{"github:", true},
	}
	for _, c := range cases {
		id, err := Parse(c.in)
		if c.wantErr && err == nil {
			t.Errorf("Parse(%q): expected error, got id %q", c.in, id)
		}
		if !c.wantErr && err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", c.in, err)
		}
	}
}

func TestTypeAndRef(t *testing.T) {
	id := New(URL, "https://docs.example.com/x")
	if id.Type() != URL {
		t.Errorf("Type() = %q, want %q", id.Type(), URL)
	}
	if id.Ref() != "https://docs.example.com/x" {
		t.Errorf("Ref() = %q", id.Ref())
	}
}
