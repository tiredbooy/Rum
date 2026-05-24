package download

var quitFunc func() 

func SetQuitFunc(f func()) {
	quitFunc = f
}