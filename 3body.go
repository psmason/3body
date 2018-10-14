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
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
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
	writer io.WriteCloser
}

func (a *animator) addLabel(img *image.Paletted, x, y int, label string) {
	col := image.NewPaletted(image.Rect(0, 0, 200, 100), []color.Color{
		color.Black,
	})
	point := fixed.Point26_6{fixed.Int26_6(x * 64), fixed.Int26_6(y * 64)}

	d := &font.Drawer{
		Dst:  img,
		Src:  col,
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
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
	img := image.NewPaletted(image.Rect(0, 0, size+1, size+1),
		[]color.Color{
			color.RGBA{0xff, 0xff, 0xff, 0xff},
			color.RGBA{0x00, 0x00, 0x00, 0x4f},
			color.RGBA{0x00, 0x00, 0x00, 0x01},
		})
	for _, p := range particles {
		a.drawCircle(img, p, drawRadius, 2)
	}

	// debugging information
	if count == 2 {
		forceLabel := fmt.Sprintf("accelerations %f::%f   %f::%f",
			particles[0].xAcceleration, particles[0].yAcceleration,
			particles[1].xAcceleration, particles[1].yAcceleration)
		velocityLabel := fmt.Sprintf("velocities %f::%f   %f::%f",
			particles[0].xVelocity, particles[0].yVelocity,
			particles[1].xVelocity, particles[1].yVelocity)
		positionsLabel := fmt.Sprintf("positions %f::%f   %f::%f",
			particles[0].xPosition, particles[0].yPosition,
			particles[1].xPosition, particles[1].yPosition)
		a.addLabel(img, 0, 50, forceLabel)
		a.addLabel(img, 0, 75, velocityLabel)
		a.addLabel(img, 0, 100, positionsLabel)
	}

	png.Encode(a.writer, img)
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

	a := animator{writer: writer}
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
