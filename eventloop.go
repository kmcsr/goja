
package goja

import (
	ctxt "context"
	"sync"
	"sync/atomic"
	"time"
)

type Callback = func()(error)

type job struct {
	alive bool
	cb    Callback
}

func makeJob(cb Callback)(j job){
	j.cb = cb
	j.alive = true
	return
}

func (j job)do()(error){
	if j.alive {
		return j.cb()
	}
	return nil
}

func (j job)cancel(){
	j.alive = false
}

type Timer struct {
	job
	timer *time.Timer
}

func (t *Timer)cancel(){
	t.job.cancel()
	t.timer.Stop()
}

type Interval struct {
	job
	ctx    ctxt.Context
	cf     ctxt.CancelFunc
	ticker *time.Ticker
}

func (i *Interval)cancel(){
	i.job.cancel()
	i.cf()
}

func (i *Interval) run(loop *EventLoop) {
	defer i.ticker.Stop()
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-i.ticker.C:
			select {
			case <-i.ctx.Done():
				return
			case loop.jobChan <- i.cb:
			}
		}
	}
}

const (
	loopIdle int32 = iota
	loopRunning
	loopStopping
)

type EventLoop struct {
	runCond *sync.Cond
	status   int32

	jobChan  chan Callback
	jobCount int32

	wakeupFlag  chan struct{}
	auxJobsLock sync.Mutex
	auxJobsSpare, auxJobs []Callback

	// TODO: use id maps insteat of pass object to js
	// timeouti uint32
	// timeouts map[uint32]*Timer
	// intervali uint32
	// intervals map[uint32]*Interval

	Onerror func(error)
}

func newEventLoop(vm *Runtime)(loop *EventLoop){
	loop = &EventLoop{
		runCond:    sync.NewCond(new(sync.Mutex)),
		status:     loopIdle,
		jobChan:    make(chan Callback),
		wakeupFlag: make(chan struct{}, 1),
		// timeouts:   make(map[uint32]*Timer),
		// intervals:  make(map[uint32]*Interval),
	}
	return
}

func (loop *EventLoop) JobCount() int32 {
	return loop.jobCount
}

func (loop *EventLoop) schedule(call FunctionCall, vm *Runtime, once bool) Value {
	if fn, ok := AssertFunction(call.Argument(0)); ok {
		delay := (time.Duration)(call.Argument(1).ToInteger()) * time.Millisecond
		var args []Value
		if len(call.Arguments) > 2 {
			args = call.Arguments[2:]
		}
		cb := func()(err error){
			_, err = fn(nil, args...)
			return
		}
		loop.jobCount++
		if once {
			return vm.ToValue(loop.addTimeout(cb, delay))
		}else{
			return vm.ToValue(loop.addInterval(cb, delay))
		}
	}
	return nil
}

func (loop *EventLoop) setTimeout1(call FunctionCall, vm *Runtime) Value {
	return loop.schedule(call, vm, true)
}

func (loop *EventLoop) setInterval1(call FunctionCall, vm *Runtime) Value {
	return loop.schedule(call, vm, false)
}

// SetTimeout schedules to run the specified function in the context
// of the loop as soon as possible after the specified timeout period.
// SetTimeout returns a Timer which can be passed to ClearTimeout.
// The instance of Runtime that is passed to the function and any Values derived
// from it must not be used outside the function. SetTimeout is
// safe to call inside or outside the loop.
func (loop *EventLoop) setTimeout2(fn Callback, timeout time.Duration) *Timer {
	t := loop.addTimeout(fn, timeout)
	loop.DoSync(func()(error){
		loop.jobCount++
		return nil
	})
	return t
}

// ClearTimeout cancels a Timer returned by SetTimeout if it has not run yet.
// ClearTimeout is safe to call inside or outside the loop.
func (loop *EventLoop) ClearTimeout(t *Timer) {
	loop.DoSync(func()(error){
		loop.clearTimeout(t)
		return nil
	})
}

// SetInterval schedules to repeatedly run the specified function in
// the context of the loop as soon as possible after every specified
// interval period.  SetInterval returns an Interval which can be
// passed to ClearInterval. The instance of Runtime that is passed to the
// function and any Values derived from it must not be used outside
// the function. SetInterval is safe to call inside or outside the
// loop.
func (loop *EventLoop) setInterval2(fn Callback, interval time.Duration) *Interval {
	i := loop.addInterval(fn, interval)
	loop.DoSync(func()(error){
		loop.jobCount++
		return nil
	})
	return i
}

// ClearInterval cancels an Interval returned by SetInterval.
// ClearInterval is safe to call inside or outside the loop.
func (loop *EventLoop) ClearInterval(i *Interval) {
	loop.DoSync(func()(error){
		loop.clearInterval(i)
		return nil
	})
}

func (loop *EventLoop) Running() bool {
	return atomic.LoadInt32(&loop.status) == loopRunning
}

func (loop *EventLoop) setRunning() {
	if !atomic.CompareAndSwapInt32(&loop.status, loopIdle, loopRunning) {
		panic("Loop is already started")
	}
}

// Run calls the specified function, starts the event loop and waits until there are no more delayed jobs to run
// after which it stops the loop and returns.
// The instance of Runtime that is passed to the function and any Values derived from it must not be used
// outside the function.
// Do NOT use this function while the loop is already running. Use RunOnLoop() instead.
// If the loop is already started it will panic.
func (loop *EventLoop) Run() {
	loop.setRunning()
	loop.run(false)
}

// Start the event loop in the background. The loop continues to run until Stop() is called.
// If the loop is already started it will panic.
func (loop *EventLoop) Start() {
	loop.setRunning()
	go loop.run(true)
}

// Stop the loop that was started with Start(). After this function returns there will be no more jobs executed
// by the loop. It is possible to call Start() or Run() again after this to resume the execution.
// Note, it does not cancel active timeouts.
// It is not allowed to run Start() (or Run()) and Stop() concurrently.
// Calling Stop() on a non-running loop has no effect.
func (loop *EventLoop) Stop() {
	loop.stop()
}

// Stop and wait until idle
func (loop *EventLoop) StopAndWait() {
	loop.stop()
	loop.runCond.L.Lock()
	defer loop.runCond.L.Unlock()
	loop.wait()
}

func (loop *EventLoop) stop() {
	if atomic.CompareAndSwapInt32(&loop.status, loopRunning, loopStopping) {
		loop.wakeup()
	}
}

// Wait until loop is idle
func (loop *EventLoop) Wait() {
	loop.runCond.L.Lock()
	defer loop.runCond.L.Unlock()
	loop.wait()
}

func (loop *EventLoop) wait() {
	for atomic.LoadInt32(&loop.status) != loopIdle {
		loop.runCond.Wait()
	}
}

func (loop *EventLoop) runAux() {
	loop.auxJobsLock.Lock()
	jobs := loop.auxJobs
	loop.auxJobs = loop.auxJobsSpare
	loop.auxJobsLock.Unlock()
	for i, job := range jobs {
		if err := job(); err != nil {
			if loop.Onerror != nil {
				loop.Onerror(err)
			}
		}
		jobs[i] = nil
	}
	if cap(jobs) > 128 {
		jobs = nil
	}
	loop.auxJobsSpare = jobs[:0]
}

func (loop *EventLoop) run(inBackground bool) {
	if inBackground {
		loop.jobCount++
	}

	loop.runAux()
	for atomic.LoadInt32(&loop.status) == loopRunning && loop.jobCount > 0 {
		select {
		case job := <-loop.jobChan:
			if err := job(); err != nil {
				if loop.Onerror != nil {
					loop.Onerror(err)
				}
			}
		case <-loop.wakeupFlag:
			loop.runAux()
		}
	}
	if inBackground {
		loop.jobCount--
	}

	loop.runCond.L.Lock()
	atomic.StoreInt32(&loop.status, loopIdle)
	loop.runCond.Broadcast()
	loop.runCond.L.Unlock()
}

func (loop *EventLoop) wakeup() {
	select {
	case loop.wakeupFlag <- struct{}{}:
	default:
	}
}

func (loop *EventLoop) DoSync(cb Callback) {
	loop.auxJobsLock.Lock()
	loop.auxJobs = append(loop.auxJobs, cb)
	loop.auxJobsLock.Unlock()
	loop.wakeup()
}

func (loop *EventLoop) addTimeout(cb Callback, timeout time.Duration)(t *Timer){
	if timeout <= 0 {
		timeout = 0
	}

	t = &Timer{
		job:   makeJob(cb),
		timer: time.AfterFunc(timeout, func(){
			loop.jobChan <- func()(err error){
				t.alive = false
				return cb()
			}
			loop.jobCount--
		}),
	}
	return
}

func (loop *EventLoop) addInterval(cb Callback, interval time.Duration)(i *Interval){
	// https://nodejs.org/api/timers.html#timers_setinterval_callback_delay_args
	if interval <= 0 {
		interval = time.Millisecond
	}

	ctx, cf := ctxt.WithCancel(ctxt.Background())
	i = &Interval{
		job:    makeJob(cb),
		ctx:    ctx,
		cf:     cf,
		ticker: time.NewTicker(interval),
	}
	go i.run(loop)
	return
}

func (loop *EventLoop) clearTimeout(t *Timer) {
	if t != nil && t.alive {
		t.cancel()
		loop.jobCount--
	}
}

func (loop *EventLoop) clearInterval(i *Interval) {
	if i != nil && i.alive {
		i.cancel()
		loop.jobCount--
	}
}
