// SPDX-License-Identifier: Apache-2.0

//go:build windows

package editor

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"snippingtoolenhanced/internal/clipboard"
	"snippingtoolenhanced/internal/gifenc"
	"snippingtoolenhanced/internal/mf"
	"snippingtoolenhanced/internal/output"
	"snippingtoolenhanced/internal/win"
)

// VideoInfo is sent to the page for a recording.
type VideoInfo struct {
	URL      string  `json:"url"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Duration float64 `json:"duration"` // seconds (0 if unknown; the page also reads it)
	Unsaved  bool    `json:"unsaved"`  // only in the working folder: ask to save before closing
}

// mediaServer serves recordings to the page over 127.0.0.1 with a random
// token, so the <video> element can stream and seek large files.
type mediaServer struct {
	once  sync.Once
	base  string
	token string
	mu    sync.Mutex
	files map[string]string
}

var media = &mediaServer{files: map[string]string{}}

func (m *mediaServer) url(path string) (string, error) {
	var err error
	m.once.Do(func() {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		m.token = hex.EncodeToString(b)
		var ln net.Listener
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return
		}
		m.base = "http://" + ln.Addr().String() + "/" + m.token + "/"
		go http.Serve(ln, http.HandlerFunc(m.serve))
	})
	if err != nil || m.base == "" {
		return "", errors.New("could not start the media server")
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	m.mu.Lock()
	m.files[id] = path
	m.mu.Unlock()
	return m.base + id + ".mp4", nil
}

func (m *mediaServer) serve(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) != 2 || parts[0] != m.token {
		http.NotFound(w, r)
		return
	}
	m.mu.Lock()
	p, ok := m.files[strings.TrimSuffix(parts[1], ".mp4")]
	m.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, _ := f.Stat()
	w.Header().Set("Access-Control-Allow-Origin", "*") // lets the page read frames for GIF export
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, "", st.ModTime(), f)
}

func isVideo(path string) bool { return strings.EqualFold(filepath.Ext(path), ".mp4") }

func loadVideo(path string) (*VideoInfo, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	u, err := media.url(path)
	if err != nil {
		return nil, err
	}
	return &VideoInfo{URL: u, Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), Path: path,
		Unsaved: output.IsUnsavedRecording(path)}, nil
}

// saveVideo copies or trims the recording into a file the user picks.
// shareVideoFile returns the file to share: the recording itself, or its
// trimmed part written to a temporary file.
func shareVideoFile(src string, start, end, duration float64) (string, error) {
	if start <= 0.05 && !(duration > 0 && end < duration-0.05) {
		return src, nil
	}
	dir := filepath.Join(output.DataDir(), "Share")
	go output.Cleanup(dir, 7*24*time.Hour)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)) + " (trimmed).mp4"
	p := filepath.Join(dir, name)
	shutdown, err := mf.Startup()
	if err != nil {
		return "", err
	}
	defer shutdown()
	tmp := p + ".partial.mp4"
	if err := mf.Trim(src, tmp, int64(start*1e7), int64(end*1e7)); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("trim: %w", err)
	}
	_ = os.Remove(p)
	return p, os.Rename(tmp, p)
}

func (e *editor) saveVideo(src string, start, end, duration float64) (string, error) {
	dir := filepath.Dir(src)
	if output.IsUnsavedRecording(src) {
		dir = e.s.RecordingsFolder
		if strings.TrimSpace(dir) == "" {
			dir = output.DefaultRecordingsFolder()
		}
		_ = os.MkdirAll(dir, 0o755)
	}
	name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	trimmed := start > 0.05 || (duration > 0 && end < duration-0.05)
	if trimmed {
		name += " (trimmed)"
	}
	p, _ := win.SaveFileDialog(e.hwnd, "Save as", dir, name, "MP4 video (*.mp4)\x00*.mp4\x00", "mp4")
	if p == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(p), ".mp4") {
		p += ".mp4"
	}
	if strings.EqualFold(filepath.Clean(p), filepath.Clean(src)) && !trimmed {
		return p, nil
	}
	if !trimmed {
		return p, copyFile(src, p)
	}
	shutdown, err := mf.Startup()
	if err != nil {
		return "", err
	}
	defer shutdown()
	tmp := p + ".partial.mp4"
	if err := mf.Trim(src, tmp, int64(start*1e7), int64(end*1e7)); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("trim: %w", err)
	}
	_ = os.Remove(p)
	return p, os.Rename(tmp, p)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst + ".partial")
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(dst+".partial", dst)
}

// ---------- GIF export ----------

type gifJob struct {
	enc *gifenc.Encoder
}

var (
	gifMu   sync.Mutex
	gifJobs = map[string]*gifJob{}
)

func gifBegin(quality string) string {
	q := gifenc.Low
	if quality == "high" {
		q = gifenc.High
	}
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	gifMu.Lock()
	gifJobs[id] = &gifJob{enc: gifenc.New(q)}
	gifMu.Unlock()
	return id
}

func gifFrame(id, dataURL string) error {
	gifMu.Lock()
	j := gifJobs[id]
	gifMu.Unlock()
	if j == nil {
		return errors.New("no GIF export in progress")
	}
	img, err := decodeDataURL(dataURL)
	if err != nil {
		return err
	}
	j.enc.Add(img)
	return nil
}

func gifEnd(id string) ([]byte, error) {
	gifMu.Lock()
	j := gifJobs[id]
	delete(gifJobs, id)
	gifMu.Unlock()
	if j == nil || j.enc.Len() == 0 {
		return nil, errors.New("no frames")
	}
	return j.enc.Bytes()
}

func (e *editor) gifFinish(id, action, name string) (string, error) {
	data, err := gifEnd(id)
	if err != nil {
		return "", err
	}
	if action == "copy" {
		p := filepath.Join(output.CapturesDir(), sanitize(name)+".gif")
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return "", err
		}
		return "", clipboard.SetGIF(p, data)
	}
	dir := output.DefaultRecordingsFolder()
	_ = os.MkdirAll(dir, 0o755)
	p, _ := win.SaveFileDialog(e.hwnd, "Export GIF", dir, sanitize(name), "GIF (*.gif)\x00*.gif\x00", "gif")
	if p == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(p), ".gif") {
		p += ".gif"
	}
	return p, os.WriteFile(p, data, 0o644)
}

func sanitize(s string) string {
	if s == "" {
		s = "Screen Recording " + time.Now().Format("2006-01-02 150405")
	}
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}

var _ = bytes.MinRead
var _ image.Image
