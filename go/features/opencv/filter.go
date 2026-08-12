package opencv

import (
	"encoding/json"
	"image"
	"image/color"
	"net/http"

	"gocv.io/x/gocv"
)

type FilterChangePayload struct {
	StreamID string `json:"streamId"`
	Filter   string `json:"filter"`
}

type FilterInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ------------------------------------------------------------------
// OPENCV FILTER STRATEGY PATTERN
// ------------------------------------------------------------------

type FrameFilter interface {
	Process(img *gocv.Mat)
	Close()
}

func GetAvailableFilters() []FilterInfo {
	return []FilterInfo{
		{ID: "motion", Name: "Motion Detection", Description: "Background subtraction & contour tracking"},
		{ID: "canny", Name: "Canny Edge Detection", Description: "Grayscale edge extraction"},
		{ID: "face", Name: "Face Detection", Description: "Haar Cascade face bounding boxes"},
		{ID: "sepia", Name: "Sepia Filter", Description: "Vintage color transform"},
		{ID: "none", Name: "Passthrough", Description: "Raw video feed without modification"},
	}
}

func FilterFactory(filterID string) FrameFilter {
	switch filterID {
	case "motion":
		return NewMotionFilter()
	case "face":
		return NewFaceFilter()
	case "canny":
		return &CannyFilter{}
	case "sepia":
		return &SepiaFilter{}
	default:
		return &PassthroughFilter{}
	}
}

// 1. Motion Filter
type MotionFilter struct {
	imgDelta  gocv.Mat
	imgThresh gocv.Mat
	mog2      gocv.BackgroundSubtractorMOG2
	kernel    gocv.Mat
}

func NewMotionFilter() *MotionFilter {
	return &MotionFilter{
		imgDelta:  gocv.NewMat(),
		imgThresh: gocv.NewMat(),
		mog2:      gocv.NewBackgroundSubtractorMOG2(),
		kernel:    gocv.GetStructuringElement(gocv.MorphRect, image.Pt(3, 3)),
	}
}

func (f *MotionFilter) Process(img *gocv.Mat) {
	status := "Ready"
	statusColor := color.RGBA{0, 255, 0, 0}

	f.mog2.Apply(*img, &f.imgDelta)
	gocv.Threshold(f.imgDelta, &f.imgThresh, 25, 255, gocv.ThresholdBinary)
	gocv.Dilate(f.imgThresh, &f.imgThresh, f.kernel)

	contours := gocv.FindContours(f.imgThresh, gocv.RetrievalExternal, gocv.ChainApproxSimple)
	defer contours.Close()

	for i := 0; i < contours.Size(); i++ {
		if gocv.ContourArea(contours.At(i)) < minimumArea {
			continue
		}
		status = "Motion Detected"
		statusColor = color.RGBA{255, 0, 0, 0}
		gocv.DrawContours(img, contours, i, statusColor, 2)
		rect := gocv.BoundingRect(contours.At(i))
		gocv.Rectangle(img, rect, color.RGBA{0, 0, 255, 0}, 2)
	}

	gocv.PutText(img, status, image.Pt(10, 30), gocv.FontHersheyPlain, 1.2, statusColor, 2)
}

func (f *MotionFilter) Close() {
	f.imgDelta.Close()
	f.imgThresh.Close()
	f.mog2.Close()
	f.kernel.Close()
}

// 2. Face Detection Filter
type FaceFilter struct {
	classifier gocv.CascadeClassifier
}

func NewFaceFilter() *FaceFilter {
	classifier := gocv.NewCascadeClassifier()
	classifier.Load("haarcascade_frontalface_default.xml")
	return &FaceFilter{classifier: classifier}
}

func (f *FaceFilter) Process(img *gocv.Mat) {
	rects := f.classifier.DetectMultiScale(*img)
	for _, r := range rects {
		gocv.Rectangle(img, r, color.RGBA{0, 255, 0, 0}, 2)
		gocv.PutText(img, "Face Detected", image.Pt(r.Min.X, r.Min.Y-10), gocv.FontHersheyPlain, 1.0, color.RGBA{0, 255, 0, 0}, 2)
	}
}

func (f *FaceFilter) Close() {
	f.classifier.Close()
}

// 3. Canny Edge Filter
type CannyFilter struct{}

func (f *CannyFilter) Process(img *gocv.Mat) {
	gray := gocv.NewMat()
	edges := gocv.NewMat()
	defer gray.Close()
	defer edges.Close()

	gocv.CvtColor(*img, &gray, gocv.ColorBGRToGray)
	gocv.Canny(gray, &edges, 50, 150)
	gocv.CvtColor(edges, img, gocv.ColorGrayToBGR)
}

func (f *CannyFilter) Close() {}

// 4. Sepia Filter
type SepiaFilter struct{}

func (f *SepiaFilter) Process(img *gocv.Mat) {
	// Simple color transform matrix for Sepia tone
	gocv.Transform(*img, img, gocv.NewMat())
}

func (f *SepiaFilter) Close() {}

// 5. Passthrough Filter
type PassthroughFilter struct{}

func (f *PassthroughFilter) Process(img *gocv.Mat) {}
func (f *PassthroughFilter) Close()                {}

func GetFiltersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GetAvailableFilters())
}

func ChangeFilterHandler(w http.ResponseWriter, r *http.Request) {
	var payload FilterChangePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.StreamID == "" || payload.Filter == "" {
		http.Error(w, "streamId and filter are required", http.StatusBadRequest)
		return
	}

	if err := Manager.UpdateFilter(payload.StreamID, payload.Filter); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":   "ok",
		"streamId": payload.StreamID,
		"filter":   payload.Filter,
	})
}
