package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os/exec"
)

func main() {
	http.HandleFunc("/lissajous", requestHandler)
	http.HandleFunc("/3body", requestHandler)
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("ffmpeg",
		"-f", "image2pipe",
		"-pix_fmt", "yuv420p",
		"-r", "16",
		"-i", "-",
		"-f", "ogg",
		"-qscale:v", "10",
		"-f", "ogg", "-")
	cmd.Stdout = w
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	path := r.URL.Path
	if path == "/lissajous" {
		lissajous(stdin)
	} else if path == "/3body" {
		nBody(stdin)
	} else {
		fmt.Fprintf(w, "nope")
	}
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

func nBody(writer io.WriteCloser) {
	// https://en.wikipedia.org/wiki/N-body_problem
	// two dimensions only

	const (
		greenIndex = 1
		size       = 800
		g          = 1E-1 // gravitational constant
		m          = 1.0  // same mass for all particles
		count      = 3    // number of particles
		epoch      = 5    // simulation epoch
		drawRadius = 8
	)

	type particle struct {
		mass                 float64
		xPosition, yPosition float64
		xVelocity, yVelocity float64
	}

	distanceFn := func(p1, p2 *particle) float64 {
		// euclidean
		return math.Sqrt(math.Pow(p1.xPosition-p2.xPosition, 2) + math.Pow(p1.yPosition-p2.yPosition, 2))
	}

	newParticle := func() particle {
		return particle{
			mass:      m,
			xPosition: rand.NormFloat64() * size / 6,
			yPosition: rand.NormFloat64() * size / 6,
			xVelocity: 0.0,
			yVelocity: 0.0,
		}
	}

	type force struct {
		x, y float64
	}

	forceFn := func(p1, p2 *particle) force {
		d := distanceFn(p1, p2)
		if 0.0 == d {
			return force{}
		}

		c := g * p1.mass * p2.mass / math.Pow(d, 2)
		return force{
			x: c * (p2.xPosition - p1.xPosition),
			y: c * (p2.yPosition - p1.yPosition),
		}
	}

	updateFn := func(p particle, f force) particle {
		return particle{
			mass:      p.mass,
			xPosition: p.xPosition + epoch*p.xVelocity,
			yPosition: p.yPosition + epoch*p.yVelocity,
			xVelocity: p.xVelocity + epoch*f.x/p.mass, // f = ma -> a = f/m
			yVelocity: p.yVelocity + epoch*f.y/p.mass, // f = ma -> a = f/m
		}
	}

	drawCircle := func(m *image.Paletted, p particle, r int, c uint8) {
		for x := -r; x < r; x++ {
			for y := -r; y < r; y++ {
				if x*x+y*y < r*r {
					m.SetColorIndex(size/2+int(p.xPosition)+x, size/2+int(p.yPosition)+y, c)
				}
			}
		}
	}

	particles := []particle{}
	for i := 0; i < count; i++ {
		particles = append(particles, newParticle())
	}

	for {
		rect := image.Rect(0, 0, size+1, size+1)
		img := image.NewPaletted(rect,
			[]color.Color{
				color.Black,
				color.RGBA{0x00, 0xFF, 0x00, 0xFF}, // green
			})

		updated := []particle{}
		for _, p1 := range particles {
			drawCircle(img, p1, drawRadius, greenIndex)

			totalForce := force{}
			for _, p2 := range particles {
				partialForce := forceFn(&p1, &p2)
				totalForce.x += partialForce.x
				totalForce.y += partialForce.y
			}
			updated = append(updated, updateFn(p1, totalForce))
		}

		png.Encode(writer, img)
		particles = updated
	}
}
