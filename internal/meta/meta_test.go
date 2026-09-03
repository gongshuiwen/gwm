package meta

import (
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid", input: `{"description":"修复","protected":true}`},
		{name: "null", input: `{"description":null,"protected":false}`},
		{name: "missing", input: `{"description":null}`, wantErr: true},
		{name: "unknown", input: `{"description":null,"protected":false,"id":1}`, wantErr: true},
		{name: "duplicate", input: `{"description":null,"description":"x","protected":false}`, wantErr: true},
		{name: "nul", input: `{"description":"\u0000","protected":false}`, wantErr: true},
		{name: "wrong type", input: `{"description":1,"protected":false}`, wantErr: true},
		{name: "trailing", input: `{"description":null,"protected":false}{}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Decode([]byte(test.input))
			if (err != nil) != test.wantErr {
				t.Fatalf("Decode() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestEncodeDescriptionLimit(t *testing.T) {
	valid := strings.Repeat("界", 1365)
	if _, err := Encode(Metadata{Description: &valid}); err != nil {
		t.Fatalf("Encode(valid) error = %v", err)
	}
	tooLarge := strings.Repeat("界", 1366)
	if _, err := Encode(Metadata{Description: &tooLarge}); err == nil {
		t.Fatal("Encode(tooLarge) succeeded")
	}
}

func TestEncodeOrder(t *testing.T) {
	value := "text"
	got, err := Encode(Metadata{Description: &value, Protected: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"description":"text","protected":true}`
	if string(got) != want {
		t.Fatalf("Encode() = %s, want %s", got, want)
	}
}
