package main

func main() {
	ch := make(chan int)
	s := New("tcp", ":8080")
	s.Start()

	<-ch
}
