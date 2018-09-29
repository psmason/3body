package main

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("ffmpeg",
		"-f", "image2pipe",
		"-pix_fmt", "yuv420p",
		"-r", "8",
		"-i", "-",
		"-f", "ogg",
		"-qscale:v", "10",
		"-f", "ogg", "-")
	cmd.Stdout = os.Stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	lissajous(stdin)
}

func lissajous(writer io.WriteCloser) {
	const (
		blackIndex = 0
		greenIndex = 1
		redIndex   = 2
		blueIndex  = 3
		res        = 0.001
		size       = 400
		nframes    = 6400
		delay      = 8
		cycles     = 5.0
	)

	freq := rand.Float64() * 3.0
	phase := 0.0
	for i := 0; ; i++ {
		rect := image.Rect(0, 0, 2*size+1, 2*size+1)
		img := image.NewPaletted(rect,
			[]color.Color{
				color.Black,
				color.RGBA{0x00, 0xFF, 0x00, 0xFF}, // green
			})
		for t := 0.0; t < cycles*2*math.Pi; t += res {
			x := math.Sin(t)
			y := math.Sin(t*freq + phase)
			img.SetColorIndex(size+int(x*size+0.5), size+int(y*size*0.5), greenIndex)
		}

		png.Encode(writer, img)
		phase += 0.01
	}
}
