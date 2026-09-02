package httpbin

type Request struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Response struct {
	Data string `json:"data"`
}
