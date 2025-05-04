// Package mocker provides useful tools that can be used in unit tests
package mocker

// Tests quite often require to replace original functions or state variables by the mock ones.
// Function below preserves and restores an item (function or variable).
//
// This function should be used like
//
// 		defer mocker.ReplaceItem(&orgVal, newVal)()
//
// - note extra brackets.
func ReplaceItem[T any](orgVal *T, newVal T) func() {
	saveVal := *orgVal
	*orgVal = newVal
	return func() { *orgVal = saveVal }
}
