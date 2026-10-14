//go:build !windows

package voice

import (
	"io"
	"os/exec"
	"syscall"
)

var recorders = [][]string{
	{"pw-record", "--rate", "16000", "--channels", "1", "--format", "s16", "{file}"},
	{"parecord", "--file-format=wav", "--format=s16le", "--rate=16000", "--channels=1", "{file}"},
	{"arecord", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", "{file}"},
	{"ffmpeg", "-loglevel", "error", "-y", "-f", "pulse", "-i", "default", "-ar", "16000", "-ac", "1", "{file}"},
}

func audioDevice() (string, error) { return "", nil }

// interruptRecorder sends SIGINT so the recorder finalises the WAV header.
func interruptRecorder(cmd *exec.Cmd, _ io.WriteCloser) { cmd.Process.Signal(syscall.SIGINT) }
