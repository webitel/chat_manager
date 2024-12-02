package helper

import "io"

type ReadCloser struct {
	rc io.ReadCloser
}

func (c ReadCloser) Close() error {
	return c.rc.Close()
}

func (c ReadCloser) Read(p []byte) (n int, err error) {
	n, err = c.rc.Read(p)
	if err == io.EOF {
		_ = c.Close()
	}
	return n, err
}
