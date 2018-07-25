package gocache

import (
	"testing"
	"time"

	"github.com/bouk/monkey"
)

func init() {
	monkey.Patch(time.Now, func() time.Time {
		return time.Date(2018, 11, 2, 0, 0, 0, 0, time.Local)
	})
}

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Error("New is nil")
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		value    *value
		expected bool
	}{
		{
			value: &value{
				expire: (time.Now().AddDate(1, 0, 0)).UnixNano(),
			},
			expected: true,
		},
		{
			value: &value{
				expire: (time.Now().AddDate(0, 0, 0)).UnixNano(),
			},
			expected: false,
		},
		{
			value: &value{
				expire: (time.Now().AddDate(-1, 0, 0)).UnixNano(),
			},
			expected: false,
		},
	}

	for i, test := range tests {
		got := test.value.isValid()

		if test.expected != got {
			t.Errorf("tests[%d] - value.isValid is wrong. expected: %v, got: %v", i, test.expected, got)
		}
	}
}
