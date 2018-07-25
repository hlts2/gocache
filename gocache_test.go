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

func TestGet(t *testing.T) {
	tests := []struct {
		key      interface{}
		val      interface{}
		expected interface{}
	}{
		{
			key:      "key-1",
			val:      "key-1_value",
			expected: "key-1_value",
		},
	}

	g := New()

	for i, test := range tests {
		ok := g.Set(test.key, test.val)
		if !ok {
			t.Errorf("tests[%d] - Set ok is wrong. expected: %v, got: %v", i, true, ok)
		}

		got, ok := g.Get(test.key)
		if !ok {
			t.Errorf("tests[%d] - Get ok is wrong. expected: %v, got: %v", i, true, ok)
		}

		if got != test.expected {
			t.Errorf("tests[%d] - Get is wrong. expected: %v, got: %v", i, test.expected, got)
		}
	}
}

func TestGetExpire(t *testing.T) {
	tests := []struct {
		key      interface{}
		val      interface{}
		expected int64
	}{
		{
			key:      "key-1",
			val:      "key-1_value",
			expected: time.Now().Add(defaultExpire).UnixNano(),
		},
	}

	g := New()

	for i, test := range tests {
		ok := g.Set(test.key, test.val)
		if !ok {
			t.Errorf("tests[%d] - Set ok is wrong. expected: %v, got: %v", i, true, ok)
		}

		got, ok := g.GetExpire(test.key)
		if !ok {
			t.Errorf("tests[%d] - GetExpire ok is wrong. expected: %v, got: %v", i, true, ok)
		}

		if got != test.expected {
			t.Errorf("tests[%d] - GetExpire is wrong. expected: %v, got: %v", i, test.expected, got)
		}
	}
}
