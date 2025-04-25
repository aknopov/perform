package main

// Tests quite often replace original functions by the mock ones.
// Functions below preserve original and return function that
// restores it with `defer replaceFun...(...)()` . Note extra brackets.

func replaceFunPlain[T any](targFun *func() T, newFun func() T) func() {
	saveFun := *targFun
	*targFun = newFun
	return func() { *targFun = saveFun }
}

func replaceFun0[T any](targFun *func() (T, error), newFun func() (T, error)) func() {
	saveFun := *targFun
	*targFun = newFun
	return func() { *targFun = saveFun }
}

func replaceFun1[T any, A1 any](targFun *func(A1) (T, error), newFun func(A1) (T, error)) func() {
	saveFun := *targFun
	*targFun = newFun
	return func() { *targFun = saveFun }
}
