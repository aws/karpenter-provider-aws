package matchers_test

var (
	universalElements = []string{"foo", "bar", "baz"}
	universalMap      = map[string]int{
		"foo": 0,
		"bar": 42,
		"baz": 666,
	}
	fooElements = []string{"foo", "foo", "foo"}
)

func universalIter(yield func(string) bool) {
	for _, element := range universalElements {
		if !yield(element) {
			return
		}
	}
}

func universalIter2(yield func(int, string) bool) {
	for idx, element := range universalElements {
		if !yield(idx, element) {
			return
		}
	}
}

func emptyIter(yield func(string) bool) {}

func emptyIter2(yield func(int, string) bool) {}

func universalMapIter2(yield func(string, int) bool) {
	for k, v := range universalMap {
		if !yield(k, v) {
			return
		}
	}
}

func fooIter(yield func(string) bool) {
	for _, foo := range fooElements {
		if !yield(foo) {
			return
		}
	}
}

func fooIter2(yield func(int, string) bool) {
	for idx, foo := range fooElements {
		if !yield(idx, foo) {
			return
		}
	}
}
