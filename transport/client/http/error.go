package http

import "fmt"

type ServerError struct {
	Code  int
	Path  string
	Cause error
	Body  []byte
}

func (e *ServerError) Error() string {
	if e.Body != nil {
		return fmt.Sprintf("%v:%s:%v:%s",
			e.Code,
			e.Path,
			e.Cause,
			e.Body,
		)
	}

	return fmt.Sprintf("%v:%s:%v",
		e.Code,
		e.Path,
		e.Cause,
	)

}
