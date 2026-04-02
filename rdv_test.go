package rendezvous

import (
	"fmt"
	"hash/fnv"
	"testing"
	"testing/quick"

	"github.com/cespare/xxhash/v2"
)

func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func TestEmpty(t *testing.T) {
	r := New([]string{}, hashString)
	r.Lookup("hello")
}

func TestLookupN(t *testing.T) {
	r := New(nodes(defaultNodeCount), xxhash.Sum64String)
	t.Log(r.Lookup("hello"))
	t.Log(r.LookupN("hello", 3))
}

func TestLookupN_DuplicateNode(t *testing.T) {
	// This hash function + node strings + input key produced a collision where
	// none of the N>1 hashes were bigger than the first hash, which clobbered
	// all other nodes.
	r := New([]string{"127.0.0.1:3501", "127.0.0.1:3502"}, xxhash.Sum64String)
	got := r.LookupN("96", 2)
	want := []string{"127.0.0.1:3501", "127.0.0.1:3502"}

	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%d want=%d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected node at %d: got=%q want=%q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestLookupN_StableOrdering(t *testing.T) {
	// When multiple nodes have the same hash value, we want the ordering to remain
	// stable. Without that, the result of this LookupN call would be the order in
	// which nodes were passed.
	r := New([]string{
		"127.0.0.1:3501",
		"127.0.0.1:3505",
		"127.0.0.1:3504",
		"127.0.0.1:3502",
		"127.0.0.1:3503",
	}, func(s string) uint64 { return 12 })
	got := r.LookupN("96", 5)
	want := []string{
		"127.0.0.1:3501",
		"127.0.0.1:3502",
		"127.0.0.1:3503",
		"127.0.0.1:3504",
		"127.0.0.1:3505",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected length: got=%d want=%d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected node at %d: got=%q want=%q (all=%v)", i, got[i], want[i], got)
		}
	}

	if want, have := r.Lookup("96"), r.LookupN("96", 1)[0]; have != want {
		t.Fatalf("unexpected discrepancy: have=%q want=%q", have, want)
	}
	if want, have := r.Lookup("96"), got[0]; have != want {
		t.Fatalf("unexpected discrepancy: have=%q want=%q", have, want)
	}
}

func TestLookupN_NoDuplicates(t *testing.T) {
	r := New(nodes(defaultNodeCount), xxhash.Sum64String)
	testCases := []struct {
		name string
		key  string
		n    int
	}{
		{name: "subset", key: "github.com/foo/bar", n: 3},
		{name: "all", key: "github.com/sourcegraph/sourcegraph", n: defaultNodeCount},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.LookupN(tc.key, tc.n)
			if len(got) != tc.n {
				t.Fatalf("unexpected length: got=%d want=%d", len(got), tc.n)
			}

			seen := make(map[string]struct{}, len(got))
			for _, node := range got {
				if _, ok := seen[node]; ok {
					t.Fatalf("LookupN returned duplicate node %q for key %q: %v", node, tc.key, got)
				}
				seen[node] = struct{}{}
			}
		})
	}
}

func TestLookupN_Lookup_Equivalence(t *testing.T) {
	r := New(nodes(defaultNodeCount), hashString)

	prop := func(k string) bool {
		a := r.LookupN(k, 1)[0]
		b := r.Lookup(k)
		return a == b
	}

	if err := quick.Check(prop, &quick.Config{MaxCount: 1e6}); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkLookup(b *testing.B) {
	b.ReportAllocs()
	for _, nodeCount := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("NodeCount-%d", nodeCount), func(b *testing.B) {
			r := New(nodes(nodeCount), xxhash.Sum64String)
			for i := 0; i < b.N; i++ {
				r.Lookup("github.com/foo/bar")
			}
		})
	}
}

func BenchmarkLookupN(b *testing.B) {
	b.ReportAllocs()
	for _, nodeCount := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("NodeCount-%d", nodeCount), func(b *testing.B) {
			r := New(nodes(nodeCount), xxhash.Sum64String)
			for i := 0; i < b.N; i++ {
				r.LookupN("github.com/foo/bar", 3)
			}

		})
	}
}

const defaultNodeCount = 24

func nodes(n int) []string {
	nodes := make([]string, n)
	for i := 0; i < cap(nodes); i++ {
		nodes[i] = fmt.Sprintf("indexed-search-%d.indexed-search:6070", i)
	}
	return nodes
}
