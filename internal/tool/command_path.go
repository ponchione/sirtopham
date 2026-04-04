package tool

import (
	"fmt"
	"os/exec"
	"sync"
)

var commandPathCache sync.Map

type commandPathResult struct {
	path string
	err  error
}

func lookupCommandPath(name string) (string, error) {
	entry, _ := commandPathCache.LoadOrStore(name, &struct {
		once sync.Once
		res  commandPathResult
	}{})
	state := entry.(*struct {
		once sync.Once
		res  commandPathResult
	})

	state.once.Do(func() {
		path, err := exec.LookPath(name)
		if err != nil {
			state.res.err = fmt.Errorf("%s not found in PATH", name)
			return
		}
		state.res.path = path
	})

	return state.res.path, state.res.err
}
