package main

import (
	"fmt"
	"image/gif"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goware/urlx"
	"github.com/tanema/gween"
	"github.com/tanema/gween/ease"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/ncruces/zenity"

	"github.com/gabriel-vasile/mimetype"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (

	// Task messages
	MessageDelete      = "delete"
	MessageSelect      = "select"
	MessageDropped     = "dropped"
	MessageDoubleClick = "double click"
	MessageDragging    = "dragging"
	MessageTaskClose   = "task close"
	MessageThemeChange = "theme change"

	// Project actions

	ActionNewProject    = "new"
	ActionLoadProject   = "load"
	ActionSaveAsProject = "save as"
	ActionRenameBoard   = "rename"
	ActionQuit          = "quit"

	BackupDelineator = "_bak_"
)

var firstFreeTaskID = 0

type Project struct {

	// Internal data to make stuff work
	FilePath            string
	GridSize            int32
	Boards              []*Board
	BoardIndex          int
	BoardPanel          rl.Rectangle
  Zoom                float32
	CameraPan           rl.Vector2
	CameraOffset        rl.Vector2
	FullyInitialized    bool
	ContextMenuOpen     bool
	ContextMenuPosition rl.Vector2
	ProjectSettingsOpen bool
	Selecting           bool
	SelectionStart      rl.Vector2
	DoubleClickTimer    time.Time
	DoubleClickTaskID   int
	CopyBuffer          []*Task
	Cutting             bool // If cutting, then this boolean is set
	TaskOpen            bool
	JustLoaded          bool
	ResizingImage       bool
	LogOn               bool

	ShortcutKeyTimer  int
	PreviousTaskType  string
	Resources         map[string]*Resource
	Modified          bool

	UndoFade      *gween.Sequence
	Undoing       int
	TaskEditRect  rl.Rectangle
}

func NewProject() *Project {

	project := &Project{
		FilePath: "",
		GridSize: 16,
    Zoom: 1.0,
		CameraPan: rl.Vector2{0, 0},
		Resources: map[string]*Resource{},
	}

  project.Boards = []*Board{NewBoard(project)}

	return project
}

func (project *Project) CurrentBoard() *Board {
	return project.Boards[project.BoardIndex]
}

func (project *Project) GetAllTasks() []*Task {
	tasks := []*Task{}
	for _, b := range project.Boards {
		tasks = append(tasks, b.Tasks...)
	}
	return tasks
}

func (project *Project) SaveAs() {

	if savePath, err := zenity.SelectFileSave(
		zenity.Title("Select a location and name to save the Project."),
		zenity.ConfirmOverwrite(),
		zenity.FileFilters{{Name: ".plan", Patterns: []string{"*.plan"}}}); err == nil && savePath != "" {

		if filepath.Ext(savePath) != ".plan" {
			savePath += ".plan"
		}

		project.ExecuteDestructiveAction(ActionSaveAsProject, savePath)

	}

}

func (project *Project) Save(backup bool) {

	success := true

  if project.FilePath != "" {

    // Sort the Tasks by their ID, then loop through them using that slice. This way,
    // They store data according to their creation ID, not according to their position
    // in the world.
    tasksByID := append([]*Task{}, project.GetAllTasks()...)

    sort.Slice(tasksByID, func(i, j int) bool { return tasksByID[i].ID < tasksByID[j].ID })

    // We're passing in actual JSON strings for task serlizations, so we have to actually construct the
    // string containing our JSON array of tasks ourselves.
    taskData := "["
    firstTask := true
    for _, task := range tasksByID {
      if firstTask {
        firstTask = false
      } else {
        taskData += ","
      }
      if task.Serializable() {
        taskData += task.Serialize()
      }
    }
    taskData += "]"

    data := `{}`

    // Not handling any of these errors because uuuuuuuuuh idkkkkkk should there ever really be errors
    // with a blank JSON {} object????
    data, _ = sjson.Set(data, `Version`, softwareVersion.String())
    data, _ = sjson.Set(data, `BoardIndex`, project.BoardIndex)
    data, _ = sjson.Set(data, `BoardCount`, len(project.Boards))
    data, _ = sjson.Set(data, `Pan\.X`, project.CameraPan.X)
    data, _ = sjson.Set(data, `Pan\.Y`, project.CameraPan.Y)
    data, _ = sjson.Set(data, `Zoom`, project.Zoom)
    data, _ = sjson.Set(data, `ColorTheme`, currentTheme)
    data, _ = sjson.Set(data, `GridSize`, project.GridSize)

    boardNames := []string{}
    for _, board := range project.Boards {
      boardNames = append(boardNames, board.Name)
    }
    data, _ = sjson.Set(data, `BoardNames`, boardNames)

    data, _ = sjson.SetRaw(data, `Tasks`, taskData) // taskData is already properly encoded and formatted JSON

    f, err := os.Create(project.FilePath)
    if err != nil {
      project.Log("Error in creating save file: ", err.Error())
    } else {
      defer f.Close()

      data = gjson.Parse(data).Get("@pretty").String() // Pretty print it so it's visually nice in the .plan file.

      f.Write([]byte(data))
      programSettings.Save()

      err = f.Sync() // Want to make sure the file is written
      if err != nil {
        project.Log("ERROR: Can't write file to system: ", err.Error())
        success = false
      }

    }

  } else {
    success = false
  }

  if success {
    if !backup {
      // Modified flag only gets cleared on manual saves, not automatic backups
      project.Modified = false
    }
  } else {
    project.Log("ERROR: Save / backup unsuccessful.")
  }

}

func LoadProjectFrom() *Project {

	// I used to have the extension for this file selector set to "*.plan", but Mac doesn't seem to recognize
	// MasterPlan's .plan files as having that extension, using both dlgs and zenity. Not sure why; filters work when loading
	// files. Maybe because .plan files don't have some kind of metadata that identifies them on Mac? Maybe I should just make them
	// JSON files; that's what they are, anyway...

	if file, err := zenity.SelectFile(zenity.Title("Select MasterPlan Project File")); err == nil && file != "" {
		if loadedProject := LoadProject(file); loadedProject != nil {
			return loadedProject
		}
	}

	return nil

}

func LoadProject(filepath string) *Project {

	project := NewProject()

	if fileData, err := ioutil.ReadFile(filepath); err == nil {

		data := gjson.Parse(string(fileData))

		if data.Get("Tasks").Exists() {

			project.JustLoaded = true

			if strings.Contains(filepath, BackupDelineator) {
				project.FilePath = strings.Split(filepath, BackupDelineator)[0]
			} else {
				project.FilePath = filepath
			}

			getFloat := func(name string) float32 {
				return float32(data.Get(name).Float())
			}

			getInt := func(name string) int {
				return int(data.Get(name).Int())
			}

			//getString := func(name string) string {
			//	return data.Get(name).String()
			//}

			//getBool := func(name string) bool {
			//	return data.Get(name).Bool()
			//}

			project.GridSize = int32(getInt(`GridSize`))
			project.CameraPan.X = getFloat(`Pan\.X`)
			project.CameraPan.Y = getFloat(`Pan\.Y`)
			project.Zoom = getFloat(`Zoom`)

			project.LogOn = false

			boardNames := []string{}
			for _, name := range data.Get(`BoardNames`).Array() {
				boardNames = append(boardNames, name.String())
			}

			for i := 0; i < getInt(`BoardCount`)-1; i++ {
				project.AddBoard()
			}

			for i := range project.Boards {
				if i < len(boardNames) {
					project.Boards[i].Name = boardNames[i]
				}
			}

			for _, taskData := range data.Get(`Tasks`).Array() {

				boardIndex := 0

				if taskData.Get(`BoardIndex`).Exists() {
					boardIndex = int(taskData.Get(`BoardIndex`).Int())
				}

				task := project.Boards[boardIndex].CreateNewTask()
				task.Deserialize(taskData.String())
			}

			project.LogOn = true

			list := []string{}

			existsInList := func(value string) bool {
				for _, item := range list {
					if value == item {
						return true
					}
				}
				return false
			}

			lastOpenedIndex := -1
			i := 0
			for _, s := range programSettings.RecentPlanList {
				_, err := os.Stat(s)
				if err == nil && !existsInList(s) {
					// If err != nil, the file must not exist, so we'll skip it
					list = append(list, s)
					if s == filepath {
						lastOpenedIndex = i
					}
					i++
				}
			}

			if lastOpenedIndex > 0 {

				// If the project to be opened is already in the recent files list, then we can just bump it up to the front.

				// ABC <- Say we want to move B to the front.

				// list = ABC_
				list = append(list, "")

				// list = AABC
				copy(list[1:], list[0:])

				// list = BABC
				list[0] = list[lastOpenedIndex+1] // Index needs to be +1 here because we made the list 1 larger above

				// list = BAC
				list = append(list[:lastOpenedIndex+1], list[lastOpenedIndex+2:]...)

			} else if lastOpenedIndex < 0 {
				list = append([]string{filepath}, list...)
			}

			programSettings.RecentPlanList = list

			programSettings.Save()

			return project

		}

	}

	// It's possible for the file to be mangled and unable to be loaded; I should actually handle this
	// with a backup system or something.

	// We log on the current project because this project didn't load correctly

	currentProject.Log("Error: Could not load plan:\n[ %s ].", filepath)

	return nil

}

func (project *Project) Log(text string, variables ...interface{}) {

	if len(variables) > 0 {
		text = fmt.Sprintf(text, variables...)
	}

	if project.LogOn {
		eventLogBuffer = append(eventLogBuffer, EventLog{time.Now(), text, gween.New(255, 0, 7, ease.InExpo)})
	}

	log.Println(text)
}

func (project *Project) HandleCamera() {

	wheel := rl.GetMouseWheelMove()


  if !project.TaskOpen && !project.ProjectSettingsOpen {
    zoom_dx := float32(0.0)

		if wheel > 0 {
      zoom_dx = 1.0
		} else if wheel < 0 {
      zoom_dx = -1.0
		}

    if zoom_dx != 0 {
      project.Zoom += project.Zoom * 0.1 * zoom_dx;
    }
	}

  if project.Zoom < 0.0001 {
    project.Zoom = 0.0001
  }

	camera.Zoom = project.Zoom

	if MouseDown(rl.MouseMiddleButton) {
		diff := GetMouseDelta()
		project.CameraPan.X += diff.X
		project.CameraPan.Y += diff.Y
	}

	project.CameraOffset.X += float32(project.CameraPan.X-project.CameraOffset.X)
	project.CameraOffset.Y += float32(project.CameraPan.Y-project.CameraOffset.Y)

	camera.Target.X = float32(-project.CameraOffset.X)
	camera.Target.Y = float32(-project.CameraOffset.Y)

	camera.Offset.X = float32(rl.GetScreenWidth() / 2)
	camera.Offset.Y = float32(rl.GetScreenHeight() / 2)
}

func (project *Project) MousingOver() string {

	if rl.CheckCollisionPointRec(GetMousePosition(), project.BoardPanel) {
		return "Boards"
	} else if project.TaskOpen {
		return "TaskOpen"
	} else {
		return "Project"
	}

}

func (project *Project) Update() {

	addToSelection := programSettings.Keybindings.On(KBAddToSelection)
	removeFromSelection := programSettings.Keybindings.On(KBRemoveFromSelection)

	// This is the origin crosshair
	rl.DrawLineEx(rl.Vector2{0, -100000}, rl.Vector2{0, 100000}, 2, getThemeColor(GUI_INSIDE))
	rl.DrawLineEx(rl.Vector2{-100000, 0}, rl.Vector2{100000, 0}, 2, getThemeColor(GUI_INSIDE))

	selectionRect := rl.Rectangle{}

	for _, task := range project.GetAllTasks() {
		task.Update()
	}

	sorted := append([]*Task{}, project.CurrentBoard().Tasks...)

	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Depth() == sorted[j].Depth() {
			if sorted[i].Rect.Y == sorted[j].Rect.Y {
				return sorted[i].Rect.X < sorted[j].Rect.X
			}
			return sorted[i].Rect.Y < sorted[j].Rect.Y
		}
		return sorted[i].Depth() < sorted[j].Depth()
	})

	for _, task := range sorted {
		task.Draw()
	}

	project.HandleCamera()

	if !project.TaskOpen {

		project.CurrentBoard().HandleDroppedFiles()

		var clickedTask *Task
		clicked := false

		// We update the tasks from top (last) down, because if you click on one, you click on the top-most one.

		if !project.ContextMenuOpen && !project.ProjectSettingsOpen && MousePressed(rl.MouseLeftButton) {
			clicked = true
		}

		if project.ResizingImage {
			project.Selecting = false
		}

		if project.MousingOver() == "Project" {

			for i := len(project.CurrentBoard().Tasks) - 1; i >= 0; i-- {

				task := project.CurrentBoard().Tasks[i]

				if rl.CheckCollisionPointRec(GetWorldMousePosition(), task.Rect) && clickedTask == nil {
					clickedTask = task
				}

			}

			if time.Since(project.DoubleClickTimer).Seconds() > 0.33 {
				project.DoubleClickTimer = time.Time{}
			}

			if clicked {

				if clickedTask == nil {
					project.SelectionStart = GetWorldMousePosition()
					project.Selecting = true
				} else {
					project.Selecting = false

					if removeFromSelection {
						clickedTask.ReceiveMessage(MessageSelect, map[string]interface{}{})
					} else if addToSelection {
						clickedTask.ReceiveMessage(MessageSelect, map[string]interface{}{
							"task": clickedTask,
						})
					} else {
						if !clickedTask.Selected { // This makes it so you don't have to shift+drag to move already selected Tasks
							project.SendMessage(MessageSelect, map[string]interface{}{
								"task": clickedTask,
							})
						} else {
							clickedTask.ReceiveMessage(MessageSelect, map[string]interface{}{
								"task": clickedTask,
							})
						}
					}

				}

				if clickedTask == nil {

					project.DoubleClickTaskID = -1

					if !project.DoubleClickTimer.IsZero() && project.DoubleClickTaskID == -1 {
						task := project.CurrentBoard().CreateNewTask()
						task.ReceiveMessage(MessageDoubleClick, nil)
						project.Selecting = false
						project.DoubleClickTimer = time.Time{}
					} else {
						project.DoubleClickTimer = time.Now()
					}

				} else {

					if clickedTask.ID == project.DoubleClickTaskID && !project.DoubleClickTimer.IsZero() && clickedTask.Selected {
						clickedTask.ReceiveMessage(MessageDoubleClick, nil)
						project.DoubleClickTimer = time.Time{}
					} else {
						project.DoubleClickTimer = time.Now()
						project.SendMessage(MessageDragging, nil)
						project.DoubleClickTaskID = clickedTask.ID
					}

				}

			}

			if project.Selecting {

				diff := rl.Vector2Subtract(GetWorldMousePosition(), project.SelectionStart)
				x1, y1 := project.SelectionStart.X, project.SelectionStart.Y
				x2, y2 := diff.X, diff.Y
				if x2 < 0 {
					x2 *= -1
					x1 = GetWorldMousePosition().X
				}
				if y2 < 0 {
					y2 *= -1
					y1 = GetWorldMousePosition().Y
				}

				selectionRect = rl.Rectangle{x1, y1, x2, y2}

				if !project.ResizingImage && MouseReleased(rl.MouseLeftButton) {

					project.Selecting = false // We're done with the selection process

					count := 0

					for _, task := range project.CurrentBoard().Tasks {

						inSelectionRect := false
						var t *Task

						if rl.CheckCollisionRecs(selectionRect, task.Rect) {
							inSelectionRect = true
							t = task
						}

						if removeFromSelection {
							if inSelectionRect {

								if task.Selected {
									count++
								}

								task.ReceiveMessage(MessageSelect, map[string]interface{}{"task": t, "invert": true})

							}
						} else {

							if !addToSelection || inSelectionRect {

								if (!task.Selected && inSelectionRect) || (!addToSelection && inSelectionRect) {
									count++
								}

								task.ReceiveMessage(MessageSelect, map[string]interface{}{
									"task": t,
								})
							}
						}
					}
				}
			}

		} else {
			if MouseReleased(rl.MouseLeftButton) {
				project.Selecting = false
			}
		}
	}

	// This is true once at least one loop has happened
	project.FullyInitialized = true

	rl.DrawRectangleLinesEx(selectionRect, 1, getThemeColor(GUI_OUTLINE_HIGHLIGHTED))

	project.Shortcuts()

	if project.JustLoaded {

		for _, t := range project.GetAllTasks() {
			t.Draw() // We need to draw the task at least once to ensure the rects are updated by the Task's contents.
			// This makes it so that neighbors can be correct.
		}

		for _, board := range project.Boards {
			board.ReorderTasks()
		}

		project.Modified = false
		project.JustLoaded = false
	}

	for _, board := range project.Boards {
		board.HandleDeletedTasks()
	}
}

func (project *Project) SendMessage(message string, data map[string]interface{}) {

	for _, board := range project.Boards {
		board.SendMessage(message, data)
	}

}

func (project *Project) Shortcuts() {

	keybindings := programSettings.Keybindings

	keybindings.HandleResettingShortcuts()

	keybindings.ReenableAllShortcuts()

	for _, clash := range keybindings.GetClashes() {
		clash.Enabled = false
	}

	if !project.ProjectSettingsOpen {

		if !project.TaskOpen {

      {
				panSpeed := float32(16 / camera.Zoom)

				if keybindings.On(KBFasterPan) {
					panSpeed *= 3
				}

				if keybindings.On(KBPanUp) {
					project.CameraPan.Y += panSpeed
				}
				if keybindings.On(KBPanDown) {
					project.CameraPan.Y -= panSpeed
				}
				if keybindings.On(KBPanLeft) {
					project.CameraPan.X += panSpeed
				}
				if keybindings.On(KBPanRight) {
					project.CameraPan.X -= panSpeed
				}

				if keybindings.On(KBBoard1) {
					if len(project.Boards) > 0 {
						project.BoardIndex = 0
					}
				} else if keybindings.On(KBBoard2) {
					if len(project.Boards) > 1 {
						project.BoardIndex = 1
					}
				} else if keybindings.On(KBBoard2) {
					if len(project.Boards) > 1 {
						project.BoardIndex = 1
					}
				} else if keybindings.On(KBBoard3) {
					if len(project.Boards) > 2 {
						project.BoardIndex = 2
					}
				} else if keybindings.On(KBBoard4) {
					if len(project.Boards) > 3 {
						project.BoardIndex = 3
					}
				} else if keybindings.On(KBBoard5) {
					if len(project.Boards) > 4 {
						project.BoardIndex = 4
					}
				} else if keybindings.On(KBBoard6) {
					if len(project.Boards) > 5 {
						project.BoardIndex = 5
					}
				} else if keybindings.On(KBBoard7) {
					if len(project.Boards) > 6 {
						project.BoardIndex = 6
					}
				} else if keybindings.On(KBBoard8) {
					if len(project.Boards) > 7 {
						project.BoardIndex = 7
					}
				} else if keybindings.On(KBBoard9) {
					if len(project.Boards) > 8 {
						project.BoardIndex = 8
					}
				} else if keybindings.On(KBBoard10) {
					if len(project.Boards) > 9 {
						project.BoardIndex = 9
					}
				} else if keybindings.On(KBCenterView) {
					project.CameraPan.X = 0
					project.CameraPan.Y = 0
				} else if keybindings.On(KBSelectAllTasks) {

					for _, task := range project.CurrentBoard().Tasks {
						task.Selected = true
					}

				} else if keybindings.On(KBCopyTasks) {
					project.CurrentBoard().CopySelectedTasks()
				} else if keybindings.On(KBCutTasks) {
					project.CurrentBoard().CutSelectedTasks()
				} else if keybindings.On(KBPasteContent) {
					project.CurrentBoard().PasteContent()
				} else if keybindings.On(KBPaste) {
					project.CurrentBoard().PasteTasks()
				} else if keybindings.On(KBCreateTask) {
					task := project.CurrentBoard().CreateNewTask()
					task.ReceiveMessage(MessageDoubleClick, nil)
				} else if keybindings.On(KBRedo) {
          // NOTE(justasd): :Undo
				} else if keybindings.On(KBUndo) {
          // NOTE(justasd): :Undo
				} else if keybindings.On(KBDeleteTasks) {
					project.CurrentBoard().DeleteSelectedTasks()
				} else if keybindings.On(KBFocusOnTasks) {
					project.CurrentBoard().FocusViewOnSelectedTasks()
				} else if keybindings.On(KBEditTasks) {
					for _, task := range project.CurrentBoard().SelectedTasks(true) {
						task.ReceiveMessage(MessageDoubleClick, nil)
					}
				} else if keybindings.On(KBSaveAs) {

					// Project Shortcuts

					project.SaveAs()
				} else if keybindings.On(KBSave) {
					if project.FilePath == "" {
						project.SaveAs()
					} else {
						project.Save(false)
					}
				} else if keybindings.On(KBLoad) {
					if project.Modified {
					} else {
						project.ExecuteDestructiveAction(ActionLoadProject, "")
					}
				} else if keybindings.On(KBDeselectTasks) {
					project.SendMessage(MessageSelect, nil)
				}
			}
		}
	}
}

func (project *Project) GetEmptyBoard() *Board {
	for _, b := range project.Boards {
		if len(b.Tasks) == 0 {
			return b
		}
	}
	return nil
}

func (project *Project) AddBoard() {
	project.Boards = append(project.Boards, NewBoard(project))
}

func (project *Project) RemoveBoard(board *Board) {
	for index, b := range project.Boards {
		if b == board {
			b.Destroy()
			project.Boards = append(project.Boards[:index], project.Boards[index+1:]...)
			project.Log("Deleted empty Board: %s", b.Name)
			break
		}
	}
}

func (project *Project) FirstFreeID() int {

	usedIDs := map[int]bool{}

	tasks := project.GetAllTasks()

	for i := 0; i < firstFreeTaskID; i++ {
		if len(tasks) > i {
			usedIDs[tasks[i].ID] = true
		}
	}

	// Reuse already spent, but nonexistent IDs (i.e. create a task that has ID 4, then
	// delete that and create a new one; it should have an ID of 4 so that when VCS diff
	// the project file, it just alters the relevant pieces of info to make the original
	// Task #4 the new Task #4)
	for i := 0; i < firstFreeTaskID; i++ {
		exists := usedIDs[i]
		if !exists {
			return i
		}
	}

	// If no spent but unused IDs exist, then we can just use a new one and move on.
	id := firstFreeTaskID

	firstFreeTaskID++

	return id

}

func (project *Project) LockPositionToGrid(xy rl.Vector2) rl.Vector2 {

	return rl.Vector2{float32(math.Round(float64(xy.X/float32(project.GridSize)))) * float32(project.GridSize),
		float32(math.Round(float64(xy.Y/float32(project.GridSize)))) * float32(project.GridSize)}

}

func (project *Project) ReloadThemes() {

	loadThemes()

	_, themeExists := guiColors[currentTheme]
	if !themeExists {
		for k := range guiColors {
			currentTheme = k
			break
		}
	}

	guiThemes := []string{}
	for theme, _ := range guiColors {
		guiThemes = append(guiThemes, theme)
	}
	sort.Strings(guiThemes)
}

func (project *Project) GetFrameTime() float32 {
	ft := deltaTime
	return ft
}

func (project *Project) Destroy() {

	for _, board := range project.Boards {
		board.Destroy()
	}

	for _, res := range project.Resources {
		res.Destroy()
	}

}

func (project *Project) RetrieveResource(resourcePath string) *Resource {

	existingResource, exists := project.Resources[resourcePath]

	if exists {
		return existingResource
	}
	return nil

}

// LoadResource returns the resource loaded from the filepath and a boolean indicating if it was just loaded (true), or
// loaded previously and retrieved (false).
func (project *Project) LoadResource(resourcePath string) (*Resource, bool) {

	downloadedFile := false
	newlyLoaded := false

	var loadedResource *Resource

	existingResource, exists := project.Resources[resourcePath]

	if exists {

		loadedResource = existingResource

		// We check to see if the mod time isn't the same; if so, we destroy the old one and load it again

		if file, err := os.Open(loadedResource.LocalFilepath); !loadedResource.Temporary && err == nil {

			if stats, err := file.Stat(); err == nil {
				// We have to check if the size is greater than 0 because it's possible we're seeing the file before it's been written fully to disk;
				if stats.Size() > 0 && stats.ModTime().After(loadedResource.ModTime) {
					loadedResource.Destroy()
					delete(project.Resources, resourcePath)
					loadedResource, newlyLoaded = project.LoadResource(resourcePath) // Force reloading if the file is outdated
				}
			}

			file.Close()

		}

	} else if resourcePath != "" {

		localFilepath := resourcePath

		// Attempt downloading it if it's an HTTP file
		if url, err := urlx.Parse(resourcePath); err == nil && url.Host != "" && url.Scheme != "" {

			response, err := http.Get(url.String())

			if err != nil {

				project.Log("Could not open HTTP address: ", err.Error())

			} else {

				tempFile, err := ioutil.TempFile("", "masterplan_resource")
				if err != nil {
					project.Log("Could not open temporary file: ", err.Error())
				} else {
					io.Copy(tempFile, response.Body)
					newlyLoaded = true
					localFilepath = tempFile.Name()
					downloadedFile = true
				}

				response.Body.Close()

				tempFile.Close()

			}

		}

		fileType, err := mimetype.DetectFile(localFilepath)

		if err != nil {
			project.Log("Error identifying file type: ", err.Error())
		} else {

			newlyLoaded = true

			// We have to rename the resource according to what it is because raylib expects the extensions of files to be correct.
			// png image files need to have .png as an extension, for example.
			if downloadedFile && !fileType.Is(strings.ToLower(filepath.Ext(localFilepath))) {
				newName := localFilepath + fileType.Extension()
				os.Rename(localFilepath, newName)
				localFilepath = newName
			}

			if strings.Contains(fileType.String(), "image") {

				if strings.Contains(fileType.String(), "gif") {
					file, err := os.Open(localFilepath)
					if err != nil {
						project.Log("Could not open GIF: ", err.Error())
					} else {

						defer file.Close()

						gifFile, err := gif.DecodeAll(file)

						if err != nil {
							project.Log("Could not decode GIF: ", err.Error())
						} else {
							res := project.RegisterResource(resourcePath, localFilepath, gifFile)
							res.Temporary = downloadedFile
							loadedResource = res
						}

					}
				} else { // Ordinary image
					tex := rl.LoadTexture(localFilepath)
					res := project.RegisterResource(resourcePath, localFilepath, tex)
					res.Temporary = downloadedFile
					loadedResource = res
				}

			} else if strings.Contains(fileType.String(), "audio") {
				res := project.RegisterResource(resourcePath, localFilepath, nil)
				res.Temporary = downloadedFile
				loadedResource = res
			} else {
				project.Log("Unable to load resource [%s].", resourcePath)
			}

		}

	}

	return loadedResource, newlyLoaded

}

func (project *Project) WorldToGrid(worldX, worldY float32) (int, int) {
	return int(worldX / float32(project.GridSize)), int(worldY / float32(project.GridSize))
}

func (project *Project) ExecuteDestructiveAction(action string, argument string) {

	switch action {
	case ActionNewProject:
		project.Destroy()
		currentProject = NewProject()
		currentProject.Log("New project created.")
	case ActionLoadProject:

		var loadProject *Project

		if argument == "" {
			loadProject = LoadProjectFrom()
		} else {
			loadProject = LoadProject(argument)
		}

		// Unsuccessful loads will not destroy the current project
		if loadProject != nil {
			currentProject.Destroy()
			currentProject = loadProject
		}

	case ActionSaveAsProject:
		project.FilePath = argument
		project.Save(false)
	case ActionQuit:
		quit = true
	}

}

func (project *Project) OpenSettings() {
	project.ReloadThemes() // Reload the themes when opening the settings window
	project.ProjectSettingsOpen = true
}
