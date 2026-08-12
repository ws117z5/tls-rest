package opencv

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
)

type HardwareProfile struct {
	Name       string
	HWAccelIn  []string
	HWAccelOut []string
	Codec      string
}

func detectHardware() HardwareProfile {
	osType := runtime.GOOS

	switch osType {
	case "darwin":
		return HardwareProfile{
			Name:       "macOS VideoToolbox (Apple Silicon / Metal)",
			HWAccelIn:  []string{"-hwaccel", "videotoolbox"},
			HWAccelOut: []string{"-c:v", "libvpx", "-deadline", "realtime", "-cpu-used", "8"},
			Codec:      "vp8",
		}

	case "linux":
		if commandExists("nvidia-smi") {
			return HardwareProfile{
				Name:       "Linux NVIDIA (NVDEC / CUDA)",
				HWAccelIn:  []string{"-hwaccel", "cuda"},
				HWAccelOut: []string{"-c:v", "vp8_vaapi", "-preset", "p1"},
				Codec:      "vp8",
			}
		}

		if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
			return HardwareProfile{
				Name:       "Linux VA-API (AMD / Intel)",
				HWAccelIn:  []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128"},
				HWAccelOut: []string{"-vf", "format=nv12,hwupload", "-vaapi_device", "/dev/dri/renderD128", "-c:v", "vp8_vaapi"},
				Codec:      "vp8",
			}
		}

	case "windows":
		if commandExists("nvidia-smi") {
			return HardwareProfile{
				Name:       "Windows NVIDIA (CUDA)",
				HWAccelIn:  []string{"-hwaccel", "cuda"},
				HWAccelOut: []string{"-c:v", "libvpx", "-deadline", "realtime"},
				Codec:      "vp8",
			}
		}
	}

	return HardwareProfile{
		Name:       "CPU Software Decoding/Encoding (libvpx)",
		HWAccelIn:  []string{},
		HWAccelOut: []string{"-c:v", "libvpx", "-deadline", "realtime", "-cpu-used", "8"},
		Codec:      "vp8",
	}
}

func commandExists(cmd string) bool {
	var stdout bytes.Buffer
	c := exec.Command("which", cmd)
	if runtime.GOOS == "windows" {
		c = exec.Command("where", cmd)
	}
	c.Stdout = &stdout
	return c.Run() == nil
}
