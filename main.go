/*
NAME
	vis3 - visualize 3D polygons

SYNOPSIS
	vis3 file
	[ prog ] | vis3

DESCRIPTION
	Vis3 creates a window and then reads rendering commands from
	standard input, or from a file if a filename is specified in
	the first program argument. The view can be rotated using the
	mouse. The coordinate space is -1.0 to 1.0 in each axis.

COMMANDS
	Commands are used to build up a scene, which persists until a
	clearscene command is read.

	Commands must be terminated with a semicolon, and a command may
	span multiple lines.

	The commands are:

	clearscene
		removes all rendering commands from the scene
	color R G B A
		sets a new rendering color; components must be in range
		0.0 to 1.0
	bgcolor R G B A
		sets a new background color; components must be in range
		0.0 to 1.0
	thickness THICKNESS
		sets a new thickness for lines, in pixels
	pointsize SIZE
		sets a new size for points, in pixels
	point X Y Z
		places a point in the scene
	line AX AY AZ  BX BY BZ
		places the line AB in the scene
	poly X Y Z [X Y Z ...]
		places in the scene a convex polygon with any number
		of points
*/

//go:generate goyacc vis3.y
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/gl/v2.1/gl"
	. "github.com/go-gl/mathgl/mgl32"
	"github.com/veandco/go-sdl2/sdl"
)

var lock sync.Mutex
var cmds []cmd
var viewPos Vec3
var viewAngles Vec3
var color Vec4
var wireColor = Vec4{1, 1, 1, 0.75}
var clearColor Vec4
var pointSize float32
var thickness float32
var timeDelta time.Duration
var dirty = make(chan int, 1)

func init() {
	runtime.LockOSThread()
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	yyErrorVerbose = true
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		go yyParse(newLexer(f))
	} else {
		go yyParse(newLexer(os.Stdin))
	}

	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		log.Fatal(err)
	}
	defer sdl.Quit()

	sdl.SetRelativeMouseMode(true)

	win, err := sdl.CreateWindow("vis3", sdl.WINDOWPOS_CENTERED,
		sdl.WINDOWPOS_CENTERED, 600, 600, sdl.WINDOW_OPENGL)
	if err != nil {
		log.Fatal(err)
	}
	defer win.Destroy()

	ctx, err := sdl.GL_CreateContext(win)
	if err != nil {
		log.Fatal(err)
	}
	defer sdl.GL_DeleteContext(ctx)
	if err := gl.Init(); err != nil {
		log.Fatal(err)
	}

	setupGL()
	clearScene()
	needRefresh()

	t := time.Now()
	now, then := t, t
	lastStatTime := t
	statFrames := 0
	running := true
	for running {
		now = time.Now()
		timeDelta = now.Sub(then)
		then = now

		statFrames++

		interval := now.Sub(lastStatTime)
		if interval >= 500*time.Millisecond {
			fps := float64(statFrames) / interval.Seconds()
			title := fmt.Sprintf("vis3 - %d fps", int(fps))
			win.SetTitle(title)
			statFrames = 0
			lastStatTime = now
		}

		for ev := sdl.PollEvent(); ev != nil; ev = sdl.PollEvent() {
			switch t := ev.(type) {
			case *sdl.QuitEvent:
				running = false
			case *sdl.MouseMotionEvent:
				viewAngles[0] += float32(t.XRel) / 4
				viewAngles[1] += float32(t.YRel) / 4
				viewAngles[1] = float32(math.Min(float64(viewAngles[1]), 89.9999))
				viewAngles[1] = float32(math.Max(float64(viewAngles[1]), -89.9999))
				needRefresh()
			case *sdl.KeyDownEvent:
				switch {
				case t.Keysym.Sym == sdl.K_F4 && t.Keysym.Mod&sdl.KMOD_ALT != 0:
					fallthrough
				case t.Keysym.Sym == sdl.K_ESCAPE:
					running = false
				default:
					needRefresh()
				}
			}
		}

		select {
		case <-dirty:
			refresh(win)
		default:
		}
		for time.Now().Sub(then).Seconds() < 1/100. {
			time.Sleep(time.Millisecond)
		}
	}
}

func clearScene() {
	lock.Lock()
	defer lock.Unlock()
	cmds = make([]cmd, 0, 1024)

	addCmd("bgcolor", []float32{0, 0, 0, 1})
	addCmd("pointsize", []float32{6})

	addCmd("thickness", []float32{2})
	addCmd("color", []float32{1, 0, 0, 1})
	addCmd("line", []float32{1, 0, 0, 0, 0, 0})
	addCmd("color", []float32{0, 1, 0, 1})
	addCmd("line", []float32{0, 1, 0, 0, 0, 0})
	addCmd("color", []float32{0, 0, 1, 1})
	addCmd("line", []float32{0, 0, 1, 0, 0, 0})

	addCmd("thickness", []float32{1})
	addCmd("color", []float32{1, 1, 1, 1})
}

func needRefresh() {
	select {
	case dirty <- 1:
	default:
	}
}

func setupGL() {
	gl.Hint(gl.LINE_SMOOTH_HINT, gl.NICEST)
	gl.Enable(gl.LINE_SMOOTH)

	gl.Disable(gl.CULL_FACE)

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LEQUAL)

	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	gl.MatrixMode(gl.PROJECTION)
	gl.LoadIdentity()
	gl.Ortho(-1, 1, -1, 1, -100, 100)

	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
}

func refresh(win *sdl.Window) {
	gl.MatrixMode(gl.MODELVIEW)
	gl.LoadIdentity()
	gl.Rotatef(viewAngles[1], 1.0, 0.0, 0.0)
	gl.Rotatef(viewAngles[0], 0.0, 1.0, 0.0)
	gl.Translatef(viewPos[0], viewPos[1], viewPos[2])

	gl.ClearColor(clearColor[0], clearColor[1], clearColor[2], clearColor[3])
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	lock.Lock()
	for i := 0; i < len(cmds); i++ {
		cmds[i].exec()
	}
	lock.Unlock()

	sdl.GL_SwapWindow(win)
}

func validArgs(name string, args []float32, expect int) bool {
	if len(args) != expect {
		log.Printf("%s: expected %d arguments, found %d",
			name, expect, len(args))
		return false
	}
	return true
}

func addCmd(name string, args []float32) {
	name = strings.ToLower(name)

	switch name {
	case "bgcolor":
		if !validArgs(name, args, 4) {
			break
		}
		clearColor = Vec4{args[0], args[1], args[2], args[3]}
	case "clearscene":
		clearScene()
	case "color":
		if !validArgs(name, args, 4) {
			break
		}
		v := Vec4{args[0], args[1], args[2], args[3]}
		c := &colorCmd{v}
		cmds = append(cmds, c)
	case "thickness":
		if !validArgs(name, args, 1) {
			break
		}
		c := &thicknessCmd{args[0]}
		cmds = append(cmds, c)
	case "line":
		if !validArgs(name, args, 6) {
			break
		}
		a := Vec3{args[0], args[1], args[2]}
		b := Vec3{args[3], args[4], args[5]}
		c := &lineCmd{a, b}
		cmds = append(cmds, c)
	case "pointsize":
		if !validArgs(name, args, 1) {
			break
		}
		c := &pointSizeCmd{args[0]}
		cmds = append(cmds, c)
	case "point":
		if !validArgs(name, args, 3) {
			break
		}
		v := Vec3{args[0], args[1], args[2]}
		c := &pointCmd{v}
		cmds = append(cmds, c)
	case "poly":
		if len(args) < 3 || len(args)%3 != 0 {
			log.Print("poly: expected at least 3 arguments")
			break
		}
		c := &polyCmd{make([]Vec3, 0, 4)}
		for i := 0; i < len(args); i += 3 {
			v := Vec3{args[i], args[i+1], args[i+2]}
			c.v = append(c.v, v)
		}
		cmds = append(cmds, c)
	case "plane":
		if !validArgs(name, args, 4) {
			break
		}
		norm := Vec3{args[0], args[1], args[2]}
		norm = norm.Normalize()
		c := &planeCmd{norm, args[3]}
		cmds = append(cmds, c)
	default:
		log.Printf("%s: unknown command", name)
	}

	needRefresh()
}

//
// Commands
//

type cmd interface {
	exec()
}

type colorCmd struct {
	c Vec4
}

func (c *colorCmd) exec() {
	color = c.c
}

type pointSizeCmd struct {
	sz float32
}

func (c *pointSizeCmd) exec() {
	pointSize = c.sz
}

type thicknessCmd struct {
	thickness float32
}

func (c *thicknessCmd) exec() {
	thickness = c.thickness
}

type pointCmd struct {
	v Vec3
}

func (c *pointCmd) exec() {
	gl.PointSize(pointSize)
	gl.Color4f(color[0], color[1], color[2], color[3])
	gl.Begin(gl.POINTS)
	gl.Vertex3f(c.v[0], c.v[1], c.v[2])
	gl.End()
}

type lineCmd struct {
	a, b Vec3
}

func (c *lineCmd) exec() {
	gl.LineWidth(thickness)
	gl.Color4f(color[0], color[1], color[2], color[3])
	gl.Begin(gl.LINES)
	gl.Vertex3f(c.a[0], c.a[1], c.a[2])
	gl.Vertex3f(c.b[0], c.b[1], c.b[2])
	gl.End()
}

type polyCmd struct {
	v []Vec3
}

func (c *polyCmd) exec() {
	gl.Color4f(color[0], color[1], color[2], color[3])
	if len(c.v) == 3 {
		gl.Begin(gl.TRIANGLES)
	} else if len(c.v) == 4 {
		gl.Begin(gl.QUADS)
	} else {
		gl.Begin(gl.POLYGON)
	}
	for i := 0; i < len(c.v); i++ {
		gl.Vertex3f(c.v[i][0], c.v[i][1], c.v[i][2])
	}
	gl.End()

	gl.LineWidth(thickness)
	gl.Color4f(wireColor[0], wireColor[1], wireColor[2], wireColor[3])
	gl.Begin(gl.LINE_LOOP)
	for i := 0; i < len(c.v)-1; i++ {
		gl.Vertex3f(c.v[i][0], c.v[i][1], c.v[i][2])
		gl.Vertex3f(c.v[i+1][0], c.v[i+1][1], c.v[i+1][2])
	}
	gl.End()
}

type planeCmd struct {
	norm Vec3
	dist float32
}

func (c *planeCmd) exec() {
}
