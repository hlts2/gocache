package gocache

import (
	"sync"
	"testing"
	"time"

	"github.com/kpango/fastime"
)

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Error("New is nil")
	}
}

func TestEntryExpire(t *testing.T) {
	tests := []struct {
		expire   int64
		expected bool
	}{
		{
			expire:   (fastime.Now().AddDate(1, 0, 0)).UnixNano(),
			expected: true,
		},
		{
			expire:   (fastime.Now().AddDate(0, 0, 0)).UnixNano(),
			expected: false,
		},
		{
			expire:   (fastime.Now().AddDate(-1, 0, 0)).UnixNano(),
			expected: false,
		},
	}

	for i, test := range tests {
		g := New().(*gocache)
		g.set("key", "val", test.expire-fastime.UnixNanoNow())

		_, got := g.Get("key")
		if test.expected != got {
			t.Errorf("tests[%d] - Get ok is wrong. expected: %v, got: %v", i, test.expected, got)
		}
	}
}

func TestConcurrent(t *testing.T) {
	g := New()

	var wg sync.WaitGroup
	keys := []string{"key-1", "key-2", "key-3", "key-4"}

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10000; j++ {
				k := keys[j%len(keys)]
				g.Set(k, j)

				if val, ok := g.Get(k); ok {
					if _, isInt := val.(int); !isInt {
						t.Errorf("Get returned unexpected type: %T", val)
						return
					}
				}

				switch j % 100 {
				case 50:
					g.Delete(k)
				case 75:
					g.SetWithExpire(k, j, time.Hour)
				}
			}
		}(i)
	}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				g.(*gocache).DeleteExpired()
			}
		}()
	}

	wg.Wait()
}

func TestSet(t *testing.T) {
	tests := []struct {
		key      string
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
		key      string
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
		key      string
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

func TestDelete(t *testing.T) {
	tests := []struct {
		key      string
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

		g.Delete(test.key)

		_, ok = g.Get(test.key)
		if ok {
			t.Errorf("tests[%d] - Set ok is wrong. expected: %v, got: %v", i, false, ok)
		}
	}
}

func TestClear(t *testing.T) {
	tests := []struct {
		keys     []string
		vals     []interface{}
		expected []bool
	}{
		{
			keys:     []string{"key-1", "key-2"},
			vals:     []interface{}{"key-1_value", "key-2_value"},
			expected: []bool{false, false},
		},
	}

	g := New()

	for i, test := range tests {
		for j := 0; j < len(test.keys); j++ {
			ok := g.Set(test.keys[j], test.vals[j])
			if !ok {
				t.Errorf("tests[%d] - Set ok is wrong. expected: %v, got: %v", i, true, ok)
			}
		}

		g.Clear()

		for j := 0; j < len(test.keys); j++ {
			val, ok := g.Get(test.keys[j])
			if ok {
				t.Errorf("tests[%d] - Get ok is wrong. expected: %v, got: %v", i, test.expected[j], ok)
			}

			if val != nil {
				t.Errorf("tests[%d] - Get is wrong. expected: %v, got: %v", i, nil, val)
			}
		}
	}
}

// func TestStartDeleteExpired(t *testing.T) {
// 	defer SetNowTime(defaultNowTimeForTest)
//
// 	tests := []struct {
// 		keys     []string
// 		vals     []interface{}
// 		expected []bool
// 	}{
// 		{
// 			keys:     []string{"key-1", "key-2"},
// 			vals:     []interface{}{"key-1_value", "key-2_value"},
// 			expected: []bool{false, false},
// 		},
// 	}
//
// 	g := New()
//
// 	for i, test := range tests {
// 		g.StartDeleteExpired(time.Millisecond * 1)
//
// 		for j := 0; j < len(test.keys); j++ {
// 			ok := g.SetWithExpire(test.keys[j], test.vals[j], time.Second*1)
// 			if !ok {
// 				t.Errorf("tests[%d] - SetWithExpire ok is wrong. expected: %v, got: %v", i, true, ok)
// 			}
// 		}
//
// 		SetNowTime(time.Date(2019, 11, 2, 0, 0, 0, 0, time.Local))
//
// 		time.Sleep(1 * time.Second)
//
// 		for j := 0; j < len(test.keys); j++ {
// 			g := g.(*gocache)
//
// 			_, ok := g.m[test.keys[j]]
// 			if ok {
// 				t.Errorf("tests[%d] - g.m[key] is wrong. expected: %v, got: %v", i, test.expected[j], ok)
// 			}
// 		}
// 	}
// }
//
// func TestStopDeleteExpired(t *testing.T) {
// 	defer SetNowTime(defaultNowTimeForTest)
//
// 	tests := []struct {
// 		keys     []string
// 		vals     []interface{}
// 		expected []bool
// 	}{
// 		{
// 			keys:     []string{"key-1", "key-2"},
// 			vals:     []interface{}{"key-1_value", "key-2_value"},
// 			expected: []bool{true, true},
// 		},
// 	}
//
// 	g := New()
// 	g.StopDeleteExpired()
//
// 	for i, test := range tests {
//
// 		for j := 0; j < len(test.keys); j++ {
// 			ok := g.Set(test.keys[j], test.vals[j])
// 			if !ok {
// 				t.Errorf("tests[%d] - SetWithExpire ok is wrong. expected: %v, got: %v", i, true, ok)
// 			}
// 		}
//
// 		SetNowTime(time.Date(2019, 11, 2, 0, 0, 0, 0, time.Local))
//
// 		for j := 0; j < len(test.keys); j++ {
// 			g := g.(*gocache)
//
// 			_, ok := g.m[test.keys[j]]
// 			if !ok {
// 				t.Errorf("tests[%d] - g.m[key] is wrong. expected: %v, got: %v", i, test.expected[j], ok)
// 			}
// 		}
// 	}
// }
//
// func TestStartingDeleteExpired(t *testing.T) {
// 	tests := []struct {
// 		g        Gocache
// 		fn       func(g Gocache)
// 		expected bool
// 	}{
// 		{
// 			g:        New().StartDeleteExpired(100 * time.Second),
// 			fn:       func(g Gocache) { g.StopDeleteExpired() },
// 			expected: true,
// 		},
// 		{
// 			g:        New().StartDeleteExpired(0 * time.Second),
// 			fn:       func(g Gocache) {},
// 			expected: false,
// 		},
// 		{
// 			g:        New().StartDeleteExpired(-1 * time.Second),
// 			fn:       func(g Gocache) {},
// 			expected: false,
// 		},
// 		{
// 			g:        New(),
// 			fn:       func(g Gocache) {},
// 			expected: false,
// 		},
// 	}
//
// 	for i, test := range tests {
// 		if test.expected != test.g.StartingDeleteExpired() {
// 			t.Errorf("tests[%d] - StartingDeleteExpired is wrong. expected: %v, got: %v", i, test.expected, test.g.StartingDeleteExpired())
// 		}
//
// 		test.fn(test.g)
// 	}
// }
