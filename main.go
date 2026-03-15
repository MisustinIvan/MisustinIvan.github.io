package main

import (
	"flag"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
)

func main() {
	imageFileName := flag.String("i", "", "the input image file path")
	outputFileName := flag.String("o", "", "the output sound file path")

	flag.Parse()

	if *imageFileName == "" || *outputFileName == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	file, err := os.Open(*imageFileName)
	if err != nil {
		log.Fatal(err)
	}

	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	bounds := img.Bounds()
	w, h := bounds.Max.X, bounds.Max.Y

	sampleRate := 44100
	secondsPerColumn := 0.1
	totalSamples := int(float64(w) * secondsPerColumn * float64(sampleRate))

	outputFile, err := os.Create(*outputFileName)
	if err != nil {
		log.Fatal(err)
	}
	enc := wav.NewEncoder(outputFile, sampleRate, 16, 1, 1)
	defer enc.Close()

	data := make([]int, totalSamples)

	minFreq, maxFreq := 0.0, 3000.0

	sampleIdx := 0
	for x := range w {
		numSamplesInCol := int(secondsPerColumn * float64(sampleRate))

		for range numSamplesInCol {
			var mixedSample float64

			for y := range h {
				r, g, b, _ := img.At(y, x).RGBA()
				brightness := float64(r+g+b) / (3.0 * 65535.0)

				freq := minFreq + (float64(h-y)/float64(h))*(maxFreq-minFreq)

				t := float64(sampleIdx) / float64(sampleRate)
				mixedSample += brightness * math.Sin(2*math.Pi*freq*t)
			}

			data[sampleIdx] = int(mixedSample / float64(h) * 32767.0)
			sampleIdx++
		}
	}

	buf := &audio.IntBuffer{Data: data, Format: &audio.Format{NumChannels: 1, SampleRate: sampleRate}}
	enc.Write(buf)
}
