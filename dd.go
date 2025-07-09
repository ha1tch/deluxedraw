package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	screenWidth  = 1280
	screenHeight = 800
	fontSize     = 8
	leftPanel    = 100
	rightPanel   = 200
)

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

	// State
	isDrawing    bool
	lastMousePos rl.Vector2
	lineStart    rl.Vector2

	// Render targets
	compositeTexture rl.RenderTexture2D
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
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 10), Y: float32(screenHeight - 40), Width: 50, Height: 30}, text: "NEW"},
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 70), Y: float32(screenHeight - 40), Width: 50, Height: 30}, text: "DELETE"},
		{rect: rl.Rectangle{X: float32(screenWidth - rightPanel + 130), Y: float32(screenHeight - 40), Width: 50, Height: 30}, text: "LOCK"},
	}

	return app
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

// Update application
func (app *App) Update() {
	mousePos := rl.GetMousePosition()

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
		app.zoom = clamp(app.zoom, 0.25, 8.0)

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
			case 1: // Delete
				app.DeleteActiveLayer()
			case 2: // Lock
				app.ToggleLockActiveLayer()
			}
		}
	}

	// Handle layer selection in right panel
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
			}
		}
	}

	// Handle drawing on canvas
	if mousePos.X > leftPanel && mousePos.X < screenWidth-rightPanel && !app.layers[app.activeLayer].locked {
		canvasX, canvasY := app.ScreenToCanvas(mousePos.X, mousePos.Y)

		if rl.IsMouseButtonPressed(rl.MouseLeftButton) {
			app.isDrawing = true
			app.lastMousePos = rl.Vector2{X: float32(canvasX), Y: float32(canvasY)}
			app.lineStart = app.lastMousePos
		}

		if rl.IsMouseButtonDown(rl.MouseLeftButton) && app.isDrawing {
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
				currentPos := rl.Vector2{X: float32(canvasX), Y: float32(canvasY)}

				rl.BeginTextureMode(app.layers[app.activeLayer].texture)

				switch app.currentTool {
				case ToolLine:
					rl.DrawLineEx(app.lineStart, currentPos, app.penSize, app.currentColor)
				case ToolRect:
					x := min(app.lineStart.X, currentPos.X)
					y := min(app.lineStart.Y, currentPos.Y)
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
	panStatus := ""
	if app.isPanning {
		panStatus = " | PANNING"
	}
	info := fmt.Sprintf("ZOOM: %.0f%% | SIZE: %dX%d | TOOL: %s | LAYER: %s%s",
		app.zoom*100, app.canvasWidth, app.canvasHeight, getCurrentToolName(app.currentTool), app.layers[app.activeLayer].name, panStatus)
	rl.DrawText(info, leftPanel+10, 20, fontSize, rl.White)

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

	rl.EndDrawing()
}

// Helper functions
func min(a, b float32) float32 {
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
	rl.InitWindow(screenWidth, screenHeight, "Deluxe Draw")
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
