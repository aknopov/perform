package main

type HashRequest struct {
	Password string `json:"password"`
	Strength int    `json:"strength"`
}

type HashResponse struct {
	Hash string `json:"hash"`
}

const (
	Port = 8080
)

func main() {
	startGin(Port)
}

func assertNoErr(err error) {
	if err != nil {
		panic(err)
	}
}
