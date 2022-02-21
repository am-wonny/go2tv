package live

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

func NewReceiverSimple() (*liveReceiverSimple, error) {
	f, _ := os.Create("__live/live_http.mp4")
	return &liveReceiverSimple{
		buf: []byte{},
		mux: sync.Mutex{},
		f:   f,
	}, nil
}

type liveReceiverSimple struct {
	buf []byte
	mux sync.Mutex
	cnt int
	f   *os.File
}

func (r *liveReceiverSimple) Write(p []byte) (n int, err error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.buf = append(r.buf, p...)
	if r.f != nil {
		return r.f.Write(p)
	}
	return len(p), nil
}

func (r *liveReceiverSimple) Read(p []byte) (n int, err error) {
	r.mux.Lock()
	if len(p) <= len(r.buf) {
		r.cnt = 0
		p = r.buf[:len(p)]
		r.buf = r.buf[len(p):]
		r.mux.Unlock()
		return len(p), nil
	} else {
		r.cnt++
		if r.cnt > 10 {
			n = copy(p, r.buf)
			r.buf = []byte{}
			return n, io.EOF
		}

		r.mux.Unlock()
		time.Sleep(200 * time.Millisecond)
		return r.Read(p)
	}
}

func (r *liveReceiverSimple) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.buf = []byte{}
	r.cnt = 0
	if r.f != nil {
		_ = r.f.Close()
	}
	return nil
}

func NewReceiver() (*liveReceiver, error) {
	return &liveReceiver{
		stack: [][]byte{},
		mux:   sync.RWMutex{},
	}, nil
}

type liveReceiver struct {
	buf   []byte
	stack [][]byte
	wCnt  int
	rCnt  int
	mux   sync.RWMutex
}

func (r *liveReceiver) Write(p []byte) (n int, err error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.buf = append(r.buf, p...)
	if r.wCnt == 0 {
		r.stack = append(r.stack, []byte(""))
	}
	if r.wCnt < 3 {
		r.stack[0] = append(r.stack[0], p...)
		r.wCnt++
	} else {
		r.stack = append(r.stack, p)
		//if len(r.stack) > 3 {
		//	r.stack = r.stack[len(r.stack)-3:]
		//}
	}
	return len(p), nil
}

func (r *liveReceiver) Read(p []byte) (n int, err error) {
	r.mux.Lock()
	if r.wCnt < 3 {
		r.rCnt++
		if r.rCnt > 100 {
			r.mux.Unlock()
			return 0, fmt.Errorf("failed to read")
		}
		r.mux.Unlock()
		time.Sleep(100 * time.Millisecond)
		return r.Read(p)
	}

	r.mux.Unlock()
	p, err = r.readStack(len(p))
	n = len(p)
	return
}

func (r *liveReceiver) readStack(i int) (p []byte, err error) {
	if i == 0 {
		return
	}
	r.mux.Lock()
	if len(r.stack) > 0 {
		r.rCnt = 0
		if len(r.stack[0]) <= i {
			p = r.stack[0]
			r.stack = r.stack[1:]
			var next []byte
			r.mux.Unlock()
			next, err = r.readStack(i - len(p))
			p = append(p, next...)
			return
		} else {
			p = r.stack[0][:i]
			r.stack[0] = r.stack[0][i:]
			r.mux.Unlock()
			return
		}
	} else {
		r.rCnt++
		if r.rCnt > 10 {
			r.mux.Unlock()
			return p, io.EOF
		}
		r.mux.Unlock()
		time.Sleep(100 * time.Millisecond)
		return r.readStack(i)
	}
}

func (r *liveReceiver) Close() error {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.stack = [][]byte{}
	return nil
}
