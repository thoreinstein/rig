package internal

import "testing"

type mockInterface interface {
	Do()
}

type mockImpl struct{}

func (m *mockImpl) Do() {}

func TestIsNilInterface(t *testing.T) {
	t.Parallel()

	var nilMock mockInterface = (*mockImpl)(nil)
	var validMock mockInterface = &mockImpl{}

	tests := []struct {
		name string
		v    any
		want bool
	}{
		{name: "plain nil", v: nil, want: true},
		{name: "typed-nil pointer", v: (*mockImpl)(nil), want: true},
		{name: "interface with typed-nil pointer", v: nilMock, want: true},
		{name: "valid pointer", v: &mockImpl{}, want: false},
		{name: "interface with valid pointer", v: validMock, want: false},
		{name: "non-pointer value", v: 42, want: false},
		{name: "string value", v: "hello", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsNilInterface(tt.v); got != tt.want {
				t.Errorf("IsNilInterface(%v) = %v, want %v", tt.v, got, tt.want)
			}
		})
	}
}
