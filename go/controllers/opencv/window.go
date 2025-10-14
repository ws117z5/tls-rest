package opencv

import "gocv.io/x/gocv"

var Windows []*gocv.Window

//Init
func init() {
	Windows = make([]*gocv.Window, 1)
	Windows[0] = gocv.NewWindow("Open CV")
}

func GetWindow() *gocv.Window {
	return Windows[0]
}

func Destroy() {
	for _, window := range Windows {
		window.Close()
	}
}
