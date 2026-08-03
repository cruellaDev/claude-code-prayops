// Package effect places and dissolves prayer visuals.
//
// Everything here is a pure function of a seed. The same seed, layout, and
// raster must produce the same placement and the same dissolve on every
// machine and every run, so nothing in this package reads a clock or a global
// random source.
package effect

// splitmix64 is a fixed, portable mixer. A hash from the standard library
// would do, but the values would then be tied to that implementation; pinning
// the arithmetic here keeps a recorded seed meaningful across Go versions.
func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// saltOf folds a short label into the hash so the same cell can have several
// independent values - one for reveal, another for dissolve.
func saltOf(salt string) uint64 {
	// FNV-1a over the label.
	h := uint64(14695981039346656037)
	for i := 0; i < len(salt); i++ {
		h ^= uint64(salt[i])
		h *= 1099511628211
	}
	return h
}

// Hash01 returns a stable value in [0,1) for one cell and purpose.
func Hash01(seed uint64, x, y int, salt string) float64 {
	h := splitmix64(seed ^ saltOf(salt))
	h = splitmix64(h ^ uint64(int64(x))*0x9e3779b97f4a7c15)
	h = splitmix64(h ^ uint64(int64(y))*0xc2b2ae3d27d4eb4f)

	// 53 bits is the most a float64 can hold exactly.
	return float64(h>>11) / float64(uint64(1)<<53)
}

// rng is a tiny deterministic source for placement decisions.
type rng struct{ state uint64 }

func newRNG(seed uint64) *rng { return &rng{state: seed} }

func (r *rng) next() uint64 {
	r.state = splitmix64(r.state)
	return r.state
}

// intN returns a value in [0,n).
func (r *rng) intN(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// float64 returns a value in [0,1).
func (r *rng) float64() float64 { return float64(r.next()>>11) / float64(uint64(1)<<53) }
