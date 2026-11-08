package middleware

import "net/http"

type capture struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func newCapture(w http.ResponseWriter) *capture {
	return &capture{ResponseWriter: w, status: http.StatusOK}
}

func (c *capture) WriteHeader(code int) {
	if c.wrote {
		return
	}
	c.status = code
	c.wrote = true
	c.ResponseWriter.WriteHeader(code)
}

func (c *capture) Write(b []byte) (int, error) {
	if !c.wrote {
		c.status = http.StatusOK
		c.wrote = true
	}
	return c.ResponseWriter.Write(b)
}

func (c *capture) Unwrap() http.ResponseWriter {
	return c.ResponseWriter
}
