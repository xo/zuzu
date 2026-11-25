package zuzu

import (
	"cmp"
	"math"
	"time"
	"unicode/utf16"

	"github.com/xo/zuzu/expr"
)

// The sort of mixed values. JavaScript compares a string and a number as
// two numbers, so "12 Angry Men" and 1776 are neither less nor greater than
// each other, and a sort of mixed values is not a total order. The order
// that vega.js gives then depends on which pairs the sort of the engine
// compares. To give the same order, zuzu sorts such values with the same
// algorithm as Array.prototype.sort of V8, the engine of Node.js: TimSort,
// as CPython documents it in listsort.txt, with the same comparisons in the
// same order. On a total order, it gives the same result as any stable
// sort.

// minGallop is the number of wins in a row of one run after which a merge
// starts to gallop, and where the threshold of a sort starts.
const minGallop = 7

// jsSort sorts a slice in place as Array.prototype.sort of V8 sorts an
// array with a compare function.
func jsSort[T any](a []T, cmp func(x, y T) int) {
	n := len(a)
	if n < 2 {
		return
	}
	s := &timSort[T]{a: a, cmp: cmp, minGallop: minGallop}
	minRun := minRunLength(n)
	low, remaining := 0, n
	for remaining != 0 {
		run := s.countRun(low, low+remaining)
		if run < minRun {
			forced := min(minRun, remaining)
			s.insertionSort(low, low+run, low+forced)
			run = forced
		}
		s.runs = append(s.runs, sortRun{base: low, length: run})
		s.mergeCollapse()
		low += run
		remaining -= run
	}
	s.mergeForceCollapse()
}

// sortRun is a run of a slice that is already sorted.
type sortRun struct {
	base, length int
}

// timSort holds the state of one sort.
type timSort[T any] struct {
	a   []T
	cmp func(x, y T) int
	// runs is the stack of the runs that wait to merge.
	runs []sortRun
	// minGallop is the number of wins in a row after which a merge
	// gallops. It adapts to the data.
	minGallop int
}

// minRunLength returns the shortest run for a slice of n elements: n
// itself below 64, else a number from 32 to 64 such that n divided by it
// is a power of two or a little less.
func minRunLength(n int) int {
	r := 0
	for n >= 64 {
		r |= n & 1
		n >>= 1
	}
	return n + r
}

// countRun returns the length of the run that starts at low and ends
// before high. A run either never descends or strictly descends, and a
// descending run is reversed.
func (s *timSort[T]) countRun(low, high int) int {
	if low+1 == high {
		return 1
	}
	length := 2
	descending := s.cmp(s.a[low+1], s.a[low]) < 0
	prev := s.a[low+1]
	for i := low + 2; i < high; i++ {
		order := s.cmp(s.a[i], prev)
		if descending && order >= 0 || !descending && order < 0 {
			break
		}
		prev = s.a[i]
		length++
	}
	if descending {
		for i, j := low, low+length-1; i < j; i, j = i+1, j-1 {
			s.a[i], s.a[j] = s.a[j], s.a[i]
		}
	}
	return length
}

// insertionSort sorts the elements from low to high, of which those from
// low to start are already sorted, with a binary search for the place of
// each element.
func (s *timSort[T]) insertionSort(low, start, high int) {
	if low == start {
		start++
	}
	for ; start < high; start++ {
		left, right := low, start
		pivot := s.a[start]
		for left < right {
			mid := left + (right-left)>>1
			if s.cmp(pivot, s.a[mid]) < 0 {
				right = mid
			} else {
				left = mid + 1
			}
		}
		copy(s.a[left+1:start+1], s.a[left:start])
		s.a[left] = pivot
	}
}

// established reports whether the run n and the two runs below it on the
// stack keep the rule of the lengths: each run is longer than the two
// above it together.
func (s *timSort[T]) established(n int) bool {
	if n < 2 {
		return true
	}
	return s.runs[n-2].length > s.runs[n-1].length+s.runs[n].length
}

// mergeCollapse merges the runs on the stack until the rule of the lengths
// holds again.
func (s *timSort[T]) mergeCollapse() {
	for len(s.runs) > 1 {
		n := len(s.runs) - 2
		switch {
		case !s.established(n+1) || !s.established(n):
			if s.runs[n-1].length < s.runs[n+1].length {
				n--
			}
			s.mergeAt(n)
		case s.runs[n].length <= s.runs[n+1].length:
			s.mergeAt(n)
		default:
			return
		}
	}
}

// mergeForceCollapse merges all the runs on the stack into one.
func (s *timSort[T]) mergeForceCollapse() {
	for len(s.runs) > 1 {
		n := len(s.runs) - 2
		if n > 0 && s.runs[n-1].length < s.runs[n+1].length {
			n--
		}
		s.mergeAt(n)
	}
}

// mergeAt merges the runs i and i+1 of the stack.
func (s *timSort[T]) mergeAt(i int) {
	baseA, lengthA := s.runs[i].base, s.runs[i].length
	baseB, lengthB := s.runs[i+1].base, s.runs[i+1].length
	s.runs[i].length = lengthA + lengthB
	if i == len(s.runs)-3 {
		s.runs[i+1] = s.runs[i+2]
	}
	s.runs = s.runs[:len(s.runs)-1]
	// The elements of A that are not greater than the first of B are
	// already in place, and so are the elements of B that are not less
	// than the last of A.
	k := s.gallopRight(s.a, s.a[baseB], baseA, lengthA, 0)
	baseA += k
	lengthA -= k
	if lengthA == 0 {
		return
	}
	lengthB = s.gallopLeft(s.a, s.a[baseA+lengthA-1], baseB, lengthB, lengthB-1)
	if lengthB == 0 {
		return
	}
	if lengthA <= lengthB {
		s.mergeLow(baseA, lengthA, baseB, lengthB)
	} else {
		s.mergeHigh(baseA, lengthA, baseB, lengthB)
	}
}

// gallopLeft returns where key goes in the sorted elements of arr from
// base to base+length, left of any element equal to it. It starts at
// base+hint and steps 1, 3, 7 and so on, then searches between two steps.
func (s *timSort[T]) gallopLeft(arr []T, key T, base, length, hint int) int {
	last, ofs := 0, 1
	if s.cmp(arr[base+hint], key) < 0 {
		// The key is right of hint.
		maxOfs := length - hint
		for ofs < maxOfs {
			if s.cmp(arr[base+hint+ofs], key) >= 0 {
				break
			}
			last = ofs
			ofs = ofs<<1 + 1
		}
		ofs = min(ofs, maxOfs)
		last += hint
		ofs += hint
	} else {
		// The key is at hint or left of it.
		maxOfs := hint + 1
		for ofs < maxOfs {
			if s.cmp(arr[base+hint-ofs], key) < 0 {
				break
			}
			last = ofs
			ofs = ofs<<1 + 1
		}
		ofs = min(ofs, maxOfs)
		last, ofs = hint-ofs, hint-last
	}
	last++
	for last < ofs {
		m := last + (ofs-last)>>1
		if s.cmp(arr[base+m], key) < 0 {
			last = m + 1
		} else {
			ofs = m
		}
	}
	return ofs
}

// gallopRight is gallopLeft, except that the key goes right of any element
// equal to it.
func (s *timSort[T]) gallopRight(arr []T, key T, base, length, hint int) int {
	last, ofs := 0, 1
	if s.cmp(key, arr[base+hint]) < 0 {
		// The key is left of hint.
		maxOfs := hint + 1
		for ofs < maxOfs {
			if s.cmp(key, arr[base+hint-ofs]) >= 0 {
				break
			}
			last = ofs
			ofs = ofs<<1 + 1
		}
		ofs = min(ofs, maxOfs)
		last, ofs = hint-ofs, hint-last
	} else {
		// The key is at hint or right of it.
		maxOfs := length - hint
		for ofs < maxOfs {
			if s.cmp(key, arr[base+hint+ofs]) < 0 {
				break
			}
			last = ofs
			ofs = ofs<<1 + 1
		}
		ofs = min(ofs, maxOfs)
		last += hint
		ofs += hint
	}
	last++
	for last < ofs {
		m := last + (ofs-last)>>1
		if s.cmp(key, arr[base+m]) < 0 {
			ofs = m
		} else {
			last = m + 1
		}
	}
	return ofs
}

// mergeLow merges the runs A and B, which lie side by side, when A is the
// shorter. It copies A aside and fills the slice from the left.
func (s *timSort[T]) mergeLow(baseA, lengthA, baseB, lengthB int) {
	a := s.a
	tmp := make([]T, lengthA)
	copy(tmp, a[baseA:baseA+lengthA])
	dest, ca, cb := baseA, 0, baseB
	a[dest] = a[cb]
	dest++
	cb++
	lengthB--
	// copyB ends the merge when one element of A is left: the rest of B
	// goes before it.
	copyB := func() {
		copy(a[dest:dest+lengthB], a[cb:cb+lengthB])
		a[dest+lengthB] = tmp[ca]
	}
	// done ends the merge when B is used up: the rest of A goes last.
	done := func() {
		copy(a[dest:dest+lengthA], tmp[ca:ca+lengthA])
	}
	if lengthB == 0 {
		done()
		return
	}
	if lengthA == 1 {
		copyB()
		return
	}
	gallop := s.minGallop
	for {
		winsA, winsB := 0, 0
		// Take one element at a time until one run wins often.
		for {
			if s.cmp(a[cb], tmp[ca]) < 0 {
				a[dest] = a[cb]
				dest++
				cb++
				winsB++
				lengthB--
				winsA = 0
				if lengthB == 0 {
					done()
					return
				}
				if winsB >= gallop {
					break
				}
			} else {
				a[dest] = tmp[ca]
				dest++
				ca++
				winsA++
				lengthA--
				winsB = 0
				if lengthA == 1 {
					copyB()
					return
				}
				if winsA >= gallop {
					break
				}
			}
		}
		// Gallop while it takes many elements at once.
		gallop++
		for first := true; first || winsA >= minGallop || winsB >= minGallop; first = false {
			gallop = max(1, gallop-1)
			s.minGallop = gallop
			winsA = s.gallopRight(tmp, a[cb], ca, lengthA, 0)
			if winsA > 0 {
				copy(a[dest:dest+winsA], tmp[ca:ca+winsA])
				dest += winsA
				ca += winsA
				lengthA -= winsA
				if lengthA == 1 {
					copyB()
					return
				}
				// A compare function that is not consistent can use A up.
				if lengthA == 0 {
					done()
					return
				}
			}
			a[dest] = a[cb]
			dest++
			cb++
			lengthB--
			if lengthB == 0 {
				done()
				return
			}
			winsB = s.gallopLeft(a, tmp[ca], cb, lengthB, 0)
			if winsB > 0 {
				copy(a[dest:dest+winsB], a[cb:cb+winsB])
				dest += winsB
				cb += winsB
				lengthB -= winsB
				if lengthB == 0 {
					done()
					return
				}
			}
			a[dest] = tmp[ca]
			dest++
			ca++
			lengthA--
			if lengthA == 1 {
				copyB()
				return
			}
		}
		gallop++
		s.minGallop = gallop
	}
}

// mergeHigh merges the runs A and B, which lie side by side, when B is the
// shorter. It copies B aside and fills the slice from the right.
func (s *timSort[T]) mergeHigh(baseA, lengthA, baseB, lengthB int) {
	a := s.a
	tmp := make([]T, lengthB)
	copy(tmp, a[baseB:baseB+lengthB])
	dest, ca, cb := baseB+lengthB-1, baseA+lengthA-1, lengthB-1
	a[dest] = a[ca]
	dest--
	ca--
	lengthA--
	// copyA ends the merge when one element of B is left: the rest of A
	// goes after it.
	copyA := func() {
		dest -= lengthA
		ca -= lengthA
		copy(a[dest+1:dest+1+lengthA], a[ca+1:ca+1+lengthA])
		a[dest] = tmp[cb]
	}
	// done ends the merge when A is used up: the rest of B goes first.
	done := func() {
		if lengthB > 0 {
			copy(a[dest-(lengthB-1):dest+1], tmp[:lengthB])
		}
	}
	if lengthA == 0 {
		done()
		return
	}
	if lengthB == 1 {
		copyA()
		return
	}
	gallop := s.minGallop
	for {
		winsA, winsB := 0, 0
		// Take one element at a time until one run wins often.
		for {
			if s.cmp(tmp[cb], a[ca]) < 0 {
				a[dest] = a[ca]
				dest--
				ca--
				winsA++
				lengthA--
				winsB = 0
				if lengthA == 0 {
					done()
					return
				}
				if winsA >= gallop {
					break
				}
			} else {
				a[dest] = tmp[cb]
				dest--
				cb--
				winsB++
				lengthB--
				winsA = 0
				if lengthB == 1 {
					copyA()
					return
				}
				if winsB >= gallop {
					break
				}
			}
		}
		// Gallop while it takes many elements at once.
		gallop++
		for first := true; first || winsA >= minGallop || winsB >= minGallop; first = false {
			gallop = max(1, gallop-1)
			s.minGallop = gallop
			k := s.gallopRight(a, tmp[cb], baseA, lengthA, lengthA-1)
			winsA = lengthA - k
			if winsA > 0 {
				dest -= winsA
				ca -= winsA
				copy(a[dest+1:dest+1+winsA], a[ca+1:ca+1+winsA])
				lengthA -= winsA
				if lengthA == 0 {
					done()
					return
				}
			}
			a[dest] = tmp[cb]
			dest--
			cb--
			lengthB--
			if lengthB == 1 {
				copyA()
				return
			}
			k = s.gallopLeft(tmp, a[ca], 0, lengthB, lengthB-1)
			winsB = lengthB - k
			if winsB > 0 {
				dest -= winsB
				cb -= winsB
				copy(a[dest+1:dest+1+winsB], tmp[cb+1:cb+1+winsB])
				lengthB -= winsB
				if lengthB == 1 {
					copyA()
					return
				}
				// A compare function that is not consistent can use B up.
				if lengthB == 0 {
					done()
					return
				}
			}
			a[dest] = a[ca]
			dest--
			ca--
			lengthA--
			if lengthA == 0 {
				done()
				return
			}
		}
		gallop++
		s.minGallop = gallop
	}
}

// vegaAscending compares two values as the ascending order of Vega does,
// with < and > of JavaScript: null and undefined come first, then NaN, and
// two values that do not compare, such as a number and a string that is
// not a number, are equal.
func vegaAscending(u, v any) int {
	uNull, vNull := u == nil || expr.IsUndefined(u), v == nil || expr.IsUndefined(v)
	lt, gt := false, false
	if !uNull && !vNull {
		lt, gt = jsLess(u, v), jsLess(v, u)
	}
	switch {
	case (lt || uNull) && !vNull:
		return -1
	case (gt || vNull) && !uNull:
		return 1
	}
	uNaN, vNaN := isNaN(u), isNaN(v)
	switch {
	case uNaN && !vNaN:
		return -1
	case vNaN && !uNaN:
		return 1
	}
	return 0
}

// isNaN reports whether a value is the number NaN or a date that is not
// valid, which is NaN as a number.
func isNaN(v any) bool {
	switch x := v.(type) {
	case float64:
		return math.IsNaN(x)
	case time.Time:
		return x.IsZero()
	}
	return false
}

// jsLess is a < b of JavaScript for two values that are not null or
// undefined. Two strings compare by their units of UTF-16, and any other
// pair compares as numbers, so that a number is never less than a string
// that is not a number.
func jsLess(a, b any) bool {
	pa, pb := jsPrimitive(a), jsPrimitive(b)
	sa, okA := pa.(string)
	sb, okB := pb.(string)
	if okA && okB {
		return compareUTF16(sa, sb) < 0
	}
	return expr.Number(pa) < expr.Number(pb)
}

// jsPrimitive converts a value to a primitive as a comparison of
// JavaScript does: a date to its number, and an array or an object to its
// string.
func jsPrimitive(v any) any {
	switch x := v.(type) {
	case time.Time:
		return expr.Number(x)
	case []any, map[string]any:
		return expr.String(x)
	}
	return v
}

// compareUTF16 compares two strings by their units of UTF-16, as
// JavaScript does. It differs from the order of the bytes of UTF-8 only
// where a character outside the basic plane meets one from U+E000 to
// U+FFFF.
func compareUTF16(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		x, y := ra[i], rb[i]
		if x == y {
			continue
		}
		if x < 0x10000 || y < 0x10000 {
			// A character outside the basic plane starts with a high
			// surrogate, which comes before U+E000.
			x, y = firstUnit(x), firstUnit(y)
		}
		return cmp.Compare(x, y)
	}
	return cmp.Compare(len(ra), len(rb))
}

// firstUnit returns the first unit of UTF-16 of a character.
func firstUnit(r rune) rune {
	if r >= 0x10000 {
		hi, _ := utf16.EncodeRune(r)
		return hi
	}
	return r
}
