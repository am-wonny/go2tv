package live

import (
	"fmt"
	"io"
	"os"
	"time"
)

func NewLiveReader(path string) (*liveReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed create new live reader: %w", err)
	}
	return &liveReader{f: f}, nil
}

type liveReader struct {
	f      *os.File
	eofCnt int
}

func (r *liveReader) Read(p []byte) (n int, err error) {
	n, err = r.f.Read(p)
	if err != nil {
		if err == io.EOF {
			r.eofCnt++
			if r.eofCnt > 10 {
				return
			}

			time.Sleep(100 * time.Millisecond)

			p2 := make([]byte, len(p)-n)
			n2, err := r.Read(p2)
			if err != nil {
				return n + n2, err
			}

			p = append(p[:n], p2...)
			return n + n2, nil
		}
		return
	}
	r.eofCnt = 0
	return
}

func (r *liveReader) Close() error {
	return r.f.Close()
}
