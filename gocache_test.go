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

func TestSet(t *testing.T) {
	tests := []struct {
		key      interface{}
		val      interface{}
		expected bool
	}{
		{
			key:      "key-1",
			val:      "key-1_value",
			expected: true,
		},
	}

	g := New()

	for i, test := range tests {
		ok := g.Set(test.key, test.val)
		if test.expected != ok {
			t.Errorf("tests[%d] - Set is wrong. expected: %v, got: %v", i, test.expected, ok)
		}
	}
}

func TestSetWithExpire(t *testing.T) {
	tests := []struct {
		key      interface{}
		val      interface{}
		expire   time.Duration
		expected bool
	}{
		{
			key:      "key-1",
			val:      "key-1_value",
			expire:   time.Second * 100,
			expected: true,
		},
		{
			key:      "key-1",
			val:      "key-1_value",
			expire:   time.Second * 0,
			expected: false,
		},
		{
			key:      "key-1",
			val:      "key-1_value",
			expire:   time.Second * -100,
			expected: false,
		},
	}

	g := New()

	for i, test := range tests {
		ok := g.SetWithExpire(test.key, test.val, test.expire)
		if test.expected != ok {
			t.Errorf("tests[%d] - SetWithExpire is wrong. expected: %v, got: %v", i, test.expected, ok)
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

func TestDelete(t *testing.T) {
	tests := []struct {
		key      interface{}
		val      interface{}
		expected bool
	}{
		{
			key:      "key-1",
			val:      "key-1_value",
			expected: true,
		},
	}

	g := New()

	for i, test := range tests {
		ok := g.Set(test.key, test.val)
		if !ok {
			t.Errorf("tests[%d] - Set ok is wrong. expected: %v, got: %v", i, true, ok)
		}

		ok = g.Delete(test.key)
		if !ok {
			t.Errorf("tests[%d] - Delete ok is wrong. expected: %v, got: %v", i, true, ok)
		}

		_, ok = g.Get(test.key)
		if ok {
			t.Errorf("tests[%d] - Set ok is wrong. expected: %v, got: %v", i, false, ok)
		}
	}
}
