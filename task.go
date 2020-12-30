package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/goware/urlx"
	"github.com/ncruces/zenity"
	"github.com/pkg/browser"
	"github.com/tanema/gween/ease"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	TASK_TYPE_NOTE = iota
	TASK_TYPE_IMAGE
	TASK_TYPE_LINE
)

type URLButton struct {
	Pos  rl.Vector2
	Text string
	Link string
	Size rl.Vector2
}

type Task struct {
	Rect     rl.Rectangle
	Board    *Board
	Position rl.Vector2
	Open     bool
	Selected bool
	MinSize  rl.Vector2
	MaxSize  rl.Vector2

	TaskType    *ButtonGroup
	Description *Textbox

	CreationTime   time.Time

	Image                        rl.Texture2D

	FilePathTextbox    *Textbox
	PrevFilePath       string
	DisplaySize        rl.Vector2
	Resizing           bool
	Dragging           bool
	MouseDragStart     rl.Vector2
	TaskDragStart      rl.Vector2
	ResizeRect         rl.Rectangle
	ImageSizeResetRect rl.Rectangle

	OriginalIndentation int
	NumberingPrefix     []int
	PrefixText          string
	ID                  int
	PercentageComplete  float32
	Visible             bool

	LineEndings []*Task
	LineBase    *Task
	LineBezier  *Checkbox

	GridPositions []Position
	Valid         bool

	EditPanel                      *Panel
	LoadMediaButton                *Button
	ClearMediaButton               *Button
	CreationLabel                  *Label
	DisplayedText                  string
	URLButtons                     []URLButton
	SuccessfullyLoadedResourceOnce bool
}

func NewTask(board *Board) *Task {

	postX := float32(180)

	task := &Task{
		Rect:                         rl.Rectangle{0, 0, 16, 16},
		Board:                        board,
		TaskType:                     NewButtonGroup(0, 32, 500, 32, 2, "Note", "Image", "Line"),
		Description:                  NewTextbox(postX, 64, 512, 32),
		NumberingPrefix:              []int{-1},
		ID:                           board.Project.FirstFreeID(),
		FilePathTextbox:              NewTextbox(postX, 64, 512, 16),
		LineEndings:                  []*Task{},
		LineBezier:                   NewCheckbox(postX, 64, 32, 32),
		GridPositions:                []Position{},
		Valid:                        true,
		LoadMediaButton:              NewButton(0, 0, 128, 32, "Load", false),
		CreationLabel:                NewLabel("Creation time"),
	}

	task.SetPanel()

	task.CreationTime = time.Now()

	task.MinSize = rl.Vector2{task.Rect.Width, task.Rect.Height}
	task.MaxSize = rl.Vector2{0, 0}
	task.Description.AllowNewlines = true
	task.FilePathTextbox.AllowNewlines = false

	task.FilePathTextbox.VerticalAlignment = ALIGN_CENTER

	return task
}

func (task *Task) SetPanel() {

	task.EditPanel = NewPanel(63, 64, 960/4*3, 560/4*3)

	column := task.EditPanel.AddColumn()
	row := column.Row()
	row.Item(NewLabel("Task Type:"))
	row = column.Row()
	row.Item(task.TaskType)

	row = column.Row()
	row.Item(NewLabel(""))

	row = column.Row()
	row.Item(NewLabel("Created On:"))
	row.Item(task.CreationLabel)

	column.Row().Item(NewLabel("Description:"),
		TASK_TYPE_NOTE)
	column.Row().Item(task.Description,
		TASK_TYPE_NOTE)

	row = column.Row()
	row.Item(NewLabel("Filepath:"), TASK_TYPE_IMAGE)
	row = column.Row()
	row.Item(task.FilePathTextbox, TASK_TYPE_IMAGE)
	row = column.Row()
	row.Item(task.LoadMediaButton, TASK_TYPE_IMAGE)

	row = column.Row()
	row.Item(NewLabel("Bezier Lines:"), TASK_TYPE_LINE)
	row.Item(task.LineBezier, TASK_TYPE_LINE)

	for _, row := range column.Rows {
		row.VerticalSpacing = 16
	}

}

func (task *Task) Clone() *Task {
	copyData := *task // By de-referencing and then making another reference, we should be essentially copying the struct

	desc := *copyData.Description
	copyData.Description = &desc

	tt := *copyData.TaskType
	copyData.TaskType = &tt

	cPath := *copyData.FilePathTextbox
	copyData.FilePathTextbox = &cPath

	bl := *copyData.LineBezier
	copyData.LineBezier = &bl

	if task.LineBase != nil {
		copyData.LineBase = task.LineBase
		copyData.LineBase.LineEndings = append(copyData.LineBase.LineEndings, &copyData)
	} else if len(task.ValidLineEndings()) > 0 {
		copyData.LineEndings = []*Task{}
		for _, end := range task.ValidLineEndings() {
			newEnding := copyData.CreateLineEnding()
			newEnding.Position = end.Position
			newEnding.Board.ReorderTasks()
		}
	}

	for _, ending := range copyData.LineEndings {
		ending.Valid = false
		task.Board.UndoBuffer.Capture(ending)
		ending.Valid = true
		task.Board.UndoBuffer.Capture(ending)

		ending.Selected = true
	}

	copyData.PrevFilePath = ""

	copyData.ID = copyData.Board.Project.FirstFreeID()

	copyData.ReceiveMessage(MessageTaskClose, nil) // We do this to recreate the resources for the Task, if necessary.

	copyData.SetPanel()

	return &copyData
}

// Serialize returns the Task's changeable properties in the form of a complete JSON object in a string.
func (task *Task) Serialize() string {

	jsonData := "{}"

	jsonData, _ = sjson.Set(jsonData, `BoardIndex`, task.Board.Index())
	jsonData, _ = sjson.Set(jsonData, `Position\.X`, task.Position.X)
	jsonData, _ = sjson.Set(jsonData, `Position\.Y`, task.Position.Y)

	if task.Resizeable() {
		jsonData, _ = sjson.Set(jsonData, `ImageDisplaySize\.X`, task.DisplaySize.X)
		jsonData, _ = sjson.Set(jsonData, `ImageDisplaySize\.Y`, task.DisplaySize.Y)
	}

	jsonData, _ = sjson.Set(jsonData, `Description`, task.Description.Text())

	if task.UsesMedia() && task.FilePathTextbox.Text() != "" {

		resourcePath := task.FilePathTextbox.Text()

		if resource := task.Board.Project.RetrieveResource(resourcePath); resource != nil && !resource.Temporary {

			// Turn the file path absolute if it's not a remote path
			relative, err := filepath.Rel(filepath.Dir(task.Board.Project.FilePath), resourcePath)

			if err == nil {

				jsonData, _ = sjson.Set(jsonData, `FilePath`, strings.Split(relative, string(filepath.Separator)))
				resourcePath = ""

			}

		}

		if resourcePath != "" {
			jsonData, _ = sjson.Set(jsonData, `FilePath`, resourcePath)
		}

	}

	jsonData, _ = sjson.Set(jsonData, `Selected`, task.Selected)

  {
    jtype := "unknown"

    if task.TaskType.CurrentChoice == TASK_TYPE_IMAGE {
      jtype = "image"
    } else if task.TaskType.CurrentChoice == TASK_TYPE_LINE {
      jtype = "line"
    } else if task.TaskType.CurrentChoice == TASK_TYPE_NOTE {
      jtype = "note"
    }

    jsonData, _ = sjson.Set(jsonData, `TaskType\.CurrentChoice`, jtype)
  }

	jsonData, _ = sjson.Set(jsonData, `CreationTime`, task.CreationTime.Format(`Jan 2 2006 15:04:05`))

	if task.Is(TASK_TYPE_LINE) {

		// We want to set this in all cases, not just if it's a Line with valid line ending Task pointers;
		// that way it serializes consistently regardless of how many line endings it has.
		jsonData, _ = sjson.Set(jsonData, `BezierLines`, task.LineBezier.Checked)

		if lineEndings := task.ValidLineEndings(); len(lineEndings) > 0 {

			lineEndingPositions := []float32{}
			for _, ending := range task.ValidLineEndings() {
				if ending.Valid {
					lineEndingPositions = append(lineEndingPositions, ending.Position.X, ending.Position.Y)
				}
			}

			jsonData, _ = sjson.Set(jsonData, `LineEndings`, lineEndingPositions)

		}

	}

	return jsonData

}

// Serializable returns if Tasks are able to be serialized properly. Only line endings aren't properly serializeable
func (task *Task) Serializable() bool {
	return !task.Is(TASK_TYPE_LINE) || task.LineBase == nil
}

// Deserialize applies the JSON data provided to the Task, effectively "loading" it from that state. Previously,
// this was done via a map[string]interface{} which was loaded using a Golang JSON decoder, but it seems like it's
// significantly faster to use gjson and sjson to get and set JSON directly from a string, and for undo and redo,
// it seems to be easier to serialize and deserialize using a string (same as saving and loading) than altering
// the functions to work (as e.g. loading numbers from JSON gives float64s, but passing the map[string]interface{} directly from
// deserialization to serialization contains values that may be other discrete number types).
func (task *Task) Deserialize(jsonData string) {

	// JSON encodes all numbers as 64-bit floats, so this saves us some visual ugliness.
	getFloat := func(name string) float32 {
		return float32(gjson.Get(jsonData, name).Float())
	}

//	getInt := func(name string) int {
//		return int(gjson.Get(jsonData, name).Int())
//	}

	getBool := func(name string) bool {
		return gjson.Get(jsonData, name).Bool()
	}

	getString := func(name string) string {
		return gjson.Get(jsonData, name).String()
	}

	hasData := func(name string) bool {
		return gjson.Get(jsonData, name).Exists()
	}

	task.Position.X = getFloat(`Position\.X`)
	task.Position.Y = getFloat(`Position\.Y`)

	task.Rect.X = task.Position.X
	task.Rect.Y = task.Position.Y

	if gjson.Get(jsonData, `ImageDisplaySize\.X`).Exists() {
		task.DisplaySize.X = getFloat(`ImageDisplaySize\.X`)
		task.DisplaySize.Y = getFloat(`ImageDisplaySize\.Y`)
	}

	task.Description.SetText(getString(`Description`))

	if f := gjson.Get(jsonData, `FilePath`); f.Exists() {

		if f.IsArray() {
			str := []string{}
			for _, component := range f.Array() {
				str = append(str, component.String())
			}

			// We need to go from the project file as the "root", as otherwise it will be relative
			// to the current working directory (which is not ideal).
			str = append([]string{filepath.Dir(task.Board.Project.FilePath)}, str...)
			joinedElements := strings.Join(str, string(filepath.Separator))
			abs, _ := filepath.Abs(joinedElements)

			task.FilePathTextbox.SetText(abs)
		} else {
			task.FilePathTextbox.SetText(getString(`FilePath`))
		}

	}

	task.Selected = getBool(`Selected`)

  {
    jtype := getString(`TaskType\.CurrentChoice`)

    if jtype == "image" {
      task.TaskType.CurrentChoice = TASK_TYPE_IMAGE
    } else if jtype == "line" {
      task.TaskType.CurrentChoice = TASK_TYPE_LINE
    } else if jtype == "note" {
      task.TaskType.CurrentChoice = TASK_TYPE_NOTE
    }
  }

	creationTime, err := time.Parse(`Jan 2 2006 15:04:05`, getString(`CreationTime`))
	if err == nil {
		task.CreationTime = creationTime
	}

	if hasData(`BezierLines`) {
		task.LineBezier.Checked = getBool(`BezierLines`)
	}

	if hasData(`LineEndings`) {
		endPositions := gjson.Get(jsonData, `LineEndings`).Array()
		for i := 0; i < len(endPositions); i += 2 {
			ending := task.CreateLineEnding()
			ending.Position.X = float32(endPositions[i].Float())
			ending.Position.Y = float32(endPositions[i+1].Float())

			ending.Rect.X = ending.Position.X
			ending.Rect.Y = ending.Position.Y
		}
	}

	// We do this to update the task after loading all of the information.
	task.LoadResource()
}

func (task *Task) Update() {

  task.MinSize = rl.Vector2{16, 16}
  task.MaxSize = rl.Vector2{0, 0}

	if task.Selected && task.Dragging && !task.Resizing {
		delta := rl.Vector2Subtract(GetWorldMousePosition(), task.MouseDragStart)
		task.Position = rl.Vector2Add(task.TaskDragStart, delta)
		task.Rect.X = task.Position.X
		task.Rect.Y = task.Position.Y
	}

	if task.Dragging && MouseReleased(rl.MouseLeftButton) {
		//task.Board.SendMessage(MessageDropped, nil)
    task.Dragging = false
		//task.Board.ReorderTasks()
	}

	if !task.Dragging || task.Resizing {

		if math.Abs(float64(task.Rect.X-task.Position.X)) <= 1 {
			task.Rect.X = task.Position.X
		}

		if math.Abs(float64(task.Rect.Y-task.Position.Y)) <= 1 {
			task.Rect.Y = task.Position.Y
		}

	}

	task.Rect.X += (task.Position.X - task.Rect.X) * 0.2
	task.Rect.Y += (task.Position.Y - task.Rect.Y) * 0.2

	task.Visible = true

	scrW := float32(rl.GetScreenWidth()) / camera.Zoom
	scrH := float32(rl.GetScreenHeight()) / camera.Zoom

	// Slight optimization
	cameraRect := rl.Rectangle{camera.Target.X - (scrW / 2), camera.Target.Y - (scrH / 2), scrW, scrH}

	if task.Board.Project.FullyInitialized {
		if !rl.CheckCollisionRecs(task.Rect, cameraRect) {
			task.Visible = false
		}
	}

	if task.Resizeable() && task.Selected && (!task.Is(TASK_TYPE_IMAGE) || task.Image.ID > 0) {
		// Only valid images or other resizeable Task Types can be resized
		task.ResizeRect = task.Rect
		task.ResizeRect.Width = 8
		task.ResizeRect.Height = 8

		//if task.Board.Project.ZoomLevel <= 1 && task.Image.Width >= 32 && task.Image.Height >= 32 {
		//	task.ResizeRect.Width *= 2
		//	task.ResizeRect.Height *= 2
		//}

		task.ResizeRect.X += task.Rect.Width - task.ResizeRect.Width
		task.ResizeRect.Y += task.Rect.Height - task.ResizeRect.Height

		task.ResizeRect.X = float32(int32(task.ResizeRect.X))
		task.ResizeRect.Y = float32(int32(task.ResizeRect.Y))
		task.ResizeRect.Width = float32(int32(task.ResizeRect.Width))
		task.ResizeRect.Height = float32(int32(task.ResizeRect.Height))

		selectedTaskCount := len(task.Board.SelectedTasks(false))

		if rl.CheckCollisionPointRec(GetWorldMousePosition(), task.ResizeRect) && MousePressed(rl.MouseLeftButton) && selectedTaskCount == 1 {
			task.Resizing = true
			task.Board.Project.ResizingImage = true
			task.Board.SendMessage(MessageDropped, nil)
		} else if !MouseDown(rl.MouseLeftButton) || task.Open || task.Board.Project.ContextMenuOpen {
			if task.Resizing {
				task.Resizing = false
				task.Board.Project.ResizingImage = false
				task.Board.SendMessage(MessageDropped, nil)
			}
		}

		if task.Resizing {

			endPoint := GetWorldMousePosition()

			task.DisplaySize.X = endPoint.X - task.Rect.X
			task.DisplaySize.Y = endPoint.Y - task.Rect.Y

			if task.Is(TASK_TYPE_IMAGE) {

				if !programSettings.Keybindings.On(KBUnlockImageASR) {
					asr := float32(task.Image.Height) / float32(task.Image.Width)
					task.DisplaySize.Y = task.DisplaySize.X * asr

				}

				if !programSettings.Keybindings.On(KBUnlockImageGrid) {
					task.DisplaySize = task.Board.Project.LockPositionToGrid(task.DisplaySize)
				}

			} else {
				task.DisplaySize = task.Board.Project.LockPositionToGrid(task.DisplaySize)
			}

		}

		if task.DisplaySize.X < task.MinSize.X && task.MinSize.X > 0 {
			task.DisplaySize.X = task.MinSize.X
		}

		if task.DisplaySize.Y < task.MinSize.Y && task.MinSize.Y > 0 {
			task.DisplaySize.Y = task.MinSize.Y
		}

		if task.DisplaySize.X > task.MaxSize.X && task.MaxSize.X > 0 {
			task.DisplaySize.X = task.MaxSize.X
		}

		if task.DisplaySize.Y > task.MaxSize.Y && task.MaxSize.Y > 0 {
			task.DisplaySize.Y = task.MaxSize.Y
		}

		switch taskType := task.TaskType.CurrentChoice; taskType {

		case TASK_TYPE_IMAGE:

			task.ImageSizeResetRect = task.ResizeRect
			task.ImageSizeResetRect.X = task.Rect.X
			task.ImageSizeResetRect.Y = task.Rect.Y

			if selectedTaskCount == 1 && rl.CheckCollisionPointRec(GetWorldMousePosition(), task.ImageSizeResetRect) && MousePressed(rl.MouseLeftButton) {
				task.DisplaySize.X = float32(task.Image.Width)
				task.DisplaySize.Y = float32(task.Image.Height)
			}
		}
	}
}

func (task *Task) DrawLine() {

	if task.Is(TASK_TYPE_LINE) {

		outlineColor := getThemeColor(GUI_INSIDE)
		color := getThemeColor(GUI_FONT_COLOR)

		for _, ending := range task.ValidLineEndings() {

			bp := rl.Vector2{task.Rect.X, task.Rect.Y}
			bp.X += float32(task.Board.Project.GridSize) / 2
			bp.Y += float32(task.Board.Project.GridSize) / 2
			ep := rl.Vector2{ending.Rect.X, ending.Rect.Y}
			ep.X += float32(task.Board.Project.GridSize) / 2
			ep.Y += float32(task.Board.Project.GridSize) / 2

			if task.LineBezier.Checked {
				if task.Board.Project.OutlineTasks.Checked {
					rl.DrawLineBezier(bp, ep, 4, outlineColor)
				}
				rl.DrawLineBezier(bp, ep, 2, color)
			} else {
				if task.Board.Project.OutlineTasks.Checked {
					rl.DrawLineEx(bp, ep, 4, outlineColor)
				}
				rl.DrawLineEx(bp, ep, 2, color)
			}

		}

	}

}

func (task *Task) Draw() {

	if !task.Visible {
		return
	}

	name := task.Description.Text()

	extendedText := false

	taskType := task.TaskType.CurrentChoice

	switch taskType {

	case TASK_TYPE_IMAGE:
		_, filename := filepath.Split(task.FilePathTextbox.Text())
		name = filename
	}

	invalidImage := task.Image.ID == 0 
	if !invalidImage && task.Is(TASK_TYPE_IMAGE) {
		name = ""
	}

	taskDisplaySize := task.DisplaySize

	if !task.Is(TASK_TYPE_IMAGE) {

		taskDisplaySize = rl.MeasureTextEx(font, name, float32(programSettings.FontSize), spacing)

		if taskDisplaySize.X > 0 {
			taskDisplaySize.X += 4
		}

		if task.Board.Project.ShowIcons.Checked && (!task.Is(TASK_TYPE_IMAGE) || invalidImage) {
			taskDisplaySize.X += 16
			if extendedText {
				taskDisplaySize.X += 16
			}
		}

		taskDisplaySize.Y, _ = TextHeight(name, false) // Custom spacing to better deal with custom fonts
		taskDisplaySize.X = float32((math.Ceil(float64((taskDisplaySize.X + 4) / float32(task.Board.Project.GridSize))))) * float32(task.Board.Project.GridSize)
		taskDisplaySize.Y = float32((math.Ceil(float64((taskDisplaySize.Y) / float32(task.Board.Project.GridSize))))) * float32(task.Board.Project.GridSize)

	}

	if task.Is(TASK_TYPE_LINE) {
		taskDisplaySize.X = 16
		taskDisplaySize.Y = 16
	}

	if taskDisplaySize.X < task.MinSize.X {
		taskDisplaySize.X = task.MinSize.X
	}
	if taskDisplaySize.Y < task.MinSize.Y {
		taskDisplaySize.Y = task.MinSize.Y
	}

	if task.MaxSize.X > 0 && taskDisplaySize.X > task.MaxSize.X {
		taskDisplaySize.X = task.MaxSize.X
	}
	if task.MaxSize.Y > 0 && taskDisplaySize.Y > task.MaxSize.Y {
		taskDisplaySize.Y = task.MaxSize.Y
	}

	if (task.Is(TASK_TYPE_IMAGE) && task.Image.ID != 0) {
		if task.Rect.Width != taskDisplaySize.X || task.Rect.Height != taskDisplaySize.Y {
			task.Rect.Width = taskDisplaySize.X
			task.Rect.Height = taskDisplaySize.Y
			task.Board.RemoveTaskFromGrid(task)
			task.Board.AddTaskToGrid(task)
		}
	} else if task.Rect.Width != taskDisplaySize.X || task.Rect.Height != taskDisplaySize.Y {
		task.Rect.Width = taskDisplaySize.X
		task.Rect.Height = taskDisplaySize.Y
		// We need to update the Task's position list because it changes here
		task.Board.RemoveTaskFromGrid(task)
		task.Board.AddTaskToGrid(task)
	}

	color := getThemeColor(GUI_INSIDE)

	if task.Is(TASK_TYPE_NOTE) {
		color = getThemeColor(GUI_NOTE_COLOR)
	}

	outlineColor := getThemeColor(GUI_OUTLINE)

	if task.Selected {
		outlineColor = getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
	}

	perc := float32(0)

	if perc > 1 {
		perc = 1
	}

	task.PercentageComplete += (perc - task.PercentageComplete) * 0.1

	// Raising these "margins" because sounds can be longer, and so 3 seconds into a 5 minute song might would be 1%, or 0.01.
	if task.PercentageComplete < 0.0001 {
		task.PercentageComplete = 0
	} else if task.PercentageComplete >= 0.9999 {
		task.PercentageComplete = 1
	}

	alpha := uint8(255)

	if task.Board.Project.TaskTransparency.Number() < 5 {
		t := float32(task.Board.Project.TaskTransparency.Number())
		alpha = uint8((t / float32(task.Board.Project.TaskTransparency.Maximum)) * (255 - 32))
		color.A = 32 + alpha
	}

	if task.Is(TASK_TYPE_IMAGE) {

		if task.Image.ID != 0 {

			src := rl.Rectangle{0, 0, float32(task.Image.Width), float32(task.Image.Height)}
			dst := task.Rect
			dst.Width = taskDisplaySize.X
			dst.Height = taskDisplaySize.Y
			rl.SetTextureFilter(task.Image, rl.FilterAnisotropic4x)
			color := rl.White
			if task.Board.Project.GraphicalTasksTransparent.Checked {
				color.A = alpha
			}
			rl.DrawTexturePro(task.Image, src, dst, rl.Vector2{}, 0, color)
		}
	}

	if task.Resizeable() && task.Selected && (!task.Is(TASK_TYPE_IMAGE) || task.Image.ID > 0) {
		// Only valid images or other resizeable Task Types can be resized

		selectedTaskCount := len(task.Board.SelectedTasks(false))

		if selectedTaskCount == 1 {

			rl.DrawRectangleRec(task.ResizeRect, getThemeColor(GUI_INSIDE))
			rl.DrawRectangleLinesEx(task.ResizeRect, 1, getThemeColor(GUI_FONT_COLOR))

			if task.Is(TASK_TYPE_IMAGE) {
				rl.DrawRectangleRec(task.ImageSizeResetRect, getThemeColor(GUI_INSIDE))
				rl.DrawRectangleLinesEx(task.ImageSizeResetRect, 1, getThemeColor(GUI_FONT_COLOR))
			}

		}

	}

	if task.Board.Project.OutlineTasks.Checked && !task.Is(TASK_TYPE_LINE) {
		rl.DrawRectangleLinesEx(task.Rect, 1, outlineColor)
	}
	if !task.Is(TASK_TYPE_IMAGE, TASK_TYPE_LINE) {

		textPos := rl.Vector2{task.Rect.X + 2, task.Rect.Y + 2}

		if task.Board.Project.ShowIcons.Checked {
			textPos.X += 16
		}

		DrawText(textPos, name)

		if !task.Board.Project.TaskOpen && !task.Board.Project.Searchbar.Focused && !task.Board.Project.ProjectSettingsOpen && task.Board.Project.PopupAction == "" && (task.Is(TASK_TYPE_NOTE)) {

			if name != task.DisplayedText {
				task.ScanTextForURLs(name)
				task.DisplayedText = name
			}

			worldGUI = true

			for _, urlButton := range task.URLButtons {

				if programSettings.Keybindings.On(KBURLButton) || task.Board.Project.AlwaysShowURLButtons.Checked {

					margin := float32(2)
					dst := rl.Rectangle{textPos.X + urlButton.Pos.X - margin, textPos.Y + urlButton.Pos.Y, urlButton.Size.X + (margin * 2), urlButton.Size.Y}
					if ImmediateButton(dst, urlButton.Text, false) {
						browser.OpenURL(urlButton.Link)
					}

				}

			}

			worldGUI = false

		}

	}

	if task.Board.Project.ShowIcons.Checked {

		iconColor := getThemeColor(GUI_FONT_COLOR)
		iconSrc := rl.Rectangle{16, 0, 16, 16}
		rotation := float32(0)

		iconSrcIconPositions := map[int][]float32{
			TASK_TYPE_NOTE:        {64, 0},
			TASK_TYPE_IMAGE:       {96, 0},
			TASK_TYPE_LINE:        {128, 32},
		}

		iconSrc.X = iconSrcIconPositions[task.TaskType.CurrentChoice][0]
		iconSrc.Y = iconSrcIconPositions[task.TaskType.CurrentChoice][1]

		// task.ArrowPointingToTask = nil

		if task.Is(TASK_TYPE_LINE) && task.LineBase != nil {

			iconSrc.X = 144
			iconSrc.Y = 32
			rotation = rl.Vector2Angle(task.LineBase.Position, task.Position)
		}

		if !task.Is(TASK_TYPE_IMAGE) || invalidImage {
			if task.Is(TASK_TYPE_LINE) {
				if task.Board.Project.OutlineTasks.Checked {
					rl.DrawTexturePro(task.Board.Project.GUI_Icons, iconSrc, rl.Rectangle{task.Rect.X + 8, task.Rect.Y + 8, 16, 16}, rl.Vector2{8, 8}, rotation, getThemeColor(GUI_INSIDE))
				}
				iconSrc.Y += 16
			}
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, iconSrc, rl.Rectangle{task.Rect.X + 8, task.Rect.Y + 8, 16, 16}, rl.Vector2{8, 8}, rotation, iconColor)
		}

		if extendedText {
			// The "..." at the end of a Task.
			iconSrc.X = 112
			iconSrc.Y = 0
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, iconSrc, rl.Rectangle{task.Rect.X + taskDisplaySize.X - 16, task.Rect.Y, 16, 16}, rl.Vector2{}, 0, iconColor)
		}
	}

	if task.Selected && task.Board.Project.PulsingTaskSelection.Checked { // Drawing selection indicator
		r := task.Rect
		t := float32(math.Sin(float64(rl.GetTime()-(float32(task.ID)*0.1))*math.Pi*4))/2 + 0.5
		f := t * 4

		margin := float32(2)

		r.X -= f + margin
		r.Y -= f + margin
		r.Width += (f + 1 + margin) * 2
		r.Height += (f + 1 + margin) * 2

		r.X = float32(int32(r.X))
		r.Y = float32(int32(r.Y))
		r.Width = float32(int32(r.Width))
		r.Height = float32(int32(r.Height))

		c := getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
		end := getThemeColor(GUI_OUTLINE_DISABLED)

		changeR := ease.Linear(t, float32(end.R), float32(c.R)-float32(end.R), 1)
		changeG := ease.Linear(t, float32(end.G), float32(c.G)-float32(end.G), 1)
		changeB := ease.Linear(t, float32(end.B), float32(c.B)-float32(end.B), 1)

		c.R = uint8(changeR)
		c.G = uint8(changeG)
		c.B = uint8(changeB)

		rl.DrawRectangleLinesEx(r, 2, c)
	}

}

func (task *Task) Depth() int {

	depth := 0

  if task.Is(TASK_TYPE_LINE) {
		depth = 100
	}

	return depth
}

func (task *Task) PostDraw() {

	if task.Open {

		column := task.EditPanel.Columns[0]

		column.Mode = task.TaskType.CurrentChoice

		task.CreationLabel.Text = task.CreationTime.Format("Monday, Jan 2, 2006, 15:04")

		task.EditPanel.Update()

		if task.LoadMediaButton.Clicked {

			filepath := ""
			var err error

			if task.Is(TASK_TYPE_IMAGE) {

				filepath, err = zenity.SelectFile(zenity.Title("Select image file"), zenity.FileFilters{zenity.FileFilter{Name: "Image File", Patterns: []string{
					"*.png",
					"*.bmp",
					"*.jpeg",
					"*.jpg",
					"*.gif",
					"*.dds",
					"*.hdr",
					"*.ktx",
					"*.astc",
				}}})

			} else {

				filepath, err = zenity.SelectFile(zenity.Title("Select sound file"), zenity.FileFilters{zenity.FileFilter{Name: "Sound File", Patterns: []string{
					"*.wav",
					"*.ogg",
					"*.flac",
					"*.mp3",
				}}})

			}

			if err == nil && filepath != "" {
				task.FilePathTextbox.SetText(filepath)
			}

		}

		if task.EditPanel.Exited {
			task.ReceiveMessage(MessageTaskClose, nil)
		}

	}

}

func (task *Task) Resizeable() bool {
	return task.Is(TASK_TYPE_IMAGE)
}

func (task *Task) LoadResource() {

	task.SuccessfullyLoadedResourceOnce = false

	if task.FilePathTextbox.Text() != "" {

		res, _ := task.Board.Project.LoadResource(task.FilePathTextbox.Text())

		if res != nil {

			task.SuccessfullyLoadedResourceOnce = true

			if task.Is(TASK_TYPE_IMAGE) {

				if res.IsTexture() {

					task.Image = res.Texture()
					if task.PrevFilePath != task.FilePathTextbox.Text() && task.DisplaySize.X == 0 && task.DisplaySize.Y == 0 {
						task.DisplaySize.X = float32(task.Image.Width)
						task.DisplaySize.Y = float32(task.Image.Height)
					}
				}
      }

			task.PrevFilePath = task.FilePathTextbox.Text()
		}
	}
}

func (task *Task) ReceiveMessage(message string, data map[string]interface{}) {

	// This exists because Line type Tasks should have an ending, either after
	// creation, or after setting the type and closing
	createAtLeastOneLineEnding := func() {
		if task.Is(TASK_TYPE_LINE) && len(task.ValidLineEndings()) == 0 {
			prevUndoOn := task.Board.UndoBuffer.On
			task.Board.UndoBuffer.On = false
			task.CreateLineEnding()
			task.Board.UndoBuffer.On = prevUndoOn
		}
	}

	if message == MessageSelect {

		if data["task"] == task {
			if data["invert"] != nil {
				task.Selected = false
			} else {
				task.Selected = true
			}
		} else if data["task"] == nil || data["task"] != task {
			task.Selected = false
		}

	} else if message == MessageDoubleClick {

		if task.LineBase != nil {
			task.LineBase.ReceiveMessage(MessageDoubleClick, nil)
    } else {

      // We have to consume after double-clicking so you don't click outside of the new panel and exit it immediately
      // or actuate a GUI element accidentally. HOWEVER, we want it here because double-clicking might not actually
      // open the Task, as can be seen here
      ConsumeMouseInput(rl.MouseLeftButton)

      task.Open = true
      task.Board.Project.TaskOpen = true
      task.Dragging = false
      task.Description.Focused = true

      if task.Board.Project.TaskEditRect.Width != 0 && task.Board.Project.TaskEditRect.Height != 0 {
        task.EditPanel.Rect = task.Board.Project.TaskEditRect
      }

      createAtLeastOneLineEnding()
      task.Board.UndoBuffer.Capture(task)
    }
	} else if message == MessageTaskClose {

		if task.Open {

			task.Board.Project.TaskEditRect = task.EditPanel.Rect

			task.Open = false
			task.Board.Project.TaskOpen = false
			task.LoadResource()
			task.Board.Project.PreviousTaskType = task.TaskType.ChoiceAsString()


			if !task.Is(TASK_TYPE_LINE) {
				for _, ending := range task.ValidLineEndings() {
					// Delete your endings if you're no longer a Line Task
					task.Board.DeleteTask(ending)
				}
			}

			// We call ReorderTasks here because changing the Task can change its Rect,
			// thereby changing its neighbors.
			task.Board.ReorderTasks()
			createAtLeastOneLineEnding()
			task.Board.UndoBuffer.Capture(task)

		}
	} else if message == MessageDragging {
		if task.Selected {
			if !task.Dragging {
				task.Board.UndoBuffer.Capture(task) // Just started dragging
			}
			task.Dragging = true
			task.MouseDragStart = GetWorldMousePosition()
			task.TaskDragStart = task.Position
		}
	} else if message == MessageDropped {
		task.Dragging = false
		if task.Valid {
			// This gets called when we reorder the board / project, which can cause problems if the Task is already removed
			// because it will then be immediately readded to the Board grid, thereby making it a "ghost" Task
			task.Position = task.Board.Project.LockPositionToGrid(task.Position)
			task.Board.RemoveTaskFromGrid(task)
			task.Board.AddTaskToGrid(task)

			if !task.Board.Project.JustLoaded {
				task.Board.UndoBuffer.Capture(task)
			}

			// Delete your endings if you're no longer a Line Task
			if !task.Is(TASK_TYPE_LINE) {
				for _, ending := range task.ValidLineEndings() {
					task.Board.DeleteTask(ending)
				}
			}

		}
	} else if message == MessageDelete {

		// We remove the Task from the grid but not change the GridPositions list because undos need to
		// re-place the Task at the original position.
		task.Board.RemoveTaskFromGrid(task)

		if task.LineBase == nil {
			if len(task.ValidLineEndings()) > 0 {
				for _, ending := range task.ValidLineEndings() {
					task.Board.DeleteTask(ending)
				}
			}
		} else if task.LineBase.Is(TASK_TYPE_LINE) {
			// task.LineBase implicity is not nil here, indicating that this is a line ending
			if len(task.LineBase.ValidLineEndings()) == 0 {
				task.Board.DeleteTask(task.LineBase)
			}
		}

	} else if message == MessageThemeChange {
	} else {
		fmt.Println("UNKNOWN MESSAGE: ", message)
	}

}

func (task *Task) ScanTextForURLs(text string) {

	task.URLButtons = []URLButton{}

	currentURLButton := URLButton{}
	wordStart := rl.Vector2{}

	for i, letter := range []rune(text) {

		validRune := true

		if i < len([]rune(task.PrefixText)) { // The numbering prefix cannot be part of the URL
			validRune = false
		}

		if letter != ' ' && letter != '\n' {
			if validRune {
				currentURLButton.Text += string(letter)
			}
			wordStart.X += rl.MeasureTextEx(font, string(letter), float32(programSettings.FontSize), spacing).X + 1
		}

		if letter == ' ' || letter == '\n' || i == len(text)-1 {

			if len(currentURLButton.Text) > 0 {
				currentURLButton.Size.X = rl.MeasureTextEx(font, currentURLButton.Text, float32(programSettings.FontSize), spacing).X
				currentURLButton.Size.Y, _ = TextHeight("A", false)

				urlText := strings.Trim(strings.Trim(strings.TrimSpace(currentURLButton.Text), "."), ":")

				if strings.Contains(urlText, ".") || strings.Contains(urlText, ":") {

					if url, err := urlx.Parse(urlText); err == nil && url.Host != "" && url.Scheme != "" {
						currentURLButton.Link = url.String()
						task.URLButtons = append(task.URLButtons, currentURLButton)
					}

				}

			}

			if letter == '\n' {
				height, _ := TextHeight("A", false)
				wordStart.Y += height
				wordStart.X = 0
			} else if letter == ' ' {
				wordStart.X += rl.MeasureTextEx(font, " ", float32(programSettings.FontSize), spacing).X + 1
			}

			currentURLButton = URLButton{}
			currentURLButton.Pos = wordStart

		}

	}

}

func (task *Task) ValidLineEndings() []*Task {
	endings := []*Task{}
	for _, ending := range task.LineEndings {
		if ending.Valid {
			endings = append(endings, ending)
		}
	}

	return endings
}

func (task *Task) CreateLineEnding() *Task {

	if task.Is(TASK_TYPE_LINE) && task.LineBase == nil {

		prevUndoOn := task.Board.UndoBuffer.On
		task.Board.UndoBuffer.On = false
		ending := task.Board.CreateNewTask()
		task.Board.UndoBuffer.On = prevUndoOn
		ending.TaskType.CurrentChoice = TASK_TYPE_LINE
		ending.Position = task.Position
		ending.Position.X += float32(task.Board.Project.GridSize) * 2
		ending.Rect.X = ending.Position.X
		ending.Rect.Y = ending.Position.Y
		task.LineEndings = append(task.LineEndings, ending)
		ending.LineBase = task

		return ending
	}
	return nil

}

func (task *Task) SmallButton(srcX, srcY, srcW, srcH, dstX, dstY float32) bool {

	dstRect := rl.Rectangle{dstX, dstY, srcW, srcH}

	rl.DrawTexturePro(
		task.Board.Project.GUI_Icons,
		rl.Rectangle{srcX, srcY, srcW, srcH},
		dstRect,
		rl.Vector2{},
		0,
		getThemeColor(GUI_FONT_COLOR))
	// getThemeColor(GUI_INSIDE_HIGHLIGHTED))

	return task.Selected && rl.CheckCollisionPointRec(GetWorldMousePosition(), dstRect) && MousePressed(rl.MouseLeftButton)

}

// Move moves the Task while checking to ensure it doesn't overlap with another Task in that position.
func (task *Task) Move(dx, dy float32) {

	if dx == 0 && dy == 0 {
		return
	}

	gs := float32(task.Board.Project.GridSize)

	free := false

	for !free {

		tasksInRect := task.Board.GetTasksInRect(task.Position.X+dx, task.Position.Y+dy, task.Rect.Width, task.Rect.Height)

		if len(tasksInRect) == 0 || (len(tasksInRect) == 1 && tasksInRect[0] == task) {
			task.Position.X += dx
			task.Position.Y += dy
			free = true
			break
		}

		if dx > 0 {
			dx += gs
		} else if dx < 0 {
			dx -= gs
		}

		if dy > 0 {
			dy += gs
		} else if dy < 0 {
			dy -= gs
		}

	}

}

func (task *Task) Destroy() {

	if task.LineBase != nil && task.LineBase.Is(TASK_TYPE_LINE) {

		for i, t := range task.LineBase.LineEndings {
			if t == task {
				task.LineBase.LineEndings[i] = nil
				task.LineBase.LineEndings = append(task.LineBase.LineEndings[:i], task.LineBase.LineEndings[i+1:]...)
			}
		}

	}
}

func (task *Task) UsesMedia() bool {
	return task.Is(TASK_TYPE_IMAGE)
}

func (task *Task) Is(taskTypes ...int) bool {
	for _, taskType := range taskTypes {
		if task.TaskType.CurrentChoice == taskType {
			return true
		}
	}
	return false
}
