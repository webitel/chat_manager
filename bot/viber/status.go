package viber

import "fmt"

type Status struct {
	Code    int    `json:"status,omitempty"`
	Message string `json:"status_message"`
}

func (e *Status) Ok() bool {
	return e.Code == 0
}

func (e *Status) Err() error {
	if e.Ok() {
		return nil
	}
	return (*Error)(e)
}

type Error Status

func (e *Error) IsCode(code int) bool {
	return code != 0 && e != nil && code == e.Code
}

func (e *Error) Error() string {
	return fmt.Sprintf("viber: (%d) %s", e.Code, e.Message)
}

func (e *Error) Status() *Status {
	return (*Status)(e)
}
