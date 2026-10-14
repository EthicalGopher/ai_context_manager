//go:build windows

package voice

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
)

// ffmpeg is the only recorder wired up on Windows. {device} is the
// DirectShow microphone: AICW_AUDIO_DEVICE, else the first one ffmpeg lists.
var recorders = [][]string{
	{"ffmpeg", "-loglevel", "error", "-y", "-f", "dshow", "-i", "audio={device}", "-ar", "16000", "-ac", "1", "{file}"},
}

var dshowAudio = regexp.MustCompile(`"([^"]+)" \(audio\)`)

func audioDevice() (string, error) {
	if d := os.Getenv("AICW_AUDIO_DEVICE"); d != "" {
		return d, nil
	}
	var out bytes.Buffer
	cmd := exec.Command("ffmpeg", "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	cmd.Stderr = &out // ffmpeg prints the device list on stderr and exits non-zero
	cmd.Run()
	if m := dshowAudio.FindSubmatch(out.Bytes()); m != nil {
		return string(m[1]), nil
	}
	return "", errors.New("no microphone found; set AICW_AUDIO_DEVICE to a DirectShow audio device name")
}

// interruptRecorder asks ffmpeg to finish by sending "q" on stdin (Windows
// has no SIGINT), which finalises the WAV header.
func interruptRecorder(_ *exec.Cmd, stdin io.WriteCloser) {
	stdin.Write([]byte("q"))
	stdin.Close()
}
