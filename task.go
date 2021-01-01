package main

import (
  "fmt"
  "math"
  "path/filepath"
  "strings"
  "time"

  //"github.com/goware/urlx"
  //"github.com/ncruces/zenity"
  "github.com/tidwall/gjson"
  "github.com/tidwall/sjson"

  rl "github.com/gen2brain/raylib-go/raylib"
)

const (
  TASK_TYPE_NOTE = "note"
  TASK_TYPE_IMAGE = "image"
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

  TaskType string
  Description string

  CreationTime   time.Time

  TextSize float32

  Image                        rl.Texture2D

  FilePath string
  PrevFilePath  string
  DisplaySize        rl.Vector2
  Resizing           bool
  Dragging           bool
  MouseDragStart     rl.Vector2
  TaskDragStart      rl.Vector2
  ResizeRect         rl.Rectangle
  ImageSizeResetRect rl.Rectangle

  OriginalIndentation int
  PrefixText          string
  ID                  int
  Visible             bool

  GridPositions []Position

  SuccessfullyLoadedResourceOnce bool
}

func NewTask(board *Board) *Task {

  //postX := float32(180)

  task := &Task{
    Rect: rl.Rectangle{0, 0, 16, 16},
    Board: board,
    TaskType: TASK_TYPE_IMAGE,
    Description: "",
    ID: board.Project.FirstFreeID(),
    FilePath: "",
    GridPositions: []Position{},
    CreationTime: time.Now(),
  }

  task.MinSize = rl.Vector2{task.Rect.Width, task.Rect.Height}
  task.MaxSize = rl.Vector2{0, 0}

  return task
}

func (task *Task) Clone() *Task {
  copyData := *task
  copyData.PrevFilePath = ""

  copyData.ID = copyData.Board.Project.FirstFreeID()

  copyData.ReceiveMessage(MessageTaskClose, nil) // We do this to recreate the resources for the Task, if necessary.

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

  jsonData, _ = sjson.Set(jsonData, `Description`, task.Description)

  if task.UsesMedia() && task.FilePath != "" {

    resourcePath := task.FilePath

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

  jsonData, _ = sjson.Set(jsonData, `TaskType\.CurrentChoice`, task.TaskType)

  jsonData, _ = sjson.Set(jsonData, `CreationTime`, task.CreationTime.Format(`Jan 2 2006 15:04:05`))

  if task.Is(TASK_TYPE_NOTE) {
    jsonData, _ = sjson.Set(jsonData, `TextSize`, task.TextSize)
  }

  return jsonData

}

// Serializable returns if Tasks are able to be serialized properly. Only line endings aren't properly serializeable
func (task *Task) Serializable() bool {
  return true
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

  //hasData := func(name string) bool {
  //  return gjson.Get(jsonData, name).Exists()
  //}

  task.Position.X = getFloat(`Position\.X`)
  task.Position.Y = getFloat(`Position\.Y`)

  task.Rect.X = task.Position.X
  task.Rect.Y = task.Position.Y

  if gjson.Get(jsonData, `ImageDisplaySize\.X`).Exists() {
    task.DisplaySize.X = getFloat(`ImageDisplaySize\.X`)
    task.DisplaySize.Y = getFloat(`ImageDisplaySize\.Y`)
  }

  task.Description = getString(`Description`)

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

      task.FilePath = abs
    } else {
      task.FilePath = getString(`FilePath`)
    }

  }

  task.Selected = getBool(`Selected`)
  task.TaskType = getString(`TaskType\.CurrentChoice`)

  creationTime, err := time.Parse(`Jan 2 2006 15:04:05`, getString(`CreationTime`))
  if err == nil {
    task.CreationTime = creationTime
  }

  if task.TaskType == TASK_TYPE_NOTE {
    task.TextSize = getFloat(`TextSize`)
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

      if task.Is(TASK_TYPE_IMAGE, TASK_TYPE_NOTE) {

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

    switch taskType := task.TaskType ; taskType {

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

func (task *Task) Draw() {

  if !task.Visible {
    return
  }

  name := task.Description

  //extendedText := false

  taskType := task.TaskType

  switch taskType {

  case TASK_TYPE_IMAGE:
    _, filename := filepath.Split(task.FilePath)
    name = filename
  }

  invalidImage := task.Image.ID == 0 
  if !invalidImage && task.Is(TASK_TYPE_IMAGE) {
    name = ""
  }

  taskDisplaySize := task.DisplaySize

  // NOTE(justasd): :Text
  /*
  if task.Is(TASK_TYPE_NOTE) {

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
  */

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

  //color := getThemeColor(GUI_INSIDE)

  if task.Is(TASK_TYPE_NOTE) {
    //color = getThemeColor(GUI_NOTE_COLOR)
  }

  //outlineColor := getThemeColor(GUI_OUTLINE)

  if task.Selected {
    //outlineColor = getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
  }

  perc := float32(0)

  if perc > 1 {
    perc = 1
  }

  //alpha := uint8(255)

  if task.Is(TASK_TYPE_IMAGE) {

    if task.Image.ID != 0 {

      src := rl.Rectangle{0, 0, float32(task.Image.Width), float32(task.Image.Height)}
      dst := task.Rect
      dst.Width = taskDisplaySize.X
      dst.Height = taskDisplaySize.Y
      rl.SetTextureFilter(task.Image, rl.FilterAnisotropic4x)
      color := rl.White
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

  if !task.Is(TASK_TYPE_IMAGE) {

    textPos := rl.Vector2{task.Rect.X + 2, task.Rect.Y + 2}

    // NOTE(justasd): :Text
    //DrawText(textPos, name)

    {
      text := name
      pos := textPos

      pos.Y -= 2 // Text is a bit low

      size := float32(128.0)

      //height, lineCount := TextHeight(text, guiMode)

      pos.X = float32(int32(pos.X))
      pos.Y = float32(int32(pos.Y))

      rl.DrawTextEx(not_shit_font, text, pos, size, spacing, rl.RayWhite)

      //for _, line := range strings.Split(text, "\n") {
      //rl.DrawTextEx(not_shit_font, line, pos, size, spacing, fontColor)
      //pos.Y += float32(int32(height / float32(lineCount)))
      //}
    }
  }
}

func (task *Task) Depth() int {

  depth := 0

  return depth
}

func (task *Task) PostDraw() {

  // NOTE(justasd): :Text
  /*
  if task.Open {

    column := task.EditPanel.Columns[0]

    column.Mode = task.TaskType

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
      }

      if err == nil && filepath != "" {
        task.FilePath.SetText(filepath)
      }
    }

    if task.EditPanel.Exited {
      task.ReceiveMessage(MessageTaskClose, nil)
    }
  }
  */
}

func (task *Task) Resizeable() bool {
  return task.Is(TASK_TYPE_IMAGE, TASK_TYPE_NOTE)
}

func (task *Task) LoadResource() {

  task.SuccessfullyLoadedResourceOnce = false

  if task.FilePath != "" {

    res, _ := task.Board.Project.LoadResource(task.FilePath)

    if res != nil {

      task.SuccessfullyLoadedResourceOnce = true

      if task.Is(TASK_TYPE_IMAGE) {

        if res.IsTexture() {

          task.Image = res.Texture()
          if task.PrevFilePath != task.FilePath && task.DisplaySize.X == 0 && task.DisplaySize.Y == 0 {
            task.DisplaySize.X = float32(task.Image.Width)
            task.DisplaySize.Y = float32(task.Image.Height)
          }
        }
      }

      task.PrevFilePath = task.FilePath
    }
  }
}

func (task *Task) ReceiveMessage(message string, data map[string]interface{}) {

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

    // We have to consume after double-clicking so you don't click outside of the new panel and exit it immediately
    // or actuate a GUI element accidentally. HOWEVER, we want it here because double-clicking might not actually
    // open the Task, as can be seen here
    ConsumeMouseInput(rl.MouseLeftButton)

    task.Open = true
    task.Board.Project.TaskOpen = true
    task.Dragging = false
  } else if message == MessageTaskClose {

    if task.Open {

      task.Open = false
      task.Board.Project.TaskOpen = false
      task.LoadResource()
      task.Board.Project.PreviousTaskType = task.TaskType

      // We call ReorderTasks here because changing the Task can change its Rect,
      // thereby changing its neighbors.
      task.Board.ReorderTasks()

    }
  } else if message == MessageDragging {
    if task.Selected {
      task.Dragging = true
      task.MouseDragStart = GetWorldMousePosition()
      task.TaskDragStart = task.Position
    }
  } else if message == MessageDropped {
    task.Dragging = false
    // This gets called when we reorder the board / project, which can cause problems if the Task is already removed
    // because it will then be immediately readded to the Board grid, thereby making it a "ghost" Task
    task.Position = task.Board.Project.LockPositionToGrid(task.Position)
    task.Board.RemoveTaskFromGrid(task)
    task.Board.AddTaskToGrid(task)
  } else if message == MessageDelete {

    // We remove the Task from the grid but not change the GridPositions list because undos need to
    // re-place the Task at the original position.
    task.Board.RemoveTaskFromGrid(task)

  } else if message == MessageThemeChange {
  } else {
    fmt.Println("UNKNOWN MESSAGE: ", message)
  }

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
}

func (task *Task) UsesMedia() bool {
  return task.Is(TASK_TYPE_IMAGE)
}

func (task *Task) Is(taskTypes ...string) bool {
  for _, taskType := range taskTypes {
    if task.TaskType == taskType {
      return true
    }
  }
  return false
}
