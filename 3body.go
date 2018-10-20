package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os/exec"
	"time"
)

const (
	greenIndex = 1
	size       = 800
	g          = 1    // gravitational constant
	m          = 1E7  // same mass for all particles
	epoch      = 1E-5 // simulation epoch
	count      = 3    // number of particles
	drawRadius = 8
)

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	http.HandleFunc("/3body", requestHandler)
	log.Fatal(http.ListenAndServe("localhost:8000", nil))
}

func requestHandler(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("ffmpeg",
		"-f", "image2pipe",
		"-pix_fmt", "yuv420p",
		"-r", "24",
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
	nBody(stdin)
}

type particle struct {
	mass                         float64
	xPosition, yPosition         float64
	xVelocity, yVelocity         float64
	xAcceleration, yAcceleration float64
}

type force struct {
	x, y float64
}

func newParticle() particle {
	return particle{
		mass:          m,
		xPosition:     rand.NormFloat64() * size / 6,
		yPosition:     rand.NormFloat64() * size / 6,
		xVelocity:     0.0,
		yVelocity:     0.0,
		xAcceleration: 0.0,
		yAcceleration: 0.0,
	}
}

func (p *particle) distanceSquared(o *particle) float64 {
	dx := p.xPosition - o.xPosition
	dy := p.yPosition - o.yPosition
	return dx*dx + dy*dy
}

func (p *particle) forceActedOnBy(o *particle) force {
	d := p.distanceSquared(o)
	if d == 0 {
		// the same particle
		return force{}
	}

	c := g * p.mass * o.mass / (d*math.Sqrt(d) + /* softening */ 1E6)
	return force{
		x: c * (o.xPosition - p.xPosition),
		y: c * (o.yPosition - p.yPosition),
	}
}

func (p *particle) totalForceActedOnBy(particles []particle) force {
	totalForce := force{}
	for _, o := range particles {
		partialForce := p.forceActedOnBy(&o)
		totalForce.x += partialForce.x
		totalForce.y += partialForce.y
	}
	return totalForce
}

func (p *particle) update(f force) particle {
	// leapfrog integration
	// https://en.wikipedia.org/wiki/Leapfrog_integration
	xVelocity := p.xVelocity + epoch*0.5*p.xAcceleration
	yVelocity := p.yVelocity + epoch*0.5*p.yAcceleration
	xPosition := p.xPosition + epoch*xVelocity
	yPosition := p.yPosition + epoch*yVelocity
	xVelocity = p.xVelocity + epoch*0.5*f.x
	yVelocity = p.yVelocity + epoch*0.5*f.y
	return particle{
		mass:          p.mass,
		xPosition:     xPosition,
		yPosition:     yPosition,
		xVelocity:     xVelocity,
		yVelocity:     yVelocity,
		xAcceleration: f.x,
		yAcceleration: f.y,
	}
}

type animator struct {
	writer              io.WriteCloser
	particleGenerations []particleGeneration
}

type particleGeneration struct {
	p particle
	c uint8
}

func (a *animator) drawCircle(img *image.Paletted, p particle, r int, c uint8) {
	for x := -r; x < r; x++ {
		for y := -r; y < r; y++ {
			if x*x+y*y < r*r {
				img.SetColorIndex(size/2+int(p.xPosition)+x, size/2+int(p.yPosition)+y, c)
			}
		}
	}
}

func (a *animator) drawParticles(particles []particle) {
	// update existing generations
	pruneIndex := -1
	for i, p := range a.particleGenerations {
		if p.c == 1 {
			pruneIndex = i
		} else {
			a.particleGenerations[i].c -= 1
		}
	}
	a.particleGenerations = a.particleGenerations[pruneIndex+1:]

	// newest generation will be black
	for _, p := range particles {
		a.particleGenerations = append(a.particleGenerations, particleGeneration{p: p, c: 8})
	}

	img := image.NewPaletted(image.Rect(0, 0, size, size),
		[]color.Color{
			color.Gray{0xff},
			color.Gray{0xdf},
			color.Gray{0xbf},
			color.Gray{0x9f},
			color.Gray{0x7f},
			color.Gray{0x5f},
			color.Gray{0x3f},
			color.Gray{0x1f},
			color.Gray{0x00},
		})
	for _, p := range a.particleGenerations {
		a.drawCircle(img, p.p, drawRadius, p.c)
	}

	jpeg.Encode(a.writer, img, nil)
}

func nBody(writer io.WriteCloser) {
	// https://en.wikipedia.org/wiki/N-body_problem
	// two dimensions only

	particles := []particle{}
	for i := 0; i < count; i++ {
		particles = append(particles, newParticle())
	}

	// leapfrog initial accelerations
	for _, p := range particles {
		totalForce := p.totalForceActedOnBy(particles)
		p.xAcceleration = totalForce.x
		p.yAcceleration = totalForce.y
	}

	a := animator{
		writer: writer,
	}
	for {
		a.drawParticles(particles)

		updated := []particle{}
		for _, p := range particles {
			totalForce := p.totalForceActedOnBy(particles)
			updated = append(updated, p.update(totalForce))
		}
		particles = updated
	}
}
