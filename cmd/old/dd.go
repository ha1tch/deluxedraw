package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	screenWidth  = 1280
	screenHeight = 800
	fontSize     = 8
	leftPanel    = 100
	rightPanel   = 200
)

// Project structure for save/load
type ProjectData struct {
	CanvasWidth  int         `json:"canvas_width"`
	CanvasHeight int         `json:"canvas_height"`
	Layers       []LayerData `json:"layers"`
	Palette      []ColorData `json:"palette"`
}

type LayerData struct {
	Name    string  `json:"name"`
	Visible bool    `json:"visible"`
	Locked  bool    `json:"locked"`
	Opacity float32 `json:"opacity"`
}

type ColorData struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"`
}

// Layer represents a single drawing layer
type Layer struct {
	name    string
	texture rl.RenderTexture2D
	visible bool
	locked  bool
	opacity float32
}

// Tool types
type ToolType int

const (
	ToolPen ToolType = iota
	ToolBrush
	ToolEraser
	ToolBucket
	ToolEyedropper
	ToolLine
	ToolRect
	ToolCircle
	ToolMove
	ToolZoom
)

// Pen shapes
type PenShape int

const (
	PenShapeRound PenShape = iota
	PenShapeSquare
)

// GUI Control types
type Button struct {
	rect     rl.Rectangle
	text     string
	icon     rune
	pressed  bool
	hover    bool
	selected bool
}

type Slider struct {
	rect  rl.Rectangle
	value float32
	min   float32
	max   float32
	label string
}

type CheckBox struct {
	rect    rl.Rectangle
	checked bool
	label   string
}

// History action for undo/redo
type HistoryAction struct {
	actionType string
	layerIndex int
	layerData  *image.RGBA
}

// Application state
type App struct {
	// Canvas and layers
	layers       []*Layer
	activeLayer  int
	canvasWidth  int
	canvasHeight int
	layerCounter int

	// View
	zoom      float32
	panX      float32
	panY      float32
	isPanning bool
	panStartX float32
	panStartY float32

	// Tools
	currentTool  ToolType
	penSize      float32
	penShape     PenShape
	currentColor rl.Color

	// UI
	toolButtons   []Button
	colorPalette  []rl.Color
	penSizeSlider Slider
	layerButtons  []Button
	shapeButtons  []Button
	fileButtons   []Button

	// State
	isDrawing    bool
	lastMousePos rl.Vector2
	lineStart    rl.Vector2

	// Layer dragging
	isDraggingLayer bool
	draggedLayer    int
	dragOffsetY     float32

	// Render targets
	compositeTexture rl.RenderTexture2D

	// File operations
	currentFilePath string

	// Undo/Redo
	history      []HistoryAction
	historyIndex int
	maxHistory   int
}

// Create a new layer
func NewLayer(name string, width, height int) *Layer {
	texture := rl.LoadRenderTexture(int32(width), int32(height))

	// Clear to transparent
	rl.BeginTextureMode(texture)
	rl.ClearBackground(rl.Color{0, 0, 0, 0})
	rl.EndTextureMode()

	return &Layer{
		name:    name,
		texture: texture,
		visible: true,
		locked:  false,
		opacity: 1.0,
	}
}

// Initialize application
func NewApp() *App {
	app := &App{
		canvasWidth:  512,
		canvasHeight: 512,
		zoom:         1.0,
		currentTool:  ToolPen,
		penSize:      4.0,
		penShape:     PenShapeRound,
		currentColor: rl.Black,
		layerCounter: 3,
		maxHistory:   50,
		historyIndex: -1,
	}

	// Create initial layers
	app.layers = append(app.layers, NewLayer("BACKGROUND", app.canvasWidth, app.canvasHeight))
	app.layers = append(app.layers, NewLayer("LAYER 1", app.canvasWidth, app.canvasHeight))
	app.layers = append(app.layers, NewLayer("LAYER 2", app.canvasWidth, app.canvasHeight))
	app.activeLayer = 1

	// Fill background with white
	rl.BeginTextureMode(app.layers[0].texture)
	rl.ClearBackground(rl.White)
	rl.EndTextureMode()

	// Initialize composite texture
	app.compositeTexture = rl.LoadRenderTexture(int32(app.canvasWidth), int32(app.canvasHeight))

	// Initialize tool buttons
	tools := []struct {
		tool ToolType
		icon rune
		name string
	}{
		{ToolPen, 'P', "PEN"},
		{ToolBrush, 'B', "BRUSH"},
		{ToolEraser, 'E', "ERASER"},
		{ToolBucket, 'F', "FILL"},
		{ToolEyedropper, 'I', "PICKER"},
		{ToolLine, 'L', "LINE"},
		{ToolRect, 'R', "RECT"},
		{ToolCircle, 'C', "CIRCLE"},
		{ToolMove, 'M', "MOVE"},
		{ToolZoom, 'Z', "ZOOM"},
	}

	x := float32(10)
	y := float32(50)
	for i, t := range tools {
		app.toolButtons = append(app.toolButtons, Button{
			rect:     rl.Rectangle{X: x + float32(i%2)*40, Y: y + float32(i/2)*40, Width: 36, Height: 36},
			text:     string(t.icon),
			selected: t.tool == app.currentTool,
		})
	}

	// Initialize shape buttons
	app.shapeButtons = append(app.shapeButtons, Button{
		rect:     rl.Rectangle{X: 10, Y: 360, Width: 35, Height: 20},
		text:     "ROUND",
		selected: true,
	})
	app.shapeButtons = append(app.shapeButtons, Button{
		rect:     rl.Rectangle{X: 50, Y: 360, Width: 35, Height: 20},
		text:     "SQUARE",
		selected: false,
	})

	// Initialize color palette
	app.colorPalette = []rl.Color{
		rl.Black, rl.White, rl.Red, rl.Green, rl.Blue,
		rl.Yellow, rl.Orange, rl.Purple, rl.Pink, rl.Brown,
		rl.Gray, rl.DarkGray, rl.LightGray, rl.SkyBlue, rl.Magenta,
		{255, 0, 128, 255}, {128, 255, 0, 255}, {0, 128, 255, 255},
	}

	// Initialize pen size slider
	app.penSizeSlider = Slider{
		rect:  rl.Rectangle{X: 10, Y: 320, Width: 70, Height: 20},
		value: app.penSize,
		min:   1,
		max:   50,
		label: "SIZE",
	}

	// Initialize layer buttons
	app.layerButtons = []Button{
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 10), Y: float32(screenHeight - 40), Width: 35, Height: 30}, text: "NEW"},
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 50), Y: float32(screenHeight - 40), Width: 35, Height: 30}, text: "DUP"},
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 90), Y: float32(screenHeight - 40), Width: 40, Height: 30}, text: "DEL"},
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 135), Y: float32(screenHeight - 40), Width: 40, Height: 30}, text: "LOCK"},
	}

	// Initialize file buttons
	app.fileButtons = []Button{
		{rect: rl.Rectangle{X: leftPanel + 10, Y: 10, Width: 40, Height: 30}, text: "SAVE"},
		{rect: rl.Rectangle{X: leftPanel + 55, Y: 10, Width: 40, Height: 30}, text: "LOAD"},
		{rect: rl.Rectangle{X: leftPanel + 100, Y: 10, Width: 60, Height: 30}, text: "EXPORT"},
		{rect: rl.Rectangle{X: leftPanel + 165, Y: 10, Width: 40, Height: 30}, text: "UNDO"},
		{rect: rl.Rectangle{X: leftPanel + 210, Y: 10, Width: 40, Height: 30}, text: "REDO"},
	}

	return app
}

// Save layer state for undo
func (app *App) SaveLayerState(layerIndex int, actionType string) {
	if layerIndex < 0 || layerIndex >= len(app.layers) {
		return
	}

	// Get image from layer texture
	img := rl.LoadImageFromTexture(app.layers[layerIndex].texture.Texture)
	defer rl.UnloadImage(img)

	// Convert to Go image
	width := int(img.Width)
	height := int(img.Height)
	goImg := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := rl.GetImageColor(*img, int32(x), int32(y))
			goImg.Set(x, height-1-y, color.RGBA{c.R, c.G, c.B, c.A}) // Flip Y
		}
	}

	// Truncate history if we're not at the end
	if app.historyIndex < len(app.history)-1 {
		app.history = app.history[:app.historyIndex+1]
	}

	// Add to history
	app.history = append(app.history, HistoryAction{
		actionType: actionType,
		layerIndex: layerIndex,
		layerData:  goImg,
	})
	app.historyIndex++

	// Limit history size
	if len(app.history) > app.maxHistory {
		app.history = app.history[1:]
		app.historyIndex--
	}
}

// Perform undo
func (app *App) Undo() {
	if app.historyIndex < 0 || app.historyIndex >= len(app.history) {
		return
	}

	action := app.history[app.historyIndex]
	if action.layerIndex < len(app.layers) {
		// Restore layer state
		layer := app.layers[action.layerIndex]

		// Convert Go image back to raylib texture
		bounds := action.layerData.Bounds()
		rlImg := rl.GenImageColor(bounds.Dx(), bounds.Dy(), rl.Blank)

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := action.layerData.At(x, y).RGBA()
				c := rl.Color{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
				rl.ImageDrawPixel(rlImg, int32(x), int32(bounds.Dy()-1-y), c) // Flip Y
			}
		}

		texture := rl.LoadTextureFromImage(rlImg)
		rl.UnloadImage(rlImg)

		// Draw to layer texture
		rl.BeginTextureMode(layer.texture)
		rl.ClearBackground(rl.Color{0, 0, 0, 0})
		rl.DrawTexture(texture, 0, 0, rl.White)
		rl.EndTextureMode()

		rl.UnloadTexture(texture)
	}

	app.historyIndex--
}

// Perform redo
func (app *App) Redo() {
	if app.historyIndex < len(app.history)-2 {
		app.historyIndex++

		// Skip to next action
		app.historyIndex++
		action := app.history[app.historyIndex]
		if action.layerIndex < len(app.layers) {
			// Restore layer state
			layer := app.layers[action.layerIndex]

			// Convert Go image back to raylib texture
			bounds := action.layerData.Bounds()
			rlImg := rl.GenImageColor(bounds.Dx(), bounds.Dy(), rl.Blank)

			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r, g, b, a := action.layerData.At(x, y).RGBA()
					c := rl.Color{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
					rl.ImageDrawPixel(rlImg, int32(x), int32(bounds.Dy()-1-y), c) // Flip Y
				}
			}

			texture := rl.LoadTextureFromImage(rlImg)
			rl.UnloadImage(rlImg)

			// Draw to layer texture
			rl.BeginTextureMode(layer.texture)
			rl.ClearBackground(rl.Color{0, 0, 0, 0})
			rl.DrawTexture(texture, 0, 0, rl.White)
			rl.EndTextureMode()

			rl.UnloadTexture(texture)
		}
	}
}

// Screen to canvas coordinates
func (app *App) ScreenToCanvas(screenX, screenY float32) (int, int) {
	canvasX := int((screenX - leftPanel - app.panX) / app.zoom)
	canvasY := int((screenY - 50 - app.panY) / app.zoom)
	return canvasX, canvasY
}

// Add new layer
func (app *App) AddLayer() {
	name := fmt.Sprintf("LAYER %d", app.layerCounter)
	app.layerCounter++
	newLayer := NewLayer(name, app.canvasWidth, app.canvasHeight)
	app.layers = append(app.layers, newLayer)
	app.activeLayer = len(app.layers) - 1
}

// Duplicate active layer
func (app *App) DuplicateActiveLayer() {
	srcLayer := app.layers[app.activeLayer]
	name := fmt.Sprintf("%s COPY", srcLayer.name)
	newLayer := NewLayer(name, app.canvasWidth, app.canvasHeight)

	// Copy content
	rl.BeginTextureMode(newLayer.texture)
	rl.DrawTextureRec(
		srcLayer.texture.Texture,
		rl.Rectangle{X: 0, Y: 0, Width: float32(srcLayer.texture.Texture.Width), Height: -float32(srcLayer.texture.Texture.Height)},
		rl.Vector2{X: 0, Y: 0},
		rl.White,
	)
	rl.EndTextureMode()

	newLayer.visible = srcLayer.visible
	newLayer.locked = srcLayer.locked
	newLayer.opacity = srcLayer.opacity

	// Insert after current layer
	app.layers = append(app.layers[:app.activeLayer+1], append([]*Layer{newLayer}, app.layers[app.activeLayer+1:]...)...)
	app.activeLayer++
}

// Delete active layer
func (app *App) DeleteActiveLayer() {
	if len(app.layers) > 1 && app.activeLayer > 0 { // Don't delete background
		// Unload the texture
		rl.UnloadRenderTexture(app.layers[app.activeLayer].texture)

		// Remove from slice
		app.layers = append(app.layers[:app.activeLayer], app.layers[app.activeLayer+1:]...)

		// Adjust active layer
		if app.activeLayer >= len(app.layers) {
			app.activeLayer = len(app.layers) - 1
		}
	}
}

// Toggle lock on active layer
func (app *App) ToggleLockActiveLayer() {
	app.layers[app.activeLayer].locked = !app.layers[app.activeLayer].locked
}

// Reorder layers
func (app *App) ReorderLayers(fromIndex, toIndex int) {
	if fromIndex < 0 || fromIndex >= len(app.layers) || toIndex < 0 || toIndex >= len(app.layers) || fromIndex == toIndex {
		return
	}

	layer := app.layers[fromIndex]

	// Remove layer from old position
	app.layers = append(app.layers[:fromIndex], app.layers[fromIndex+1:]...)

	// Insert at new position
	if toIndex > fromIndex {
		toIndex--
	}
	app.layers = append(app.layers[:toIndex], append([]*Layer{layer}, app.layers[toIndex:]...)...)

	// Update active layer if it was moved
	if app.activeLayer == fromIndex {
		app.activeLayer = toIndex
	} else if fromIndex < app.activeLayer && toIndex >= app.activeLayer {
		app.activeLayer--
	} else if fromIndex > app.activeLayer && toIndex <= app.activeLayer {
		app.activeLayer++
	}
}

// Export to PNG
func (app *App) ExportPNG(filename string) error {
	// Compose layers first
	app.ComposeLayers()

	// Get image from composite texture
	img := rl.LoadImageFromTexture(app.compositeTexture.Texture)
	defer rl.UnloadImage(img)

	// Convert to Go image
	width := int(img.Width)
	height := int(img.Height)
	goImg := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := rl.GetImageColor(*img, int32(x), int32(y))
			goImg.Set(x, height-1-y, color.RGBA{c.R, c.G, c.B, c.A}) // Flip Y
		}
	}

	// Save as PNG
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, goImg)
}

// Export to JPG
func (app *App) ExportJPG(filename string) error {
	// Compose layers first
	app.ComposeLayers()

	// Get image from composite texture
	img := rl.LoadImageFromTexture(app.compositeTexture.Texture)
	defer rl.UnloadImage(img)

	// Convert to Go image (RGB, no alpha for JPEG)
	width := int(img.Width)
	height := int(img.Height)
	goImg := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with white background
	draw.Draw(goImg, goImg.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := rl.GetImageColor(*img, int32(x), int32(y))
			// Alpha blend with white background
			alpha := float64(c.A) / 255.0
			r := uint8(float64(c.R)*alpha + 255*(1-alpha))
			g := uint8(float64(c.G)*alpha + 255*(1-alpha))
			b := uint8(float64(c.B)*alpha + 255*(1-alpha))
			goImg.Set(x, height-1-y, color.RGBA{r, g, b, 255}) // Flip Y
		}
	}

	// Save as JPEG
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return jpeg.Encode(file, goImg, &jpeg.Options{Quality: 95})
}

// Save project as .ddd
func (app *App) SaveProject(filename string) error {
	// Create project data
	project := ProjectData{
		CanvasWidth:  app.canvasWidth,
		CanvasHeight: app.canvasHeight,
		Layers:       make([]LayerData, len(app.layers)),
		Palette:      make([]ColorData, len(app.colorPalette)),
	}

	// Fill layer data
	for i, layer := range app.layers {
		project.Layers[i] = LayerData{
			Name:    layer.name,
			Visible: layer.visible,
			Locked:  layer.locked,
			Opacity: layer.opacity,
		}
	}

	// Fill palette data
	for i, color := range app.colorPalette {
		project.Palette[i] = ColorData{
			R: color.R,
			G: color.G,
			B: color.B,
			A: color.A,
		}
	}

	// Create zip file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// Save project.json
	jsonData, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return err
	}

	jsonFile, err := zipWriter.Create("project.json")
	if err != nil {
		return err
	}
	_, err = jsonFile.Write(jsonData)
	if err != nil {
		return err
	}

	// Save each layer as PNG
	for i, layer := range app.layers {
		img := rl.LoadImageFromTexture(layer.texture.Texture)
		defer rl.UnloadImage(img)

		// Convert to Go image
		width := int(img.Width)
		height := int(img.Height)
		goImg := image.NewRGBA(image.Rect(0, 0, width, height))

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				c := rl.GetImageColor(*img, int32(x), int32(y))
				goImg.Set(x, height-1-y, color.RGBA{c.R, c.G, c.B, c.A}) // Flip Y
			}
		}

		// Add to zip
		pngFile, err := zipWriter.Create(fmt.Sprintf("layer_%d.png", i))
		if err != nil {
			return err
		}

		err = png.Encode(pngFile, goImg)
		if err != nil {
			return err
		}
	}

	app.currentFilePath = filename
	return nil
}

// Load project from .ddd
func (app *App) LoadProject(filename string) error {
	// Open zip file
	reader, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Find and read project.json
	var projectFile *zip.File
	layerFiles := make(map[int]*zip.File)

	for _, file := range reader.File {
		if file.Name == "project.json" {
			projectFile = file
		} else if strings.HasPrefix(file.Name, "layer_") && strings.HasSuffix(file.Name, ".png") {
			// Extract layer index
			var idx int
			fmt.Sscanf(file.Name, "layer_%d.png", &idx)
			layerFiles[idx] = file
		}
	}

	if projectFile == nil {
		return fmt.Errorf("project.json not found in archive")
	}

	// Read project data
	rc, err := projectFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	var project ProjectData
	decoder := json.NewDecoder(rc)
	err = decoder.Decode(&project)
	if err != nil {
		return err
	}

	// Clear existing layers
	for _, layer := range app.layers {
		rl.UnloadRenderTexture(layer.texture)
	}
	app.layers = nil

	// Update canvas size
	app.canvasWidth = project.CanvasWidth
	app.canvasHeight = project.CanvasHeight

	// Recreate composite texture
	rl.UnloadRenderTexture(app.compositeTexture)
	app.compositeTexture = rl.LoadRenderTexture(int32(app.canvasWidth), int32(app.canvasHeight))

	// Load layers
	for i, layerData := range project.Layers {
		layer := NewLayer(layerData.Name, app.canvasWidth, app.canvasHeight)
		layer.visible = layerData.Visible
		layer.locked = layerData.Locked
		layer.opacity = layerData.Opacity

		// Load layer image
		if layerFile, ok := layerFiles[i]; ok {
			rc, err := layerFile.Open()
			if err == nil {
				// Read PNG data
				imgData, err := io.ReadAll(rc)
				rc.Close()

				if err == nil {
					// Decode PNG
					img, err := png.Decode(bytes.NewReader(imgData))
					if err == nil {
						// Convert to raylib texture
						bounds := img.Bounds()
						rlImg := rl.GenImageColor(bounds.Dx(), bounds.Dy(), rl.Blank)

						for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
							for x := bounds.Min.X; x < bounds.Max.X; x++ {
								r, g, b, a := img.At(x, y).RGBA()
								c := rl.Color{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
								rl.ImageDrawPixel(rlImg, int32(x), int32(bounds.Dy()-1-y), c) // Flip Y
							}
						}

						texture := rl.LoadTextureFromImage(rlImg)
						rl.UnloadImage(rlImg)

						// Draw to layer texture
						rl.BeginTextureMode(layer.texture)
						rl.DrawTexture(texture, 0, 0, rl.White)
						rl.EndTextureMode()

						rl.UnloadTexture(texture)
					}
				}
			}
		}

		app.layers = append(app.layers, layer)
	}

	// Load palette
	if len(project.Palette) > 0 {
		app.colorPalette = nil
		for _, colorData := range project.Palette {
			app.colorPalette = append(app.colorPalette, rl.Color{
				R: colorData.R,
				G: colorData.G,
				B: colorData.B,
				A: colorData.A,
			})
		}
	}

	// Reset view
	app.activeLayer = min(1, len(app.layers)-1)
	app.zoom = 1.0
	app.panX = 0
	app.panY = 0
	app.currentFilePath = filename

	return nil
}

// Update application
func (app *App) Update() {
	mousePos := rl.GetMousePosition()

	// Handle keyboard shortcuts
	if rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) {
		if rl.IsKeyPressed(rl.KeyZ) {
			if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
				app.Redo()
			} else {
				app.Undo()
			}
		}
		if rl.IsKeyPressed(rl.KeyY) {
			app.Redo()
		}
		if rl.IsKeyPressed(rl.KeyS) {
			if app.currentFilePath != "" {
				app.SaveProject(app.currentFilePath)
			} else {
				app.SaveProject("untitled.ddd")
			}
		}
		if rl.IsKeyPressed(rl.KeyO) {
			app.LoadProject("untitled.ddd") // In real app, would show file dialog
		}
		if rl.IsKeyPressed(rl.KeyE) {
			app.ExportPNG("export.png")
		}
	}

	// Handle file buttons
	for i, btn := range app.fileButtons {
		if rl.CheckCollisionPointRec(mousePos, btn.rect) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			switch i {
			case 0: // Save
				if app.currentFilePath != "" {
					app.SaveProject(app.currentFilePath)
				} else {
					app.SaveProject("untitled.ddd")
				}
			case 1: // Load
				app.LoadProject("untitled.ddd") // In real app, would show file dialog
			case 2: // Export
				app.ExportPNG("export.png")
			case 3: // Undo
				app.Undo()
			case 4: // Redo
				app.Redo()
			}
		}
	}

	// Handle layer dragging
	if app.isDraggingLayer {
		if rl.IsMouseButtonReleased(rl.MouseLeftButton) {
			// Calculate new position
			layerY := float32(100)
			newIndex := -1

			for i := len(app.layers) - 1; i >= 0; i-- {
				y := layerY + float32(len(app.layers)-1-i)*60
				if mousePos.Y < y+30 {
					newIndex = i
				}
			}

			if newIndex >= 0 && newIndex != app.draggedLayer {
				app.ReorderLayers(app.draggedLayer, newIndex)
			}

			app.isDraggingLayer = false
		}
		return // Don't process other inputs while dragging
	}

	// Handle space+drag panning
	if rl.IsKeyDown(rl.KeySpace) {
		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			app.isPanning = true
			app.panStartX = mousePos.X - app.panX
			app.panStartY = mousePos.Y - app.panY
		}
	}

	if app.isPanning && rl.IsMouseButtonDown(rl.MouseLeftButton) {
		app.panX = mousePos.X - app.panStartX
		app.panY = mousePos.Y - app.panStartY
	}

	if rl.IsMouseButtonReleased(rl.MouseLeftButton) || !rl.IsKeyDown(rl.KeySpace) {
		app.isPanning = false
	}

	// Don't process other inputs while panning
	if app.isPanning {
		return
	}

	// Handle zoom with mouse wheel
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 && mousePos.X > leftPanel && mousePos.X < screenWidth-rightPanel {
		oldZoom := app.zoom
		app.zoom *= 1.0 + wheel*0.1
		app.zoom = clamp(app.zoom, 0.25, 32.0)

		// Zoom towards mouse position
		if app.zoom != oldZoom {
			zoomFactor := app.zoom / oldZoom
			app.panX = mousePos.X - leftPanel - (mousePos.X-leftPanel-app.panX)*zoomFactor
			app.panY = mousePos.Y - 50 - (mousePos.Y-50-app.panY)*zoomFactor
		}
	}

	// Handle tool buttons
	for i := range app.toolButtons {
		btn := &app.toolButtons[i]
		btn.hover = rl.CheckCollisionPointRec(mousePos, btn.rect)

		if btn.hover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			// Deselect all buttons
			for j := range app.toolButtons {
				app.toolButtons[j].selected = false
			}
			btn.selected = true
			app.currentTool = ToolType(i)
		}
	}

	// Handle shape buttons
	for i := range app.shapeButtons {
		btn := &app.shapeButtons[i]
		btn.hover = rl.CheckCollisionPointRec(mousePos, btn.rect)

		if btn.hover && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			// Deselect all shape buttons
			for j := range app.shapeButtons {
				app.shapeButtons[j].selected = false
			}
			btn.selected = true
			app.penShape = PenShape(i)
		}
	}

	// Handle color palette
	paletteY := float32(400)
	for i, color := range app.colorPalette {
		x := float32(10 + (i%3)*25)
		y := paletteY + float32(i/3)*25
		rect := rl.Rectangle{X: x, Y: y, Width: 20, Height: 20}

		if rl.CheckCollisionPointRec(mousePos, rect) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			app.currentColor = color
		}
	}

	// Handle pen size slider
	if rl.CheckCollisionPointRec(mousePos, app.penSizeSlider.rect) && rl.IsMouseButtonDown(rl.MouseLeftButton) {
		relX := mousePos.X - app.penSizeSlider.rect.X
		app.penSizeSlider.value = app.penSizeSlider.min + (relX/app.penSizeSlider.rect.Width)*(app.penSizeSlider.max-app.penSizeSlider.min)
		app.penSizeSlider.value = clamp(app.penSizeSlider.value, app.penSizeSlider.min, app.penSizeSlider.max)
		app.penSize = app.penSizeSlider.value
	}

	// Handle layer buttons
	for i, btn := range app.layerButtons {
		if rl.CheckCollisionPointRec(mousePos, btn.rect) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			switch i {
			case 0: // New
				app.AddLayer()
			case 1: // Duplicate
				app.DuplicateActiveLayer()
			case 2: // Delete
				app.DeleteActiveLayer()
			case 3: // Lock
				app.ToggleLockActiveLayer()
			}
		}
	}

	// Handle layer selection and dragging
	layerY := float32(100)
	for i := len(app.layers) - 1; i >= 0; i-- {
		y := layerY + float32(len(app.layers)-1-i)*60
		layerRect := rl.Rectangle{
			X:      screenWidth - rightPanel + 10,
			Y:      y,
			Width:  rightPanel - 20,
			Height: 50,
		}

		if rl.CheckCollisionPointRec(mousePos, layerRect) && rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			// Check if clicking on visibility toggle
			visRect := rl.Rectangle{X: layerRect.X + 5, Y: layerRect.Y + 5, Width: 20, Height: 20}
			if rl.CheckCollisionPointRec(mousePos, visRect) {
				app.layers[i].visible = !app.layers[i].visible
			} else {
				app.activeLayer = i
				// Start dragging
				app.isDraggingLayer = true
				app.draggedLayer = i
				app.dragOffsetY = mousePos.Y - y
			}
		}
	}

	// Handle drawing on canvas
	if mousePos.X > leftPanel && mousePos.X < screenWidth-rightPanel && !app.layers[app.activeLayer].locked {
		canvasX, canvasY := app.ScreenToCanvas(mousePos.X, mousePos.Y)

		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			// Handle eyedropper tool
			if app.currentTool == ToolEyedropper {
				// Get color from composite texture
				app.ComposeLayers()
				img := rl.LoadImageFromTexture(app.compositeTexture.Texture)
				defer rl.UnloadImage(img)

				if canvasX >= 0 && canvasX < app.canvasWidth && canvasY >= 0 && canvasY < app.canvasHeight {
					color := rl.GetImageColor(*img, int32(canvasX), int32(app.canvasHeight-1-canvasY)) // Flip Y
					app.currentColor = color
				}
			} else {
				// Save state before any drawing operation
				if !app.layers[app.activeLayer].locked {
					app.SaveLayerState(app.activeLayer, "draw")
				}
				app.isDrawing = true
				app.lastMousePos = rl.Vector2{X: float32(canvasX), Y: float32(canvasY)}
				app.lineStart = app.lastMousePos
			}
		}
		currentPos := rl.Vector2{X: float32(canvasX), Y: float32(canvasY)}

		// Draw on active layer
		rl.BeginTextureMode(app.layers[app.activeLayer].texture)

		switch app.currentTool {
		case ToolPen, ToolBrush:
			if app.penShape == PenShapeSquare {
				// Draw square pen
				DrawSquareLine(app.lastMousePos, currentPos, app.penSize, app.currentColor)
			} else {
				// Draw round pen
				rl.DrawLineEx(app.lastMousePos, currentPos, app.penSize, app.currentColor)
				rl.DrawCircleV(currentPos, app.penSize/2, app.currentColor)
			}
		case ToolEraser:
			// Use blend mode for erasing
			rl.BeginBlendMode(rl.BlendSubtractColors)
			if app.penShape == PenShapeSquare {
				DrawSquareLine(app.lastMousePos, currentPos, app.penSize*2, rl.Color{255, 255, 255, 255})
			} else {
				rl.DrawLineEx(app.lastMousePos, currentPos, app.penSize*2, rl.Color{255, 255, 255, 255})
				rl.DrawCircleV(currentPos, app.penSize, rl.Color{255, 255, 255, 255})
			}
			rl.EndBlendMode()
		}

		rl.EndTextureMode()
		app.lastMousePos = currentPos
	}

	if rl.IsMouseButtonReleased(rl.MouseLeftButton) && app.isDrawing {
		if app.currentTool == ToolLine || app.currentTool == ToolRect || app.currentTool == ToolCircle {
			currentPos := rl.Vector2{X: float32(screenWidth), Y: float32(screenHeight)}

			rl.BeginTextureMode(app.layers[app.activeLayer].texture)

			switch app.currentTool {
			case ToolLine:
				rl.DrawLineEx(app.lineStart, currentPos, app.penSize, app.currentColor)
			case ToolRect:
				x := minf(app.lineStart.X, currentPos.X)
				y := minf(app.lineStart.Y, currentPos.Y)
				w := abs(currentPos.X - app.lineStart.X)
				h := abs(currentPos.Y - app.lineStart.Y)
				rl.DrawRectangleLines(int32(x), int32(y), int32(w), int32(h), app.currentColor)
			case ToolCircle:
				center := rl.Vector2{
					X: (app.lineStart.X + currentPos.X) / 2,
					Y: (app.lineStart.Y + currentPos.Y) / 2,
				}
				radius := rl.Vector2Distance(app.lineStart, currentPos) / 2
				rl.DrawCircleLines(int32(center.X), int32(center.Y), radius, app.currentColor)
			}

			rl.EndTextureMode()
		}
		app.isDrawing = false
	}

	// Handle panning with middle mouse button
	if rl.IsMouseButtonDown(rl.MouseMiddleButton) {
		delta := rl.GetMouseDelta()
		app.panX += delta.X
		app.panY += delta.Y
	}
}

// Draw square line (for square pen)
func DrawSquareLine(start, end rl.Vector2, size float32, color rl.Color) {
	// Bresenham's line algorithm with square brush
	dx := abs(end.X - start.X)
	dy := abs(end.Y - start.Y)
	sx := float32(1)
	sy := float32(1)
	if start.X > end.X {
		sx = -1
	}
	if start.Y > end.Y {
		sy = -1
	}
	err := dx - dy

	x0 := start.X
	y0 := start.Y

	for {
		// Draw square at current position
		rl.DrawRectangle(int32(x0-size/2), int32(y0-size/2), int32(size), int32(size), color)

		if x0 == end.X && y0 == end.Y {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// Render layers to composite texture
func (app *App) ComposeLayers() {
	rl.BeginTextureMode(app.compositeTexture)
	rl.ClearBackground(rl.Color{0, 0, 0, 0})

	for _, layer := range app.layers {
		if !layer.visible {
			continue
		}

		rl.DrawTextureRec(
			layer.texture.Texture,
			rl.Rectangle{X: 0, Y: 0, Width: float32(layer.texture.Texture.Width), Height: -float32(layer.texture.Texture.Height)},
			rl.Vector2{X: 0, Y: 0},
			rl.White,
		)
	}

	rl.EndTextureMode()
}

// Draw application
func (app *App) Draw() {
	rl.BeginDrawing()
	rl.ClearBackground(rl.Color{40, 40, 40, 255})

	mousePos := rl.GetMousePosition()

	// Compose layers
	app.ComposeLayers()

	// Draw left toolbar
	rl.DrawRectangle(0, 0, leftPanel, screenHeight, rl.Color{50, 50, 50, 255})

	// Draw title
	rl.DrawText("DELUXE DRAW", 10, 10, fontSize, rl.White)
	rl.DrawText("TOOLS", 10, 35, fontSize, rl.LightGray)

	// Draw tool buttons
	for i, btn := range app.toolButtons {
		color := rl.Color{70, 70, 70, 255}
		if btn.selected {
			color = rl.Color{100, 100, 150, 255}
		} else if btn.hover {
			color = rl.Color{80, 80, 80, 255}
		}

		rl.DrawRectangleRec(btn.rect, color)
		rl.DrawRectangleLinesEx(btn.rect, 1, rl.Color{90, 90, 90, 255})

		// Draw icon (simplified)
		textX := int32(btn.rect.X + btn.rect.Width/2 - 4)
		textY := int32(btn.rect.Y + btn.rect.Height/2 - 4)
		rl.DrawText(btn.text, textX, textY, fontSize, rl.White)

		// Draw tooltip on hover
		if btn.hover {
			tools := []string{"PEN", "BRUSH", "ERASER", "FILL", "PICKER", "LINE", "RECT", "CIRCLE", "MOVE", "ZOOM"}
			rl.DrawText(tools[i], int32(mousePos.X+10), int32(mousePos.Y), fontSize, rl.Yellow)
		}
	}

	// Draw pen size slider
	rl.DrawText(app.penSizeSlider.label, int32(app.penSizeSlider.rect.X), int32(app.penSizeSlider.rect.Y-12), fontSize, rl.LightGray)
	rl.DrawRectangleRec(app.penSizeSlider.rect, rl.Color{60, 60, 60, 255})
	sliderPos := app.penSizeSlider.rect.X + (app.penSizeSlider.value-app.penSizeSlider.min)/(app.penSizeSlider.max-app.penSizeSlider.min)*app.penSizeSlider.rect.Width
	rl.DrawRectangle(int32(sliderPos-2), int32(app.penSizeSlider.rect.Y), 4, int32(app.penSizeSlider.rect.Height), rl.White)
	rl.DrawText(fmt.Sprintf("%.0f", app.penSize), int32(app.penSizeSlider.rect.X), int32(app.penSizeSlider.rect.Y+25), fontSize, rl.White)

	// Draw shape selector
	rl.DrawText("SHAPE", 10, 345, fontSize, rl.LightGray)
	for _, btn := range app.shapeButtons {
		color := rl.Color{70, 70, 70, 255}
		if btn.selected {
			color = rl.Color{100, 100, 150, 255}
		} else if btn.hover {
			color = rl.Color{80, 80, 80, 255}
		}

		rl.DrawRectangleRec(btn.rect, color)
		rl.DrawRectangleLinesEx(btn.rect, 1, rl.Color{90, 90, 90, 255})

		// Draw tiny text
		fontSize := 6
		textW := rl.MeasureText(btn.text, int32(fontSize))
		textX := int32(btn.rect.X + btn.rect.Width/2 - float32(textW)/2)
		textY := int32(btn.rect.Y + btn.rect.Height/2 - 3)
		rl.DrawText(btn.text, textX, textY, int32(fontSize), rl.White)
	}

	// Draw color palette
	rl.DrawText("COLORS", 10, 385, fontSize, rl.LightGray)
	paletteY := float32(400)
	for i, color := range app.colorPalette {
		x := float32(10 + (i%3)*25)
		y := paletteY + float32(i/3)*25
		rect := rl.Rectangle{X: x, Y: y, Width: 20, Height: 20}

		rl.DrawRectangleRec(rect, color)
		if app.currentColor == color {
			rl.DrawRectangleLinesEx(rect, 2, rl.White)
		} else {
			rl.DrawRectangleLinesEx(rect, 1, rl.Color{70, 70, 70, 255})
		}
	}

	// Draw current color
	rl.DrawRectangle(10, 620, 40, 30, app.currentColor)
	rl.DrawRectangleLines(10, 620, 40, 30, rl.White)

	// Draw right panel (layers)
	rl.DrawRectangle(screenWidth-rightPanel, 0, rightPanel, screenHeight, rl.Color{50, 50, 50, 255})
	rl.DrawText("LAYERS", screenWidth-rightPanel+10, 10, fontSize, rl.White)

	// Draw layer entries (top to bottom)
	layerY := float32(100)
	for i := len(app.layers) - 1; i >= 0; i-- {
		y := layerY + float32(len(app.layers)-1-i)*60

		// Skip if being dragged
		if app.isDraggingLayer && i == app.draggedLayer {
			continue
		}

		// Layer background
		bgColor := rl.Color{60, 60, 60, 255}
		if i == app.activeLayer {
			bgColor = rl.Color{80, 80, 120, 255}
		}
		rl.DrawRectangle(screenWidth-rightPanel+10, int32(y), rightPanel-20, 50, bgColor)

		// Visibility toggle
		visX := int32(screenWidth - rightPanel + 15)
		visY := int32(y + 5)
		rl.DrawRectangle(visX, visY, 20, 20, rl.Color{40, 40, 40, 255})
		rl.DrawRectangleLines(visX, visY, 20, 20, rl.White)
		if app.layers[i].visible {
			rl.DrawText("V", visX+6, visY+6, fontSize, rl.White)
		}

		// Lock indicator
		if app.layers[i].locked {
			rl.DrawText("L", visX+45, visY+6, fontSize, rl.Yellow)
		}

		// Layer name
		nameColor := rl.White
		if app.layers[i].locked {
			nameColor = rl.Color{200, 200, 100, 255}
		}
		rl.DrawText(app.layers[i].name, screenWidth-rightPanel+45, int32(y+8), fontSize, nameColor)

		// Mini preview
		previewSize := float32(30)
		previewX := float32(screenWidth - rightPanel + rightPanel - 50)
		previewY := y + 10

		// Draw checkerboard background for preview
		rl.DrawRectangle(int32(previewX), int32(previewY), int32(previewSize), int32(previewSize), rl.Color{100, 100, 100, 255})
		rl.DrawRectangle(int32(previewX+previewSize/2), int32(previewY), int32(previewSize/2), int32(previewSize/2), rl.Color{150, 150, 150, 255})
		rl.DrawRectangle(int32(previewX), int32(previewY+previewSize/2), int32(previewSize/2), int32(previewSize/2), rl.Color{150, 150, 150, 255})

		// Draw layer preview
		srcRect := rl.Rectangle{X: 0, Y: 0, Width: float32(app.canvasWidth), Height: -float32(app.canvasHeight)}
		dstRect := rl.Rectangle{X: previewX, Y: previewY, Width: previewSize, Height: previewSize}
		rl.DrawTexturePro(app.layers[i].texture.Texture, srcRect, dstRect, rl.Vector2{}, 0, rl.White)
		rl.DrawRectangleLinesEx(dstRect, 1, rl.Color{70, 70, 70, 255})
	}

	// Draw dragged layer
	if app.isDraggingLayer {
		y := mousePos.Y - app.dragOffsetY

		// Layer background with transparency
		rl.DrawRectangle(screenWidth-rightPanel+10, int32(y), rightPanel-20, 50, rl.Color{100, 100, 150, 200})

		// Layer name
		rl.DrawText(app.layers[app.draggedLayer].name, screenWidth-rightPanel+45, int32(y+8), fontSize, rl.White)

		// Draw insertion line
		layerY := float32(100)
		for i := len(app.layers) - 1; i >= 0; i-- {
			checkY := layerY + float32(len(app.layers)-1-i)*60
			if mousePos.Y < checkY+30 {
				rl.DrawRectangle(screenWidth-rightPanel+10, int32(checkY-2), rightPanel-20, 4, rl.Yellow)
				break
			}
		}
	}

	// Draw layer buttons
	for _, btn := range app.layerButtons {
		color := rl.Color{70, 70, 70, 255}
		if rl.CheckCollisionPointRec(mousePos, btn.rect) {
			color = rl.Color{80, 80, 80, 255}
		}

		rl.DrawRectangleRec(btn.rect, color)
		rl.DrawRectangleLinesEx(btn.rect, 1, rl.Color{90, 90, 90, 255})

		textW := rl.MeasureText(btn.text, fontSize)
		textX := int32(btn.rect.X + btn.rect.Width/2 - float32(textW)/2)
		textY := int32(btn.rect.Y + btn.rect.Height/2 - 4)
		rl.DrawText(btn.text, textX, textY, fontSize, rl.White)
	}

	// Draw top bar
	rl.DrawRectangle(leftPanel, 0, screenWidth-leftPanel-rightPanel, 50, rl.Color{60, 60, 60, 255})

	// Draw file buttons
	for _, btn := range app.fileButtons {
		color := rl.Color{70, 70, 70, 255}
		if rl.CheckCollisionPointRec(mousePos, btn.rect) {
			color = rl.Color{80, 80, 80, 255}
		}

		rl.DrawRectangleRec(btn.rect, color)
		rl.DrawRectangleLinesEx(btn.rect, 1, rl.Color{90, 90, 90, 255})

		textW := rl.MeasureText(btn.text, fontSize)
		textX := int32(btn.rect.X + btn.rect.Width/2 - float32(textW)/2)
		textY := int32(btn.rect.Y + btn.rect.Height/2 - 4)
		rl.DrawText(btn.text, textX, textY, fontSize, rl.White)
	}

	// Draw status info
	panStatus := ""
	if app.isPanning {
		panStatus = " | PANNING"
	}
	fileStatus := "UNTITLED"
	if app.currentFilePath != "" {
		fileStatus = filepath.Base(app.currentFilePath)
	}
	historyStatus := ""
	if len(app.history) > 0 {
		historyStatus = fmt.Sprintf(" | HISTORY: %d/%d", app.historyIndex+1, len(app.history))
	}
	info := fmt.Sprintf("FILE: %s | ZOOM: %.0f%% | %dX%d | %s%s%s",
		fileStatus, app.zoom*100, app.canvasWidth, app.canvasHeight, app.layers[app.activeLayer].name, panStatus, historyStatus)
	rl.DrawText(info, leftPanel+280, 20, fontSize, rl.White)

	// Draw canvas viewport
	rl.BeginScissorMode(leftPanel, 50, screenWidth-leftPanel-rightPanel, screenHeight-50)

	// Draw checkerboard background
	tileSize := int32(16 * app.zoom)
	offsetX := int32(app.panX) % (tileSize * 2)
	offsetY := int32(app.panY) % (tileSize * 2)

	for y := int32(-2); y < int32(screenHeight/int(tileSize))+2; y++ {
		for x := int32(-2); x < int32(screenWidth/int(tileSize))+2; x++ {
			if (x+y)%2 == 0 {
				rl.DrawRectangle(
					leftPanel+x*tileSize+offsetX,
					50+y*tileSize+offsetY,
					tileSize, tileSize,
					rl.Color{150, 150, 150, 255},
				)
			}
		}
	}

	// Draw canvas
	srcRect := rl.Rectangle{X: 0, Y: 0, Width: float32(app.canvasWidth), Height: -float32(app.canvasHeight)}
	dstRect := rl.Rectangle{
		X:      leftPanel + app.panX,
		Y:      50 + app.panY,
		Width:  float32(app.canvasWidth) * app.zoom,
		Height: float32(app.canvasHeight) * app.zoom,
	}
	rl.DrawTexturePro(app.compositeTexture.Texture, srcRect, dstRect, rl.Vector2{}, 0, rl.White)

	// Draw canvas border
	rl.DrawRectangleLinesEx(dstRect, 2, rl.Color{100, 100, 100, 255})

	// Draw cursor
	if mousePos.X > leftPanel && mousePos.X < screenWidth-rightPanel && !app.isPanning {
		canvasX, canvasY := app.ScreenToCanvas(mousePos.X, mousePos.Y)

		if canvasX >= 0 && canvasX < app.canvasWidth && canvasY >= 0 && canvasY < app.canvasHeight {
			switch app.currentTool {
			case ToolPen, ToolBrush, ToolEraser:
				if app.penShape == PenShapeSquare {
					size := app.penSize * app.zoom
					rl.DrawRectangleLines(int32(mousePos.X-size/2), int32(mousePos.Y-size/2), int32(size), int32(size), rl.White)
				} else {
					radius := app.penSize * app.zoom / 2
					rl.DrawCircleLines(int32(mousePos.X), int32(mousePos.Y), radius, rl.White)
				}
			case ToolEyedropper:
				rl.DrawRectangleLines(int32(mousePos.X-5), int32(mousePos.Y-5), 10, 10, rl.White)
			}
		}
	}

	// Draw panning cursor
	if app.isPanning {
		rl.DrawText("HAND", int32(mousePos.X+10), int32(mousePos.Y-10), fontSize, rl.Yellow)
	}

	// Draw space hint
	if rl.IsKeyDown(rl.KeySpace) && !app.isPanning {
		rl.DrawText("CLICK AND DRAG TO PAN", int32(mousePos.X+10), int32(mousePos.Y+10), fontSize, rl.Yellow)
	}

	rl.EndScissorMode()

	// Draw shortcuts help
	rl.DrawText("CTRL+Z: UNDO | CTRL+Y: REDO | SPACE+DRAG: PAN", 10, screenHeight-20, fontSize, rl.LightGray)

	rl.EndDrawing()
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func abs(a float32) float32 {
	if a < 0 {
		return -a
	}
	return a
}

func clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func getCurrentToolName(tool ToolType) string {
	names := []string{"PEN", "BRUSH", "ERASER", "FILL", "PICKER", "LINE", "RECT", "CIRCLE", "MOVE", "ZOOM"}
	if int(tool) < len(names) {
		return names[tool]
	}
	return "UNKNOWN"
}

func main() {
	rl.InitWindow(screenWidth, screenHeight, "Deluxe Draw - Advanced Sprite Editor")
	rl.SetTargetFPS(60)

	app := NewApp()

	for !rl.WindowShouldClose() {
		app.Update()
		app.Draw()
	}

	// Clean up
	for _, layer := range app.layers {
		rl.UnloadRenderTexture(layer.texture)
	}
	rl.UnloadRenderTexture(app.compositeTexture)
	rl.CloseWindow()
}
