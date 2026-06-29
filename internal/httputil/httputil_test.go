package httputil

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadLimited(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		limit   int64
		want    string
		wantErr bool
	}{
		{
			name:    "under limit",
			data:    "hello",
			limit:   10,
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "at limit",
			data:    "hello",
			limit:   5,
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "over limit",
			data:    "hello world",
			limit:   5,
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty",
			data:    "",
			limit:   10,
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.data)
			got, err := ReadLimited(r, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadLimited() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("ReadLimited() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadErrorBody(t *testing.T) {
	// Small body should work
	small := "error message"
	got, err := ReadErrorBody(strings.NewReader(small))
	if err != nil {
		t.Errorf("ReadErrorBody() error = %v", err)
	}
	if string(got) != small {
		t.Errorf("ReadErrorBody() = %q, want %q", got, small)
	}

	// Large body should fail
	large := bytes.Repeat([]byte("x"), int(MaxErrorBodySize)+100)
	_, err = ReadErrorBody(bytes.NewReader(large))
	if err == nil {
		t.Error("ReadErrorBody() expected error for large body")
	}
}

func TestReadJSONBody(t *testing.T) {
	// Normal JSON should work
	json := `{"key": "value"}`
	got, err := ReadJSONBody(strings.NewReader(json))
	if err != nil {
		t.Errorf("ReadJSONBody() error = %v", err)
	}
	if string(got) != json {
		t.Errorf("ReadJSONBody() = %q, want %q", got, json)
	}

	// Large body should fail
	large := bytes.Repeat([]byte("x"), int(MaxJSONBodySize)+100)
	_, err = ReadJSONBody(bytes.NewReader(large))
	if err == nil {
		t.Error("ReadJSONBody() expected error for large body")
	}
}

func TestLimitReader(t *testing.T) {
	data := "hello world"
	r := LimitReader(strings.NewReader(data), 5)

	buf := make([]byte, 10)
	n, _ := r.Read(buf)

	if n != 5 {
		t.Errorf("LimitReader read %d bytes, want 5", n)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("LimitReader read %q, want %q", buf[:n], "hello")
	}
}
