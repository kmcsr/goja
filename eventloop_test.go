
package goja

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func newRuntimeForTest(t *testing.T)(vm *Runtime){
	vm = New()
	vm.Set("log", func(msg string) {
		t.Log(time.Now().Format("15:04:05.000"), "console:", msg)
	})
	return
}

func TestLoopRun(t *testing.T) {
	t.Parallel()
	const SCRIPT = `
	setTimeout(function() {
		log("ok");
	}, 1000);
	log("Started");
	`

	loop := newRuntimeForTest(t)
	prg, err := Compile("main.js", SCRIPT, false)
	if err != nil {
		t.Fatal(err)
	}
	loop.Run(func(vm *Runtime)(error){
		vm.RunProgram(prg)
		return nil
	})
}

func TestLoopStart(t *testing.T) {
	t.Parallel()
	const SCRIPT = `
	setTimeout(function() {
		log("ok");
	}, 1000);
	log("Started");
	`

	prg, err := Compile("main.js", SCRIPT, false)
	if err != nil {
		t.Fatal(err)
	}

	loop := newRuntimeForTest(t)
	loop.Start()

	loop.RunOnLoop(func(vm *Runtime)(error){
		vm.RunProgram(prg)
		return nil
	})

	time.Sleep(2 * time.Second)
	loop.Stop()
}

func TestLoopInterval(t *testing.T) {
	t.Parallel()
	const SCRIPT = `
	var count = 0;
	var t = setInterval(function() {
		log("tick");
		if (++count > 2) {
			clearInterval(t);
		}
	}, 1000);
	log("Started");
	`

	loop := newRuntimeForTest(t)
	prg, err := Compile("main.js", SCRIPT, false)
	if err != nil {
		t.Fatal(err)
	}
	loop.Run(func(vm *Runtime)(error){
		vm.RunProgram(prg)
		return nil
	})
}

func TestLoopRunNoSchedule(t *testing.T) {
	loop := newRuntimeForTest(t)
	fired := false
	loop.Run(func(vm *Runtime)(error){ // should not hang
		fired = true
		// do not schedule anything
		return nil
	})

	if !fired {
		t.Fatal("Not fired")
	}
}

func TestLoopClearIntervalRace(t *testing.T) {
	t.Parallel()
	const SCRIPT = `
	log("calling setInterval");
	var t = setInterval(function() {
		log("tick");
	}, 500);
	log("calling sleep");
	sleep(2000);
	log("calling clearInterval");
	clearInterval(t);
	`

	loop := newRuntimeForTest(t)
	prg, err := Compile("main.js", SCRIPT, false)
	if err != nil {
		t.Fatal(err)
	}
	// Should not hang
	loop.Run(func(vm *Runtime)(error){
		vm.Set("sleep", func(ms int) {
			<-time.After(time.Duration(ms) * time.Millisecond)
		})
		vm.RunProgram(prg)
		return nil
	})
}

func TestLoopNativeTimeout(t *testing.T) {
	t.Parallel()
	fired := false
	loop := newRuntimeForTest(t)
	loop.SetTimeout(func(*Runtime)(error){
		fired = true
		return nil
	}, 1*time.Second)
	loop.Run(func(*Runtime)(error){
		// do not schedule anything
		return nil
	})
	if !fired {
		t.Fatal("Not fired")
	}
}

func TestLoopNativeClearTimeout(t *testing.T) {
	t.Parallel()
	fired := false
	loop := newRuntimeForTest(t)
	timer := loop.SetTimeout(func(*Runtime)(error){
		fired = true
		return nil
	}, 2*time.Second)
	loop.SetTimeout(func(*Runtime)(error){
		loop.ClearTimeout(timer)
		return nil
	}, 1*time.Second)
	loop.Run(func(*Runtime)(error){
		// do not schedule anything
		return nil
	})
	if fired {
		t.Fatal("Cancelled timer fired!")
	}
}

func TestLoopNativeInterval(t *testing.T) {
	t.Parallel()
	count := 0
	loop := newRuntimeForTest(t)
	var i *Interval
	i = loop.SetInterval(func(*Runtime)(error){
		t.Log("tick")
		count++
		if count > 2 {
			loop.ClearInterval(i)
		}
		return nil
	}, 1*time.Second)
	loop.Run(nil)
	if count != 3 {
		t.Fatal("Expected interval to fire 3 times, got", count)
	}
}

func TestLoopNativeClearInterval(t *testing.T) {
	t.Parallel()
	count := 0
	loop := newRuntimeForTest(t)
	loop.Run(func(*Runtime)(error){
		i := loop.SetInterval(func(*Runtime)(error){
			t.Log("tick")
			count++
			return nil
		}, 500*time.Millisecond)
		<-time.After(2 * time.Second)
		loop.ClearInterval(i)
		return nil
	})
	if count != 0 {
		t.Fatal("Expected interval to fire 0 times, got", count)
	}
}

func TestLoopSetTimeoutConcurrent(t *testing.T) {
	t.Parallel()
	loop := newRuntimeForTest(t)
	loop.Start()
	ch := make(chan struct{}, 1)
	loop.SetTimeout(func(*Runtime)(error){
		ch <- struct{}{}
		return nil
	}, 100*time.Millisecond)
	<-ch
	loop.Stop()
}

func TestLoopClearTimeoutConcurrent(t *testing.T) {
	t.Parallel()
	loop := newRuntimeForTest(t)
	loop.Start()
	timer := loop.SetTimeout(func(*Runtime)(error){
		return nil
	}, 100*time.Millisecond)
	loop.ClearTimeout(timer)
	loop.StopAndWait()
	if c := loop.jobCount; c != 0 {
		t.Fatalf("jobCount: %d", c)
	}
}

func TestLoopClearIntervalConcurrent(t *testing.T) {
	t.Parallel()
	loop := newRuntimeForTest(t)
	loop.Start()
	ch := make(chan struct{}, 1)
	i := loop.SetInterval(func(*Runtime)(error){
		ch <- struct{}{}
		return nil
	}, 500*time.Millisecond)

	<-ch
	loop.ClearInterval(i)
	loop.StopAndWait()
	if c := loop.jobCount; c != 0 {
		t.Fatalf("jobCount: %d", c)
	}
}

func TestLoopRunOnStoppedLoop(t *testing.T) {
	t.Parallel()
	loop := newRuntimeForTest(t)
	var failed int32
	done := make(chan struct{})
	go func() {
		for atomic.LoadInt32(&failed) == 0 {
			loop.Start()
			time.Sleep(10 * time.Millisecond)
			loop.StopAndWait()
		}
	}()
	go func() {
		time.Sleep(3 * time.Millisecond)
		for atomic.LoadInt32(&failed) == 0 {
			loop.RunOnLoop(func(*Runtime)(error){
				if !loop.Running() {
					atomic.StoreInt32(&failed, 1)
					close(done)
					return nil
				}
				return nil
			})
			time.Sleep(10 * time.Millisecond)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	if atomic.LoadInt32(&failed) != 0 {
		t.Fatal("running job on stopped loop")
	}
}

func TestLoopPromise(t *testing.T) {
	t.Parallel()
	const SCRIPT = `
	let result;
	const p = new Promise((resolve, reject) => {
		setTimeout(() => {resolve("passed")}, 500);
	});
	p.then(value => {
		result = value;
	});
	`

	loop := newRuntimeForTest(t)
	prg, err := Compile("main.js", SCRIPT, false)
	if err != nil {
		t.Fatal(err)
	}
	loop.Run(func(vm *Runtime)(error){
		_, err = vm.RunProgram(prg)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	loop.Run(func(vm *Runtime)(error){
		result := vm.Get("result")
		if !result.SameAs(vm.ToValue("passed")) {
			err = fmt.Errorf("unexpected result: %v", result)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoopPromiseNative(t *testing.T) {
	t.Parallel()
	const SCRIPT = `
	let result;
	p.then(value => {
		result = value;
		done();
	});
	`

	loop := newRuntimeForTest(t)
	prg, err := Compile("main.js", SCRIPT, false)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan error)
	loop.Start()
	defer loop.Stop()

	loop.RunOnLoop(func(vm *Runtime)(error){
		vm.Set("done", func() {
			ch <- nil
		})
		p, resolve, _ := vm.NewPromise()
		vm.Set("p", p)
		_, err = vm.RunProgram(prg)
		if err != nil {
			ch <- err
			return nil
		}
		go func() {
			time.Sleep(500 * time.Millisecond)
			loop.RunOnLoop(func(*Runtime)(error){
				resolve("passed")
				return nil
			})
		}()
		return nil
	})
	err = <-ch
	if err != nil {
		t.Fatal(err)
	}
	loop.RunOnLoop(func(vm *Runtime)(error){
		result := vm.Get("result")
		if !result.SameAs(vm.ToValue("passed")) {
			ch <- fmt.Errorf("unexpected result: %v", result)
		} else {
			ch <- nil
		}
		return nil
	})
	err = <-ch
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoopStopNoWait(t *testing.T) {
	t.Parallel()
	loop := newRuntimeForTest(t)
	var ran int32
	loop.Run(func(runtime *Runtime)(error){
		loop.SetTimeout(func(*Runtime)(error){
			atomic.StoreInt32(&ran, 1)
			return nil
		}, 5*time.Second)

		loop.SetTimeout(func(*Runtime)(error){
			loop.Stop()
			return nil
		}, 500*time.Millisecond)

		return nil
	})

	if atomic.LoadInt32(&ran) != 0 {
		t.Fatal("ran != 0")
	}
}
