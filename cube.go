package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const windowWidth = 800
const windowHeight = 600

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func init() {
	// This is needed to arrange that main() runs on main thread.
	// See documentation for functions that are only allowed to be called from the main thread.
	runtime.LockOSThread()
}

func main() {
	// initialize glfw
	if err := glfw.Init(); err != nil {
		log.Fatalln("Failed to initialize GLFW:", err)
	}
	defer glfw.Terminate()

	// set up window
	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 3)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	window, err := glfw.CreateWindow(windowWidth, windowHeight, "Cube", nil, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent()

	// Initialize opengl (glow)
	if err := gl.Init(); err != nil {
		panic(err)
	}

	// enable depth testing otherwise it looks
	// like shit
	gl.Enable(gl.DEPTH_TEST)

	// debug
	// version := gl.GoStr(gl.GetString(gl.VERSION))
	// fmt.Println("OpenGL version", version)

	// load vertex and frag shaders
	program, err := shaderProgFromFile("shader.vert", "shader.frag")
	if err != nil {
		panic(err)
	}

	// set program to be used
	gl.UseProgram(program)

	// create vertex array object and index buffers
	var vao uint32
	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)

	var vbo uint32
	gl.GenBuffers(1, &vbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(cubeVertices)*4, gl.Ptr(cubeVertices), gl.STATIC_DRAW)

	// position attribute
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	// texture coord attribute
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)

	// set uniforms for textures 1 and 2
	textureUniform1 := gl.GetUniformLocation(program, gl.Str("texture1\x00"))
	gl.Uniform1i(textureUniform1, 0)

	textureUniform2 := gl.GetUniformLocation(program, gl.Str("texture2\x00"))
	gl.Uniform1i(textureUniform2, 1)

	// load textures
	texture1, err := loadTexture("container.jpg")
	if err != nil {
		log.Fatalln(err)
	}

	texture2, err := loadTexture("awesomeface.png")
	if err != nil {
		log.Fatalln(err)
	}

	// bit different to C++ version, but never mind
	angle := 0.0
	previousTime := glfw.GetTime()

	for !window.ShouldClose() {
		gl.ClearColor(0.2, 0.3, 0.3, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

		// update
		time := glfw.GetTime()
		deltaTime := time - previousTime
		previousTime = time

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, texture1)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, texture2)

		gl.UseProgram(program)

		angle += deltaTime
		// These are a little bit different from
		// glm, but close enough that I can easily
		// translate...
		model := mgl32.HomogRotate3D(float32(angle), mgl32.Vec3{0.5, 1.0, 0.0})
		view := mgl32.Translate3D(0.0, 0.0, -3.0)
		projection := mgl32.Perspective(mgl32.DegToRad(45.0), float32(windowWidth)/windowHeight, 0.1, 100)

		// retrieve matrix uniform location
		modelLoc := gl.GetUniformLocation(program, gl.Str("model\x00"))
		viewLoc := gl.GetUniformLocation(program, gl.Str("view\x00"))
		// pass them to shaders
		// matrix layout is a bit different as well, but again
		// close enough to be able to make an educated guess...
		gl.UniformMatrix4fv(modelLoc, 1, false, &model[0])
		gl.UniformMatrix4fv(viewLoc, 1, false, &view[0])
		// get uniform for projection
		projLoc := gl.GetUniformLocation(program, gl.Str("projection\x00"))
		gl.UniformMatrix4fv(projLoc, 1, false, &projection[0])
		// bind and draw
		gl.BindVertexArray(vao)
		gl.DrawArrays(gl.TRIANGLES, 0, 6*2*3)
		window.SwapBuffers()
		glfw.PollEvents()
	}
}

func shaderProgFromFile(vertShaderPath, fragShaderPath string) (uint32, error) {
	// read vert shader from file raw
	vertSourceRaw, err := ioutil.ReadFile(vertShaderPath)
	if err != nil {
		log.Fatal(err)
	}

	// and turn them back into strings? std::string oh
	// how I miss thee
	vertSource := string(vertSourceRaw)

	// do the same for frag shader as above
	fragSourceRaw, err := ioutil.ReadFile(fragShaderPath)
	if err != nil {
		log.Fatal(err)
	}

	fragSource := string(fragSourceRaw)

	// compile vert and frag shader
	fragShader, err := compileShader(vertSource, gl.VERTEX_SHADER)

	if err != nil {
		return 0, err
	}

	vertShader, err := compileShader(fragSource, gl.FRAGMENT_SHADER)

	if err != nil {
		return 0, err
	}

	// create the program, attach shaders
	// and link
	program := gl.CreateProgram()

	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)

	// check status for errors
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)

	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertShader)
	gl.DeleteShader(fragShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	// compiles shader
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

func loadTexture(file string) (uint32, error) {
	// does what it says on the tin
	// reads from same directory... probably should
	// make it a bit more versatile but meh
	imgFile, err := os.Open(file)
	if err != nil {
		return 0, fmt.Errorf("Texture %q not found: %v", file, err)
	}

	// I actually really like this compared
	// with C++, don't need to use SOIL or
	// stb_image to load things - just use image!
	img, _, err := image.Decode(imgFile)
	if err != nil {
		return 0, err
	}

	rgba := image.NewRGBA(img.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return 0, fmt.Errorf("Unsupported stride")
	}
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	var texture uint32
	// generate and bind texture...
	// image rocks, gives really easy
	// access to the file's data!
	gl.GenTextures(1, &texture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))

	return texture, nil
}

var cubeVertices = []float32{
	// x   y     z     u     v
	-0.5, -0.5, -0.5, 0.0, 0.0,
	0.5, -0.5, -0.5, 1.0, 0.0,
	0.5, 0.5, -0.5, 1.0, 1.0,
	0.5, 0.5, -0.5, 1.0, 1.0,
	-0.5, 0.5, -0.5, 0.0, 1.0,
	-0.5, -0.5, -0.5, 0.0, 0.0,

	-0.5, -0.5, 0.5, 0.0, 0.0,
	0.5, -0.5, 0.5, 1.0, 0.0,
	0.5, 0.5, 0.5, 1.0, 1.0,
	0.5, 0.5, 0.5, 1.0, 1.0,
	-0.5, 0.5, 0.5, 0.0, 1.0,
	-0.5, -0.5, 0.5, 0.0, 0.0,

	-0.5, 0.5, 0.5, 1.0, 0.0,
	-0.5, 0.5, -0.5, 1.0, 1.0,
	-0.5, -0.5, -0.5, 0.0, 1.0,
	-0.5, -0.5, -0.5, 0.0, 1.0,
	-0.5, -0.5, 0.5, 0.0, 0.0,
	-0.5, 0.5, 0.5, 1.0, 0.0,

	0.5, 0.5, 0.5, 1.0, 0.0,
	0.5, 0.5, -0.5, 1.0, 1.0,
	0.5, -0.5, -0.5, 0.0, 1.0,
	0.5, -0.5, -0.5, 0.0, 1.0,
	0.5, -0.5, 0.5, 0.0, 0.0,
	0.5, 0.5, 0.5, 1.0, 0.0,

	-0.5, -0.5, -0.5, 0.0, 1.0,
	0.5, -0.5, -0.5, 1.0, 1.0,
	0.5, -0.5, 0.5, 1.0, 0.0,
	0.5, -0.5, 0.5, 1.0, 0.0,
	-0.5, -0.5, 0.5, 0.0, 0.0,
	-0.5, -0.5, -0.5, 0.0, 1.0,

	-0.5, 0.5, -0.5, 0.0, 1.0,
	0.5, 0.5, -0.5, 1.0, 1.0,
	0.5, 0.5, 0.5, 1.0, 0.0,
	0.5, 0.5, 0.5, 1.0, 0.0,
	-0.5, 0.5, 0.5, 0.0, 0.0,
	-0.5, 0.5, -0.5, 0.0, 1.0,
}
