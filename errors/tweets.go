package errors

type TweetsError struct {
	err        string //error description
	statusCode int    // HTTP Code
}

func (e *TweetsError) Error() string {
	return e.err
}

func (e *TweetsError) StatusCode() int {
	return e.statusCode
}
