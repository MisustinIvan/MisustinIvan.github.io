package main

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"syscall/js"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

// bufferWriteSeeker implements io.WriteSeeker for a memory buffer,
// allowing the WAV encoder to update headers.
type bufferWriteSeeker struct {
	buf []byte
	off int
}

func (bws *bufferWriteSeeker) Write(p []byte) (n int, err error) {
	end := bws.off + len(p)
	if end > len(bws.buf) {
		newBuf := make([]byte, end)
		copy(newBuf, bws.buf)
		bws.buf = newBuf
	}
	copy(bws.buf[bws.off:], p)
	bws.off = end
	return len(p), nil
}

func (bws *bufferWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case 0: // SeekStart
		newOffset = offset
	case 1: // SeekCurrent
		newOffset = int64(bws.off) + offset
	case 2: // SeekEnd
		newOffset = int64(len(bws.buf)) + offset
	}
	if newOffset < 0 {
		newOffset = 0
	}
	bws.off = int(newOffset)
	return newOffset, nil
}

func (bws *bufferWriteSeeker) Bytes() []byte {
	return bws.buf
}

func generate(this js.Value, inputs []js.Value) interface{} {
	if len(inputs) < 5 {
		return "Missing arguments: expected (data, minFreq, maxFreq, sampleRate, secondsPerColumn)"
	}
	inputJS := inputs[0]
	length := inputJS.Get("length").Int()
	imgData := make([]byte, length)
	js.CopyBytesToGo(imgData, inputJS)

	minFreq := inputs[1].Float()
	maxFreq := inputs[2].Float()
	sampleRate := inputs[3].Int()
	secondsPerStep := inputs[4].Float()

	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "Decode error: " + err.Error()
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	totalSamples := int(float64(h) * secondsPerStep * float64(sampleRate))
	data := make([]int, totalSamples)

	sampleIdx := 0

	// For SDR waterfalls (scrolling down):
	// To see the image upright, we play the bottom rows first.
	// Horizontal (X) maps to Frequency.
	// Vertical (Y) maps to Time.
	for y := h - 1; y >= 0; y-- {
		numSamplesInStep := int(secondsPerStep * float64(sampleRate))

		type tone struct {
			freq float64
			amp  float64
		}
		var tones []tone
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			brightness := float64(r+g+b) / (3.0 * 65535.0)
			if brightness > 0.01 {
				// Map X to frequency range (left to right)
				freq := minFreq + (float64(x)/float64(w))*(maxFreq-minFreq)
				tones = append(tones, tone{freq, brightness})
			}
		}

		for i := 0; i < numSamplesInStep; i++ {
			var mixedSample float64
			t := float64(sampleIdx) / float64(sampleRate)

			for _, tn := range tones {
				mixedSample += tn.amp * math.Sin(2*math.Pi*tn.freq*t)
			}
			// Normalize by width as we mix 'w' possible frequencies
			data[sampleIdx] = int(mixedSample / float64(w) * 32767.0)
			sampleIdx++
			if sampleIdx >= totalSamples {
				break
			}
		}
		if sampleIdx >= totalSamples {
			break
		}
	}

	ws := &bufferWriteSeeker{}
	enc := wav.NewEncoder(ws, sampleRate, 16, 1, 1)

	audioBuffer := &audio.IntBuffer{
		Data:   data,
		Format: &audio.Format{NumChannels: 1, SampleRate: sampleRate},
	}

	if err := enc.Write(audioBuffer); err != nil {
		return "Write error: " + err.Error()
	}
	enc.Close()

	outputBytes := ws.Bytes()
	dst := js.Global().Get("Uint8Array").New(len(outputBytes))
	js.CopyBytesToJS(dst, outputBytes)
	return dst
}

func main() {
	c := make(chan struct{}, 0)
	js.Global().Set("generate", js.FuncOf(generate))
	<-c
}
