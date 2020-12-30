package main

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
  "os/exec"
  "path/filepath"

	"github.com/gabriel-vasile/mimetype"
	rl "github.com/gen2brain/raylib-go/raylib"
	uuid "github.com/gofrs/uuid"
)

type Position struct {
	X, Y int
}

type Board struct {
	Tasks         []*Task
	ToBeDeleted   []*Task
	ToBeRestored  []*Task
	Project       *Project
	Name          string
	TaskLocations map[Position][]*Task
	UndoBuffer    *UndoBuffer
}

func NewBoard(project *Project) *Board {
	board := &Board{
		Tasks:         []*Task{},
		Project:       project,
		Name:          fmt.Sprintf("Board %d", len(project.Boards)+1),
		TaskLocations: map[Position][]*Task{},
	}

	board.UndoBuffer = NewUndoBuffer(board)

	return board
}

func (board *Board) CreateNewTask() *Task {
	newTask := NewTask(board)
	halfGrid := float32(board.Project.GridSize / 2)
	gp := rl.Vector2{GetWorldMousePosition().X - halfGrid, GetWorldMousePosition().Y - halfGrid}

	newTask.Position = board.Project.LockPositionToGrid(gp)

	newTask.Rect.X, newTask.Rect.Y = newTask.Position.X, newTask.Position.Y
	board.Tasks = append(board.Tasks, newTask)

	board.ReorderTasks()

	newTask.TaskType.SetChoice(board.Project.PreviousTaskType)

	if newTask.TaskType.ChoiceAsString() == "Image" || newTask.TaskType.ChoiceAsString() == "Sound" {
		newTask.FilePathTextbox.Focused = true
	} else {
		newTask.Description.Focused = true
	}

	// We need to record both the Task being invalid, as well as being valid, for undoing / redoing
	newTask.Valid = false
	board.UndoBuffer.Capture(newTask)
	newTask.Valid = true
	board.UndoBuffer.Capture(newTask)

	if !board.Project.JustLoaded {
		// If we're loading a project, we don't want to automatically select new tasks
		board.Project.SendMessage(MessageSelect, map[string]interface{}{"task": newTask})
	}

	return newTask
}

func (board *Board) DeleteTask(task *Task) {

	if task.Valid {

		board.UndoBuffer.Capture(task)
		task.Valid = false
		board.UndoBuffer.Capture(task)

		board.ToBeDeleted = append(board.ToBeDeleted, task)
		task.ReceiveMessage(MessageDelete, map[string]interface{}{"task": task})
	}

}

func (board *Board) RestoreTask(task *Task) {

	if !task.Valid {

		board.UndoBuffer.Capture(task)
		task.Valid = true
		board.UndoBuffer.Capture(task)

		board.ToBeRestored = append(board.ToBeRestored, task)
		task.ReceiveMessage(MessageDropped, map[string]interface{}{"task": task})

	}

}

func (board *Board) DeleteSelectedTasks() {

	selected := board.SelectedTasks(false)

	for _, t := range selected {
		board.DeleteTask(t)
	}

	board.ReorderTasks()
}

func (board *Board) FocusViewOnSelectedTasks() {

	if len(board.Tasks) > 0 {

		center := rl.Vector2{}
		taskCount := float32(0)

		for _, task := range board.SelectedTasks(false) {
			taskCount++
			center.X += task.Position.X + task.Rect.Width/2
			center.Y += task.Position.Y + task.Rect.Height/2
		}

		if taskCount > 0 {

			center.X = center.X / taskCount
			center.Y = center.Y / taskCount

			center.X *= -1
			center.Y *= -1

			board.Project.CameraPan = center // Pan's a negative offset for the camera

		}

	}

}

func (board *Board) HandleDroppedFiles() {

	if rl.IsFileDropped() {

		fileCount := int32(0)
		for _, filePath := range rl.GetDroppedFiles(&fileCount) {

			taskType, _ := mimetype.DetectFile(filePath)

			if taskType != nil {

				task := NewTask(board)
				task.Position.X = camera.Target.X
				task.Position.Y = camera.Target.Y
				success := true

				if strings.Contains(taskType.String(), "image") {
					task.TaskType.CurrentChoice = TASK_TYPE_IMAGE
					task.FilePathTextbox.SetText(filePath)
					task.LoadResource()
				} else if strings.HasPrefix(taskType.String(), "text/") {

					// Attempt to read it in
					data, err := ioutil.ReadFile(filePath)
					if err == nil {
						task.Description.SetText(string(data))
						task.TaskType.CurrentChoice = TASK_TYPE_NOTE
					}

				} else {
					board.Project.Log("Could not create a Task for incompatible file at [%s].", filePath)
					success = false
				}

				board.Tasks = append(board.Tasks, task)
				if !success {
					board.DeleteTask(task)
				}
				continue
			}
		}
		rl.ClearDroppedFiles()

	}

}

func (board *Board) CopySelectedTasks() {

	board.Project.Cutting = false

	board.Project.CopyBuffer = []*Task{}

	for _, task := range board.SelectedTasks(false) {
		board.Project.CopyBuffer = append(board.Project.CopyBuffer, task)
	}
}

func (board *Board) CutSelectedTasks() {
	board.Project.LogOn = false
	board.CopySelectedTasks()
	board.Project.LogOn = true
	board.Project.Cutting = true
}

func (board *Board) PasteTasks() {

	if len(board.Project.CopyBuffer) > 0 {

		board.UndoBuffer.On = false

		for _, task := range board.Tasks {
			task.Selected = false
		}

		clones := []*Task{}

		cloneTask := func(srcTask *Task) *Task {
			ogBoard := srcTask.Board
			srcTask.Board = board
			clone := srcTask.Clone()
			srcTask.Board = ogBoard
			board.Tasks = append(board.Tasks, clone)
			clone.LoadResource()
			clones = append(clones, clone)
			return clone
		}

		copied := func(task *Task) bool {
			for _, copy := range board.Project.CopyBuffer {
				if copy == task {
					return true
				}
			}
			return false
		}

		center := rl.Vector2{}

		for _, t := range board.Project.CopyBuffer {
			tp := t.Position
			tp.X += t.Rect.Width / 2
			tp.Y += t.Rect.Height / 2
			center = rl.Vector2Add(center, tp)
		}

		center.X /= float32(len(board.Project.CopyBuffer))
		center.Y /= float32(len(board.Project.CopyBuffer))

		for _, srcTask := range board.Project.CopyBuffer {

			if srcTask.Is(TASK_TYPE_LINE) && srcTask.LineBase != nil && srcTask.Board != board {
				if !copied(srcTask.LineBase) {
					board.Project.Log("WARNING: Cannot paste Line arrows on a different board than the Line base.")
				}
			} else if srcTask.LineBase == nil || !copied(srcTask.LineBase) {

				clone := cloneTask(srcTask)
				diff := rl.Vector2Subtract(GetWorldMousePosition(), center)
				clone.Position = rl.Vector2Add(clone.Position, diff)
				clone.Position = board.Project.LockPositionToGrid(clone.Position)

				if clone.Is(TASK_TYPE_LINE) {
					for _, ending := range clone.LineEndings {
						ending.Position = rl.Vector2Add(ending.Position, diff)
						ending.Position = board.Project.LockPositionToGrid(ending.Position)
					}
				}

			}

		}

		board.ReorderTasks()

		for _, clone := range clones {
			if clone.Is(TASK_TYPE_LINE) && len(clone.LineEndings) > 0 {
				clones = append(clones, clone.LineEndings...)
			}
		}

		board.UndoBuffer.On = true

		for _, clone := range clones {

			clone.Valid = false
			board.UndoBuffer.Capture(clone)
			clone.Valid = true
			board.UndoBuffer.Capture(clone)

			clone.Selected = true
		}

		board.ReorderTasks()

		if board.Project.Cutting {
			for _, task := range board.Project.CopyBuffer {
				task.Board.DeleteTask(task)
			}
			board.Project.Cutting = false
			board.Project.CopyBuffer = []*Task{}
		}

	}

}

func (board *Board) PasteContent() {

  // TODO(justasd): :Portability
  result, err := exec.Command("xclip", "-t", "TARGETS", "-o").CombinedOutput()
  if err != nil {
    board.Project.Log("Failed to get target data from xclip: '%s'.", err)
    return
  }

  result_str := string(result[:])
  targets := strings.Split(strings.Replace(result_str, "\r\n", "\n", -1), "\n")

  get_clipboard_data := func(target string) ([]byte, error) {
    result, err := exec.Command("xclip", "-t", target, "-o").CombinedOutput()
    if err != nil {
      board.Project.Log("Failed to get clipboard data from xclip: '%s'.", err)
      return nil, err
    }

    return result, nil
  }

  //fmt.Println(targets)

  for _, target := range targets {
    if strings.EqualFold(target, "STRING") {
      task := board.CreateNewTask()
      task.TaskType.CurrentChoice = TASK_TYPE_NOTE

      result, err := get_clipboard_data(target)
      if err != nil { return }

      str := string(result[:])
			task.Description.SetText(str)

      break
    }

    if strings.Contains(target, "image") {

      img_data , err := get_clipboard_data(target)
      if err != nil { return }


      id := uuid.Must(uuid.NewV4()).String()
      dir := filepath.Dir(board.Project.FilePath)

      // NOTE(justasd): Extension is a @HACK
      save_path := filepath.Join(dir, id) + ".png"
      err = ioutil.WriteFile(save_path, img_data, 0644)
      if err != nil {
        board.Project.Log("Failed to save iamge file to '%s'.", save_path)
        return
      }

      fmt.Printf("Saved image to '%s'\n", save_path)

      task := board.CreateNewTask()
      task.TaskType.CurrentChoice = TASK_TYPE_IMAGE
      task.FilePathTextbox.SetText(save_path)
      task.LoadResource()

      break
    }
  }
}

func (board *Board) ReorderTasks() {

  return

	sort.Slice(board.Tasks, func(i, j int) bool {
		ba := board.Tasks[i]
		bb := board.Tasks[j]
		if ba.Position.Y == bb.Position.Y {
			return ba.Position.X < bb.Position.X
		}
		return ba.Position.Y < bb.Position.Y
	})

	// Reordering Tasks should not alter the Undo Buffer, as altering the Undo Buffer generally happens explicitly

	prevOn := board.UndoBuffer.On
	board.UndoBuffer.On = false
	board.SendMessage(MessageDropped, nil)
	board.UndoBuffer.On = prevOn

}

// Returns the index of the board in the Project's Board stack
func (board *Board) Index() int {
	for i := range board.Project.Boards {
		if board.Project.Boards[i] == board {
			return i
		}
	}
	return -1
}

func (board *Board) Destroy() {
	for _, task := range board.Tasks {
		task.ReceiveMessage(MessageDelete, map[string]interface{}{"task": task})
		task.Destroy()
	}
}

func (board *Board) GetTasksInPosition(x, y float32) []*Task {
	cx, cy := board.Project.WorldToGrid(x, y)
	return board.TaskLocations[Position{cx, cy}]
}

func (board *Board) GetTasksInRect(x, y, w, h float32) []*Task {

	tasks := []*Task{}

	added := func(t *Task) bool {
		for _, t2 := range tasks {
			if t2 == t {
				return true
			}
		}
		return false
	}

	for cy := y; cy < y+h; cy += float32(board.Project.GridSize) {

		for cx := x; cx < x+w; cx += float32(board.Project.GridSize) {

			for _, t := range board.GetTasksInPosition(cx, cy) {
				if !added(t) {
					tasks = append(tasks, t)
				}
			}

		}

	}

	return tasks
}

func (board *Board) RemoveTaskFromGrid(task *Task) {

	for _, position := range task.GridPositions {

		for i, t := range board.TaskLocations[position] {

			if t == task {
				board.TaskLocations[position][i] = nil
				board.TaskLocations[position] = append(board.TaskLocations[position][:i], board.TaskLocations[position][i+1:]...)
				break
			}

		}

	}

}

func (board *Board) AddTaskToGrid(task *Task) {

	positions := []Position{}

	gs := float32(board.Project.GridSize)
	startX, startY := int(task.Position.X/gs), int(task.Position.Y/gs)
	endX, endY := int((task.Position.X+task.Rect.Width)/gs), int((task.Position.Y+task.Rect.Height)/gs)

	for y := startY; y < endY; y++ {

		for x := startX; x < endX; x++ {

			p := Position{x, y}

			positions = append(positions, p)

			_, exists := board.TaskLocations[p]

			if !exists {
				board.TaskLocations[p] = []*Task{}
			}

			board.TaskLocations[p] = append(board.TaskLocations[p], task)

		}

	}

	task.GridPositions = positions

}

func (board *Board) SelectedTasks(returnFirstSelectedTask bool) []*Task {

	selected := []*Task{}

	for _, task := range board.Tasks {

		if task.Selected {

			selected = append(selected, task)

			if returnFirstSelectedTask {
				return selected
			}

		}

	}

	return selected

}

func (board *Board) HandleDeletedTasks() {

	changed := false

	for _, task := range board.ToBeDeleted {
		for index, t := range board.Tasks {
			if task == t {
				board.Tasks[index] = nil
				board.Tasks = append(board.Tasks[:index], board.Tasks[index+1:]...)
				changed = true
				break
			}
		}
	}
	board.ToBeDeleted = []*Task{}

	for _, task := range board.ToBeRestored {
		board.Tasks = append(board.Tasks, task)
		changed = true
	}
	board.ToBeRestored = []*Task{}

	// We only want to reorder tasks if tasks were actually deleted or restored, as it is costly.
	if changed {
		board.ReorderTasks()
	}

}

func (board *Board) SendMessage(message string, data map[string]interface{}) {

	for _, task := range board.Tasks {
		task.ReceiveMessage(message, data)
	}

}
