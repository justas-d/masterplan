package main

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goware/urlx"
	"github.com/ncruces/zenity"
	"github.com/pkg/browser"
	"github.com/tanema/gween/ease"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/chonla/roman-number-go"

	"github.com/hako/durafmt"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	TASK_TYPE_BOOLEAN = iota
	TASK_TYPE_PROGRESSION
	TASK_TYPE_NOTE
	TASK_TYPE_IMAGE
	TASK_TYPE_SOUND
	TASK_TYPE_TIMER
	TASK_TYPE_LINE
	TASK_TYPE_MAP
	TASK_TYPE_WHITEBOARD
)

const (
	TASK_NOT_DUE = iota
	TASK_DUE_FUTURE
	TASK_DUE_TODAY
	TASK_DUE_LATE
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
	CompletionTime time.Time

	DeadlineCheckbox     *Checkbox
	DeadlineDaySpinner   *NumberSpinner
	DeadlineMonthSpinner *Spinner
	DeadlineYearSpinner  *NumberSpinner

	TimerSecondSpinner *NumberSpinner
	TimerMinuteSpinner *NumberSpinner
	TimerValue         float32
	TimerRunning       bool
	TimerName          *Textbox

	CompletionCheckbox           *Checkbox
	CompletionProgressionCurrent *NumberSpinner
	CompletionProgressionMax     *NumberSpinner
	Image                        rl.Texture2D

	GifAnimation *GifAnimation

	SoundControl       *beep.Ctrl
	SoundVolume        *effects.Volume
	SoundStream        beep.StreamSeekCloser
	SoundComplete      bool
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
	// ArrowPointingToTask *Task

	TaskAbove     *Task
	TaskBelow     *Task
	TaskRight     *Task
	TaskLeft      *Task
	TaskUnder     *Task
	RestOfStack   []*Task
	SubTasks      []*Task
	GridPositions []Position
	Valid         bool

	EditPanel                      *Panel
	CompletionTimeLabel            *Label
	LoadMediaButton                *Button
	ClearMediaButton               *Button
	CreationLabel                  *Label
	DisplayedText                  string
	URLButtons                     []URLButton
	SuccessfullyLoadedResourceOnce bool

	MapImage   *MapImage
	Whiteboard *Whiteboard
}

func NewTask(board *Board) *Task {

	months := []string{
		"January",
		"February",
		"March",
		"April",
		"May",
		"June",
		"July",
		"August",
		"September",
		"October",
		"November",
		"December",
	}

	postX := float32(180)

	task := &Task{
		Rect:                         rl.Rectangle{0, 0, 16, 16},
		Board:                        board,
		TaskType:                     NewButtonGroup(0, 32, 500, 32, 2, "Check Box", "Progression", "Note", "Image", "Sound", "Timer", "Line", "Map", "Whiteboard"),
		Description:                  NewTextbox(postX, 64, 512, 32),
		TimerName:                    NewTextbox(postX, 64, 256, 16),
		CompletionCheckbox:           NewCheckbox(postX, 96, 32, 32),
		CompletionProgressionCurrent: NewNumberSpinner(postX, 96, 128, 40),
		CompletionProgressionMax:     NewNumberSpinner(postX+80, 96, 128, 40),
		NumberingPrefix:              []int{-1},
		ID:                           board.Project.FirstFreeID(),
		FilePathTextbox:              NewTextbox(postX, 64, 512, 16),
		DeadlineCheckbox:             NewCheckbox(postX, 112, 32, 32),
		DeadlineMonthSpinner:         NewSpinner(postX+40, 128, 200, 40, months...),
		DeadlineDaySpinner:           NewNumberSpinner(postX+100, 80, 160, 40),
		DeadlineYearSpinner:          NewNumberSpinner(postX+240, 128, 160, 40),
		TimerMinuteSpinner:           NewNumberSpinner(postX, 0, 160, 40),
		TimerSecondSpinner:           NewNumberSpinner(postX, 0, 160, 40),
		LineEndings:                  []*Task{},
		LineBezier:                   NewCheckbox(postX, 64, 32, 32),
		GridPositions:                []Position{},
		Valid:                        true,
		LoadMediaButton:              NewButton(0, 0, 128, 32, "Load", false),
		CompletionTimeLabel:          NewLabel("Completion time"),
		CreationLabel:                NewLabel("Creation time"),
	}

	task.SetPanel()

	task.DeadlineMonthSpinner.ExpandUpwards = true
	task.DeadlineMonthSpinner.ExpandMaxRowCount = 5

	task.CreationTime = time.Now()
	task.CompletionProgressionCurrent.Textbox.MaxCharactersPerLine = 19
	task.CompletionProgressionCurrent.Textbox.AllowNewlines = false
	task.CompletionProgressionCurrent.Minimum = 0

	task.CompletionProgressionMax.Textbox.MaxCharactersPerLine = 19
	task.CompletionProgressionMax.Textbox.AllowNewlines = false
	task.CompletionProgressionMax.Minimum = 0

	task.MinSize = rl.Vector2{task.Rect.Width, task.Rect.Height}
	task.MaxSize = rl.Vector2{0, 0}
	task.Description.AllowNewlines = true
	task.FilePathTextbox.AllowNewlines = false

	task.FilePathTextbox.VerticalAlignment = ALIGN_CENTER

	task.DeadlineDaySpinner.Minimum = 1
	task.DeadlineDaySpinner.Maximum = 31
	task.DeadlineDaySpinner.Loop = true
	task.DeadlineDaySpinner.Rect.X = task.DeadlineMonthSpinner.Rect.X + task.DeadlineMonthSpinner.Rect.Width + 8
	task.DeadlineYearSpinner.Rect.X = task.DeadlineDaySpinner.Rect.X + task.DeadlineDaySpinner.Rect.Width + 8

	task.TimerSecondSpinner.Minimum = 0
	task.TimerSecondSpinner.Maximum = 59
	task.TimerMinuteSpinner.Minimum = 0

	task.SoundVolume = &effects.Volume{
		Base: 2,
	}

	task.UpdateSoundVolume()

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
		TASK_TYPE_BOOLEAN,
		TASK_TYPE_PROGRESSION,
		TASK_TYPE_NOTE)
	column.Row().Item(task.Description,
		TASK_TYPE_BOOLEAN,
		TASK_TYPE_PROGRESSION,
		TASK_TYPE_NOTE)

	row = column.Row()
	row.Item(NewLabel("Name:"), TASK_TYPE_TIMER)

	row = column.Row()
	row.Item(task.TimerName, TASK_TYPE_TIMER)

	row = column.Row()
	row.Item(NewLabel("Filepath:"), TASK_TYPE_IMAGE, TASK_TYPE_SOUND)
	row = column.Row()
	row.Item(task.FilePathTextbox, TASK_TYPE_IMAGE, TASK_TYPE_SOUND)
	row = column.Row()
	row.Item(task.LoadMediaButton, TASK_TYPE_IMAGE, TASK_TYPE_SOUND)

	row = column.Row()
	row.Item(NewLabel("Completed:"), TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION)
	row.Item(task.CompletionCheckbox, TASK_TYPE_BOOLEAN)
	row.Item(task.CompletionProgressionCurrent, TASK_TYPE_PROGRESSION)
	row.Item(NewLabel("out of"), TASK_TYPE_PROGRESSION)
	row.Item(task.CompletionProgressionMax, TASK_TYPE_PROGRESSION)

	row = column.Row()
	row.Item(NewLabel("Completion Date:"), TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION)
	row.Item(task.CompletionTimeLabel, TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION)

	row = column.Row()
	row.Item(NewLabel("Deadline:"), TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION)
	row.Item(task.DeadlineCheckbox, TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION).Name = "deadline_on"

	row = column.Row()
	row.Item(task.DeadlineDaySpinner, TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION).Name = "deadline_sub"
	row.Item(task.DeadlineMonthSpinner, TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION).Name = "deadline_sub"
	row.Item(task.DeadlineYearSpinner, TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION).Name = "deadline_sub"

	row = column.Row()
	row.Item(NewLabel("Minutes:"), TASK_TYPE_TIMER)
	row.Item(task.TimerMinuteSpinner, TASK_TYPE_TIMER)
	row.Item(NewLabel("Seconds:"), TASK_TYPE_TIMER)
	row.Item(task.TimerSecondSpinner, TASK_TYPE_TIMER)

	row = column.Row()
	row.Item(NewLabel("Bezier Lines:"), TASK_TYPE_LINE)
	row.Item(task.LineBezier, TASK_TYPE_LINE)

	row = column.Row()
	row.Item(NewButton(0, 0, 128, 32, "Shift Up", false), TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD).Name = "shift up"
	row = column.Row()
	row.Item(NewButton(0, 0, 128, 32, "Shift Left", false), TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD).Name = "shift left"
	row.Item(NewButton(0, 0, 128, 32, "Shift Right", false), TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD).Name = "shift right"
	row = column.Row()
	row.Item(NewButton(0, 0, 128, 32, "Shift Down", false), TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD).Name = "shift down"

	row = column.Row()
	row.Item(NewButton(0, 0, 128, 32, "Clear", false), TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD).Name = "clear"
	row.Item(NewButton(0, 0, 128, 32, "Invert", false), TASK_TYPE_WHITEBOARD).Name = "invert"

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

	cc := *copyData.CompletionCheckbox
	copyData.CompletionCheckbox = &cc

	// We have to make explicit clones of some elements, though, as they have references otherwise
	copyData.CompletionProgressionCurrent = task.CompletionProgressionCurrent.Clone()
	copyData.CompletionProgressionMax = task.CompletionProgressionMax.Clone()

	cPath := *copyData.FilePathTextbox
	copyData.FilePathTextbox = &cPath

	copyData.TimerMinuteSpinner = task.TimerMinuteSpinner.Clone()
	copyData.TimerSecondSpinner = task.TimerSecondSpinner.Clone()

	timerName := *copyData.TimerName
	copyData.TimerName = &timerName

	dlc := *copyData.DeadlineCheckbox
	copyData.DeadlineCheckbox = &dlc

	copyData.DeadlineDaySpinner = task.DeadlineDaySpinner.Clone()

	dms := *copyData.DeadlineMonthSpinner
	copyData.DeadlineMonthSpinner = &dms

	copyData.DeadlineYearSpinner = task.DeadlineYearSpinner.Clone()

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

	copyData.TimerRunning = false // We don't want to clone the timer running
	copyData.TimerValue = 0
	copyData.PrevFilePath = ""
	copyData.GifAnimation = nil
	copyData.SoundControl = nil
	copyData.SoundStream = nil

	if copyData.SoundVolume != nil {
		speaker.Lock()
		ov := *copyData.SoundVolume
		copyData.SoundVolume = &ov
		copyData.SoundVolume.Streamer = nil
		speaker.Unlock()
	}

	copyData.ID = copyData.Board.Project.FirstFreeID()

	copyData.ReceiveMessage(MessageTaskClose, nil) // We do this to recreate the resources for the Task, if necessary.

	copyData.SetPanel()

	if task.MapImage != nil {
		copyData.MapImage = NewMapImage(&copyData)
		copyData.MapImage.Copy(task.MapImage)
	}

	if task.Whiteboard != nil {
		copyData.Whiteboard = NewWhiteboard(&copyData)
		copyData.Whiteboard.Copy(task.Whiteboard)
	}

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

	jsonData, _ = sjson.Set(jsonData, `Checkbox\.Checked`, task.CompletionCheckbox.Checked)
	jsonData, _ = sjson.Set(jsonData, `Progression\.Current`, task.CompletionProgressionCurrent.Number())
	jsonData, _ = sjson.Set(jsonData, `Progression\.Max`, task.CompletionProgressionMax.Number())
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
	jsonData, _ = sjson.Set(jsonData, `TaskType\.CurrentChoice`, task.TaskType.CurrentChoice)

	if task.Board.Project.SaveSoundsPlaying.Checked {
		jsonData, _ = sjson.Set(jsonData, `SoundPaused`, task.SoundControl != nil && task.SoundControl.Paused)
	}

	if task.DeadlineCheckbox.Checked {
		jsonData, _ = sjson.Set(jsonData, `DeadlineDaySpinner\.Number`, task.DeadlineDaySpinner.Number())
		jsonData, _ = sjson.Set(jsonData, `DeadlineMonthSpinner\.CurrentChoice`, task.DeadlineMonthSpinner.CurrentChoice)
		jsonData, _ = sjson.Set(jsonData, `DeadlineYearSpinner\.Number`, task.DeadlineYearSpinner.Number())
	}

	if task.Is(TASK_TYPE_TIMER) {
		jsonData, _ = sjson.Set(jsonData, `TimerSecondSpinner\.Number`, task.TimerSecondSpinner.Number())
		jsonData, _ = sjson.Set(jsonData, `TimerMinuteSpinner\.Number`, task.TimerMinuteSpinner.Number())
		jsonData, _ = sjson.Set(jsonData, `TimerName\.Text`, task.TimerName.Text())
	}

	jsonData, _ = sjson.Set(jsonData, `CreationTime`, task.CreationTime.Format(`Jan 2 2006 15:04:05`))

	if !task.CompletionTime.IsZero() {
		jsonData, _ = sjson.Set(jsonData, `CompletionTime`, task.CompletionTime.Format(`Jan 2 2006 15:04:05`))
	}

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

	if task.Is(TASK_TYPE_MAP) && task.MapImage != nil {
		data := [][]int32{}
		for y := 0; y < int(task.MapImage.cellHeight); y++ {
			data = append(data, []int32{})
			for x := 0; x < int(task.MapImage.cellWidth); x++ {
				data[y] = append(data[y], task.MapImage.Data[y][x])
			}
		}
		jsonData, _ = sjson.Set(jsonData, `MapData`, data)
	}

	if task.Is(TASK_TYPE_WHITEBOARD) && task.Whiteboard != nil {
		jsonData, _ = sjson.Set(jsonData, `Whiteboard`, task.Whiteboard.Serialize())
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

	getInt := func(name string) int {
		return int(gjson.Get(jsonData, name).Int())
	}

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

	task.CompletionCheckbox.Checked = getBool(`Checkbox\.Checked`)
	task.CompletionProgressionCurrent.SetNumber(getInt(`Progression\.Current`))
	task.CompletionProgressionMax.SetNumber(getInt(`Progression\.Max`))
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
	task.TaskType.CurrentChoice = getInt(`TaskType\.CurrentChoice`)

	if hasData(`DeadlineDaySpinner\.Number`) {
		task.DeadlineCheckbox.Checked = true
		task.DeadlineDaySpinner.SetNumber(getInt(`DeadlineDaySpinner\.Number`))
		task.DeadlineMonthSpinner.CurrentChoice = getInt(`DeadlineMonthSpinner\.CurrentChoice`)
		task.DeadlineYearSpinner.SetNumber(getInt(`DeadlineYearSpinner\.Number`))
	}

	if hasData(`TimerSecondSpinner\.Number`) {
		task.TimerSecondSpinner.SetNumber(getInt(`TimerSecondSpinner\.Number`))
		task.TimerMinuteSpinner.SetNumber(getInt(`TimerMinuteSpinner\.Number`))
		task.TimerName.SetText(getString(`TimerName\.Text`))
	}

	creationTime, err := time.Parse(`Jan 2 2006 15:04:05`, getString(`CreationTime`))
	if err == nil {
		task.CreationTime = creationTime
	}

	if hasData(`CompletionTime`) {
		// Wouldn't be strange to not have a completion for incomplete Tasks.
		ctString := getString(`CompletionTime`)
		completionTime, err := time.Parse(`Jan 2 2006 15:04:05`, ctString)
		if err == nil {
			task.CompletionTime = completionTime
		}
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

	if hasData(`MapData`) {

		if task.MapImage == nil {
			task.MapImage = NewMapImage(task)
		}

		for y, row := range gjson.Get(jsonData, `MapData`).Array() {
			for x, value := range row.Array() {
				task.MapImage.Data[y][x] = int32(value.Int())
			}
		}

		task.MapImage.cellWidth = int(int32(task.DisplaySize.X) / task.Board.Project.GridSize)
		task.MapImage.cellHeight = int((int32(task.DisplaySize.Y) - task.Board.Project.GridSize) / task.Board.Project.GridSize)
		task.MapImage.Changed = true
	}

	if hasData(`Whiteboard`) {

		if task.Whiteboard == nil {
			task.Whiteboard = NewWhiteboard(task)
		}

		task.Whiteboard.Resize(task.DisplaySize.X, task.DisplaySize.Y-float32(task.Board.Project.GridSize))

		wbData := []string{}
		for _, row := range gjson.Get(jsonData, `Whiteboard`).Array() {
			wbData = append(wbData, row.String())
		}

		task.Whiteboard.Deserialize(wbData)

	}

	// We do this to update the task after loading all of the information.
	task.LoadResource()

	if task.SoundControl != nil {
		task.SoundControl.Paused = true
		if gjson.Get(jsonData, `SoundPaused`).Exists() {
			task.SoundControl.Paused = getBool(`SoundPaused`)
		}
	}
}

func (task *Task) Update() {

	if task.Is(TASK_TYPE_MAP) {
		task.MinSize = rl.Vector2{64, 80}
		task.MaxSize = rl.Vector2{512, 512 + 16}
	} else if task.Is(TASK_TYPE_WHITEBOARD) {
		task.MinSize = rl.Vector2{128, 80}
		task.MaxSize = rl.Vector2{512, 512 + 16}
	} else {
		task.MinSize = rl.Vector2{16, 16}
		task.MaxSize = rl.Vector2{0, 0}
	}

	if task.SoundComplete {

		// We want to lock and unlock the speaker as little as possible, and only when manipulating streams or controls.

		speaker.Lock()

		task.SoundComplete = false
		task.SoundControl.Paused = true
		task.SoundStream.Seek(0)

		speaker.Unlock()

		speaker.Play(beep.Seq(task.SoundControl, beep.Callback(task.OnSoundCompletion)))

		speaker.Lock()

		above := task.TaskAbove

		if task.TaskBelow != nil && task.TaskBelow.Is(TASK_TYPE_SOUND) && task.TaskBelow.SoundControl != nil {
			task.SoundControl.Paused = true
			task.TaskBelow.SoundControl.Paused = false
		} else if above != nil {

			for above.TaskAbove != nil && above.TaskAbove.SoundControl != nil && above.Is(TASK_TYPE_SOUND) {
				above = above.TaskAbove
			}

			if above != nil {
				task.SoundControl.Paused = true
				above.SoundControl.Paused = false
			}
		} else {
			task.SoundControl.Paused = false
		}

		speaker.Unlock()

	}

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
		if task.Complete() && task.CompletionTime.IsZero() {
			task.CompletionTime = time.Now()
		} else if !task.Complete() {
			task.CompletionTime = time.Time{}
		}

		if !rl.CheckCollisionRecs(task.Rect, cameraRect) {
			task.Visible = false
		}
	}

	if task.Is(TASK_TYPE_TIMER) {

		if task.TimerRunning {

			countdownMax := float32(task.TimerSecondSpinner.Number() + (task.TimerMinuteSpinner.Number() * 60))

			if countdownMax <= 0 {
				task.TimerRunning = false
			} else {

				if task.TimerValue >= countdownMax {

					task.TimerValue = countdownMax
					task.TimerRunning = false
					task.TimerValue = 0
					task.Board.Project.Log("Timer [%s] elapsed.", task.TimerName.Text())

					if task.Board.Project.SoundVolume.Number() > 0 {

						audioFile, _ := task.Board.Project.LoadResource(GetPath("assets", "alarm.wav"))
						stream, format, _ := audioFile.Audio()

						fn := func() {
							stream.Close()
						}

						volumed := &effects.Volume{
							Streamer: stream,
							Base:     2,
							Volume:   float64(task.Board.Project.SoundVolume.Number()-10) / 2,
						}

						speaker.Play(beep.Seq(beep.Resample(1, format.SampleRate, beep.SampleRate(task.Board.Project.SampleRate.ChoiceAsInt()), volumed), beep.Callback(fn)))

					}

					if task.TaskBelow != nil && task.TaskBelow.Is(TASK_TYPE_TIMER) {
						task.TaskBelow.ToggleTimer()
					}

				} else {
					task.TimerValue += rl.GetFrameTime()
				}

			}

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

		case TASK_TYPE_MAP:
			if task.MapImage != nil {
				iy := task.DisplaySize.Y - float32(task.Board.Project.GridSize)
				task.MapImage.Resize(task.DisplaySize.X, iy)
			}

		case TASK_TYPE_WHITEBOARD:
			if task.Whiteboard != nil {
				iy := task.DisplaySize.Y - float32(task.Board.Project.GridSize)
				task.Whiteboard.Resize(task.DisplaySize.X, iy)
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

	if task.Board.Project.BracketSubtasks.Checked && len(task.SubTasks) > 0 {

		endingTask := task.SubTasks[len(task.SubTasks)-1]

		for len(endingTask.SubTasks) != 0 {
			endingTask = endingTask.SubTasks[len(endingTask.SubTasks)-1]
		}

		ep := endingTask.Position
		ep.Y += endingTask.Rect.Height

		gh := float32(task.Board.Project.GridSize / 2)
		lines := []rl.Vector2{
			{task.Position.X, task.Position.Y + gh},
			{task.Position.X - gh, task.Position.Y + gh},
			{task.Position.X - gh, ep.Y - gh},
			{ep.X, ep.Y - gh},
		}

		lineColor := getThemeColor(GUI_INSIDE)

		ts := []*Task{}
		ts = append(ts, task.SubTasks...)
		ts = append(ts, task)

		for _, t := range ts {
			if t.Selected {
				lineColor = getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
				break
			}
		}

		for i := range lines {
			if i == len(lines)-1 {
				break
			}
			rl.DrawLineEx(lines[i], lines[i+1], 1, lineColor)
		}
		// rl.DrawLineEx(task.Position, ep, 1, rl.White)

	}

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
	case TASK_TYPE_SOUND:
		_, filename := filepath.Split(task.FilePathTextbox.Text())
		name = filename
	case TASK_TYPE_BOOLEAN:
		fallthrough
	case TASK_TYPE_PROGRESSION:
		cut := strings.Index(name, "\n")
		if cut >= 0 {
			if task.Board.Project.ShowIcons.Checked {
				extendedText = true
			}
			name = name[:cut]
		}
	case TASK_TYPE_TIMER:
		minutes := int(task.TimerValue / 60)
		seconds := int(task.TimerValue) % 60
		timeString := fmt.Sprintf("%02d:%02d", minutes, seconds)
		maxTimeString := fmt.Sprintf("%02d:%02d", task.TimerMinuteSpinner.Number(), task.TimerSecondSpinner.Number())
		name = task.TimerName.Text() + " : " + timeString + " / " + maxTimeString
	}

	if len(task.SubTasks) > 0 && task.Completable() {
		currentFinished := 0
		for _, child := range task.SubTasks {
			if child.Complete() {
				currentFinished++
			}
		}
		name = fmt.Sprintf("%s (%d / %d)", name, currentFinished, len(task.SubTasks))
	} else if task.Is(TASK_TYPE_PROGRESSION) {
		name = fmt.Sprintf("%s (%d / %d)", name, task.CompletionProgressionCurrent.Number(), task.CompletionProgressionMax.Number())
	}

	sequenceType := task.Board.Project.NumberingSequence.CurrentChoice
	if sequenceType != NUMBERING_SEQUENCE_OFF && task.NumberingPrefix[0] != -1 && task.Completable() {
		n := ""

		for i, value := range task.NumberingPrefix {

			if !task.Board.Project.NumberTopLevel.Checked && i == 0 {
				continue
			}

			romanNumber := roman.NewRoman().ToRoman(value)

			switch sequenceType {
			case NUMBERING_SEQUENCE_NUMBER:
				n += fmt.Sprintf("%d.", value)
			case NUMBERING_SEQUENCE_NUMBER_DASH:
				if i == len(task.NumberingPrefix)-1 {
					n += fmt.Sprintf("%d)", value)
				} else {
					n += fmt.Sprintf("%d-", value)
				}
			case NUMBERING_SEQUENCE_BULLET:
				fallthrough
			case NUMBERING_SEQUENCE_SQUARE:
				fallthrough
			case NUMBERING_SEQUENCE_STAR:
				n += "   "
			case NUMBERING_SEQUENCE_ROMAN:
				n += fmt.Sprintf("%s.", romanNumber)

			}
		}
		task.PrefixText = n
		name = fmt.Sprintf("%s %s", task.PrefixText, name)
	}

	invalidImage := task.Image.ID == 0 && task.GifAnimation == nil
	if !invalidImage && task.Is(TASK_TYPE_IMAGE) {
		name = ""
	}

	if task.Completable() && !task.Complete() && task.DeadlineCheckbox.Checked {
		// If there's a deadline, let's tell you how long you have
		deadlineDuration := task.CalculateDeadlineDuration()
		deadlineDuration += time.Hour * 24
		if deadlineDuration.Hours() > 24 {
			duration, _ := durafmt.ParseString(deadlineDuration.String())
			duration.LimitFirstN(1)
			name += " | Due in " + duration.String()
		} else if deadlineDuration.Hours() >= 0 {
			name += " | Due today!"
		} else {
			duration, _ := durafmt.ParseString((-deadlineDuration).String())
			duration.LimitFirstN(1)
			name += fmt.Sprintf(" | Overdue by %s!", duration.String())
		}

	}

	taskDisplaySize := task.DisplaySize

	if !task.Is(TASK_TYPE_IMAGE, TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD) {

		taskDisplaySize = rl.MeasureTextEx(font, name, float32(programSettings.FontSize), spacing)

		if taskDisplaySize.X > 0 {
			taskDisplaySize.X += 4
		}

		if task.Board.Project.ShowIcons.Checked && (!task.Is(TASK_TYPE_IMAGE) || invalidImage) {
			taskDisplaySize.X += 16
			if extendedText {
				taskDisplaySize.X += 16
			}

			if task.Is(TASK_TYPE_TIMER, TASK_TYPE_SOUND) {
				taskDisplaySize.X += 32
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

	if (task.Is(TASK_TYPE_IMAGE) && task.Image.ID != 0) || task.Is(TASK_TYPE_MAP) || task.Is(TASK_TYPE_WHITEBOARD) {
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

	if task.Complete() && !task.Is(TASK_TYPE_PROGRESSION) && len(task.SubTasks) == 0 {
		color = getThemeColor(GUI_INSIDE_HIGHLIGHTED)
	}

	if task.Is(TASK_TYPE_NOTE) {
		color = getThemeColor(GUI_NOTE_COLOR)
	}

	outlineColor := getThemeColor(GUI_OUTLINE)

	if task.Selected {
		outlineColor = getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
	}

	// Moved this to a function because it's used for the inside and outside, and the
	// progress bar for progression-based Tasks.
	applyGlow := func(color rl.Color) rl.Color {

		if (task.Completable() && ((task.Complete() && task.Board.Project.CompleteTasksGlow.Checked) || (!task.Complete() && task.Board.Project.IncompleteTasksGlow.Checked))) || (task.Selected && task.Board.Project.SelectedTasksGlow.Checked) {

			glowVariance := float64(20)
			if task.Selected {
				glowVariance = 80
			}

			glow := int32(math.Sin(float64((rl.GetTime()*math.Pi*2-(float32(task.ID)*0.1))))*(glowVariance/2) + (glowVariance / 2))

			color = ColorAdd(color, -glow)
		}

		return color

	}

	color = applyGlow(color)
	outlineColor = applyGlow(outlineColor)

	perc := float32(0)

	if len(task.SubTasks) > 0 && task.Completable() {
		totalComplete := 0
		for _, child := range task.SubTasks {
			if child.Complete() {
				totalComplete++
			}
		}
		perc = float32(totalComplete) / float32(len(task.SubTasks))
	} else if task.Is(TASK_TYPE_PROGRESSION) {

		cnum := task.CompletionProgressionCurrent.Number()
		mnum := task.CompletionProgressionMax.Number()

		if mnum < cnum {
			task.CompletionProgressionMax.SetNumber(cnum)
			mnum = cnum
		}

		if mnum != 0 {
			perc = float32(cnum) / float32(mnum)
		}

	} else if task.Is(TASK_TYPE_SOUND) && task.SoundStream != nil {
		pos := task.SoundStream.Position()
		len := task.SoundStream.Len()
		perc = float32(pos) / float32(len)
	} else if task.Is(TASK_TYPE_TIMER) {

		countdownMax := float32(task.TimerSecondSpinner.Number() + (task.TimerMinuteSpinner.Number() * 60))

		// If countdownMax == 0, then task.TimerValue / countdownMax can equal a NaN, which breaks drawing the
		// filling rectangle.
		if countdownMax > 0 {
			perc = task.TimerValue / countdownMax
		}

	}

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

	bgRect := task.Rect
	if task.Is(TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD) {
		bgRect.Height = 16 // For a Map, the background is effectively transparent
	}

	// Lines don't get a background
	if !task.Is(TASK_TYPE_LINE) {
		//rl.DrawRectangleRec(bgRect, color)
	}

	//if task.Board.Project.DeadlineAnimation.CurrentChoice < 4 {
	//	if task.Due() == TASK_DUE_TODAY {
	//		src := rl.Rectangle{208 + rl.GetTime()*30, 0, task.Rect.Width, task.Rect.Height}
	//		dst := task.Rect
	//		rl.DrawTexturePro(task.Board.Project.Patterns, src, dst, rl.Vector2{}, 0, getThemeColor(GUI_INSIDE_HIGHLIGHTED))
	//	} else if task.Due() == TASK_DUE_LATE {
	//		src := rl.Rectangle{208 + rl.GetTime()*120, 16, task.Rect.Width, task.Rect.Height}
	//		dst := task.Rect
	//		rl.DrawTexturePro(task.Board.Project.Patterns, src, dst, rl.Vector2{}, 0, getThemeColor(GUI_INSIDE_HIGHLIGHTED))
	//	}
	//}

	if task.PercentageComplete != 0 {
		rect := task.Rect
		rect.Width *= task.PercentageComplete
		rectColor := applyGlow(getThemeColor(GUI_INSIDE_HIGHLIGHTED))
		rectColor.A = alpha
		//rl.DrawRectangleRec(rect, rectColor)
	}

	if task.Is(TASK_TYPE_IMAGE) {

		if task.GifAnimation != nil {
			task.Image = task.GifAnimation.GetTexture()
			task.GifAnimation.Update(task.Board.Project.GetFrameTime())
		}

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

	if task.Is(TASK_TYPE_MAP) && task.MapImage != nil {

		task.MapImage.Update()

		bgColor := rl.Black
		bgColor.A = 64
		color := rl.White
		if task.Board.Project.GraphicalTasksTransparent.Checked {
			color.A = alpha
		}
		gs := float32(task.Board.Project.GridSize)
		src := rl.Rectangle{0, 0, float32(task.MapImage.Texture.Texture.Width), -float32(task.MapImage.Texture.Texture.Height)}
		dst := rl.Rectangle{task.Rect.X, task.Rect.Y + gs, float32(task.MapImage.Texture.Texture.Width), float32(task.MapImage.Texture.Texture.Height)}

		if task.MapImage.Editing {
			bgColor = getThemeColor(GUI_INSIDE_HIGHLIGHTED)
			color.A = 255
		} else {
			themeColor := getThemeColor(GUI_INSIDE_DISABLED)
			if themeColor.R < 128 && themeColor.G < 128 && themeColor.B < 128 {
				bgColor.R = rl.White.R
				bgColor.G = rl.White.G
				bgColor.B = rl.White.B
			}
		}

		rl.DrawRectanglePro(
			rl.Rectangle{task.Rect.X,
				task.Rect.Y + gs,
				task.MapImage.Width(),
				task.MapImage.Height()},
			rl.Vector2{},
			0,
			[]rl.Color{bgColor})

		rl.DrawTexturePro(task.MapImage.Texture.Texture, src, dst, rl.Vector2{}, 0, color)

	}

	if task.Is(TASK_TYPE_WHITEBOARD) && task.Whiteboard != nil {

		task.Whiteboard.Update()

		y := task.Rect.Y + float32(task.Board.Project.GridSize)
		rl.DrawLineEx(rl.Vector2{task.Rect.X, y}, rl.Vector2{task.Rect.X + task.Rect.Width, y}, 1, getThemeColor(GUI_OUTLINE))

		gs := float32(task.Board.Project.GridSize)
		src := rl.Rectangle{0, 0, float32(task.Whiteboard.Texture.Texture.Width), -float32(task.Whiteboard.Texture.Texture.Height)}
		dst := rl.Rectangle{task.Rect.X, task.Rect.Y + gs, float32(task.Whiteboard.Texture.Texture.Width * 2), float32(task.Whiteboard.Texture.Texture.Height * 2)}

		color := rl.White
		if task.Board.Project.GraphicalTasksTransparent.Checked {
			color.A = alpha
		}
		rl.DrawTexturePro(task.Whiteboard.Texture.Texture, src, dst, rl.Vector2{}, 0, color)

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
	if !task.Is(TASK_TYPE_IMAGE, TASK_TYPE_LINE, TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD) {

		textPos := rl.Vector2{task.Rect.X + 2, task.Rect.Y + 2}

		if task.Board.Project.ShowIcons.Checked {
			textPos.X += 16
		}
		if task.Is(TASK_TYPE_TIMER, TASK_TYPE_SOUND) {
			textPos.X += 32
		}

		DrawText(textPos, name)

		if !task.Board.Project.TaskOpen && !task.Board.Project.Searchbar.Focused && !task.Board.Project.ProjectSettingsOpen && task.Board.Project.PopupAction == "" && (task.Is(TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION, TASK_TYPE_NOTE)) {

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

	controlPos := float32(0)

	if task.Board.Project.ShowIcons.Checked {

		controlPos = 16

		iconColor := getThemeColor(GUI_FONT_COLOR)
		iconSrc := rl.Rectangle{16, 0, 16, 16}
		rotation := float32(0)

		iconSrcIconPositions := map[int][]float32{
			TASK_TYPE_BOOLEAN:     {0, 0},
			TASK_TYPE_PROGRESSION: {32, 0},
			TASK_TYPE_NOTE:        {64, 0},
			TASK_TYPE_SOUND:       {80, 0},
			TASK_TYPE_IMAGE:       {96, 0},
			TASK_TYPE_TIMER:       {0, 16},
			TASK_TYPE_LINE:        {128, 32},
			TASK_TYPE_MAP:         {0, 32},
			TASK_TYPE_WHITEBOARD:  {64, 16},
		}

		if task.Is(TASK_TYPE_SOUND) {
			if task.SoundStream == nil || task.SoundControl.Paused {
				iconColor = getThemeColor(GUI_OUTLINE)
			}
		}

		iconSrc.X = iconSrcIconPositions[task.TaskType.CurrentChoice][0]
		iconSrc.Y = iconSrcIconPositions[task.TaskType.CurrentChoice][1]

		if len(task.SubTasks) > 0 && task.Completable() {
			iconSrc.X = 128 // Hardcoding this because I'm an idiot
			iconSrc.Y = 16
		}

		// task.ArrowPointingToTask = nil

		if task.Is(TASK_TYPE_LINE) && task.LineBase != nil {

			iconSrc.X = 144
			iconSrc.Y = 32
			rotation = rl.Vector2Angle(task.LineBase.Position, task.Position)

			if task.TaskRight != nil && task.TaskRight != task.LineBase {
				rotation = 0
			} else if task.TaskLeft != nil && task.TaskLeft != task.LineBase {
				rotation = 180
			} else if task.TaskAbove != nil && task.TaskAbove != task.LineBase {
				rotation = -90
			} else if task.TaskBelow != nil && task.TaskBelow != task.LineBase {
				rotation = 90
			}

			if task.TaskUnder != nil {
				// Line endings that are inside Task Rectangles become "X"
				iconSrc.X = 160
				rotation = 0
			}

		}

		if task.Complete() {
			iconSrc.X += 16
			iconColor = getThemeColor(GUI_OUTLINE_HIGHLIGHTED)
		}

		if task.Is(TASK_TYPE_SOUND) && task.SoundStream == nil {
			iconSrc.Y += 16
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

		if task.Completable() && !task.Complete() && task.DeadlineCheckbox.Checked {

			deadlineAnimate := task.Board.Project.DeadlineAnimation.CurrentChoice

			if deadlineAnimate < 3 {
				clockPos := rl.Vector2{0, 0}
				iconSrc = rl.Rectangle{144, 0, 16, 16}

				if task.Due() == TASK_DUE_LATE {
					iconSrc.X += 32
				} else if task.Due() == TASK_DUE_TODAY {
					iconSrc.X += 16
				} // else it's due in the future, so just a clock icon is fine

				if deadlineAnimate == 0 || (deadlineAnimate == 1 && task.Due() == TASK_DUE_LATE) {
					clockPos.X += float32(math.Sin(float64(float32(task.ID)*0.1)+float64(rl.GetTime())*3.1415)) * 4
				}

				rl.DrawTexturePro(task.Board.Project.GUI_Icons, iconSrc, rl.Rectangle{task.Rect.X - 16 + clockPos.X, task.Rect.Y + clockPos.Y, 16, 16}, rl.Vector2{0, 0}, 0, rl.White)

			}

		}

	}

	if task.NumberingPrefix[0] != -1 && task.Completable() {

		numberingIcon := map[int]rl.Rectangle{
			NUMBERING_SEQUENCE_BULLET: rl.Rectangle{176, 32, 8, 8},
			NUMBERING_SEQUENCE_SQUARE: rl.Rectangle{184, 32, 8, 8},
			NUMBERING_SEQUENCE_STAR:   rl.Rectangle{192, 32, 8, 8},
		}

		if src, exists := numberingIcon[task.Board.Project.NumberingSequence.CurrentChoice]; exists {
			x := float32(18)

			if !task.Board.Project.ShowIcons.Checked {
				x -= 16
			}

			bulletCount := len(task.NumberingPrefix)
			if !task.Board.Project.NumberTopLevel.Checked {
				bulletCount--
			}

			for i := 0; i < bulletCount; i++ {
				rl.DrawTexturePro(task.Board.Project.GUI_Icons, src, rl.Rectangle{task.Rect.X + x, task.Rect.Y + 4, src.Width, src.Height}, rl.Vector2{}, 0, getThemeColor(GUI_FONT_COLOR))
				x += src.Width
			}

		}

	}

	if task.Is(TASK_TYPE_TIMER) {

		x := task.Rect.X + controlPos
		y := task.Rect.Y

		srcX := float32(16)
		if task.TimerRunning {
			srcX += 16
		}

		if task.SmallButton(srcX, 16, 16, 16, x, y) && (task.TimerMinuteSpinner.Number() > 0 || task.TimerSecondSpinner.Number() > 0) {
			task.ToggleTimer()
		}
		if task.SmallButton(48, 16, 16, 16, x+16, y) {
			task.TimerValue = 0
			task.Board.Project.Log("Timer [%s] reset.", task.TimerName.Text())
		}
	} else if task.Is(TASK_TYPE_SOUND) {

		x := task.Rect.X + controlPos
		y := task.Rect.Y

		srcX := float32(16)
		if task.SoundControl != nil && !task.SoundControl.Paused {
			srcX += 16
		}

		if task.SmallButton(srcX, 16, 16, 16, x, y) && task.SoundControl != nil {
			task.ToggleSound()
		}
		if task.SmallButton(48, 16, 16, 16, x+16, y) && task.SoundControl != nil {
			speaker.Lock()
			task.SoundStream.Seek(0)
			speaker.Unlock()
			_, filename := filepath.Split(task.FilePathTextbox.Text())
			task.Board.Project.Log("Sound Task [%s] restarted.", filename)
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

	if task.Is(TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD) {
		depth = -100
	} else if task.Is(TASK_TYPE_LINE) {
		depth = 100
	}

	return depth

}

func (task *Task) UpdateNeighbors() {

	gs := float32(task.Board.Project.GridSize)

	task.TaskRight = nil
	task.TaskLeft = nil
	task.TaskAbove = nil
	task.TaskBelow = nil
	task.TaskUnder = nil

	tasks := task.Board.GetTasksInRect(task.Position.X+gs, task.Position.Y, task.Rect.Width, task.Rect.Height)
	sortfunc := func(i, j int) bool {
		return tasks[i].Numberable() || (tasks[i].Is(TASK_TYPE_NOTE) && !tasks[j].Numberable()) // Prioritize numberable Tasks or Notes to be counted as neighbors (though other Tasks can be neighbors still)
	}

	sort.Slice(tasks, sortfunc)
	for _, t := range tasks {
		if t != task {
			task.TaskRight = t
			break
		}
	}

	tasks = task.Board.GetTasksInRect(task.Position.X-gs, task.Position.Y, task.Rect.Width, task.Rect.Height)
	sort.Slice(tasks, sortfunc)

	for _, t := range tasks {
		if t != task {
			task.TaskLeft = t
			break
		}
	}

	tasks = task.Board.GetTasksInRect(task.Position.X, task.Position.Y-gs, task.Rect.Width, task.Rect.Height)
	sort.Slice(tasks, sortfunc)

	for _, t := range tasks {
		if t != task {
			task.TaskAbove = t
			break
		}
	}

	tasks = task.Board.GetTasksInRect(task.Position.X, task.Position.Y+gs, task.Rect.Width, task.Rect.Height)
	sort.Slice(tasks, sortfunc)
	for _, t := range tasks {
		if t != task {
			task.TaskBelow = t
			break
		}
	}

	tasks = task.Board.GetTasksInRect(task.Position.X, task.Position.Y, task.Rect.Width, task.Rect.Height)
	sort.Slice(tasks, sortfunc)
	for _, t := range tasks {
		if t != task {
			task.TaskUnder = t
			break
		}
	}

}

func (task *Task) DeadlineTime() time.Time {
	return time.Date(task.DeadlineYearSpinner.Number(), time.Month(task.DeadlineMonthSpinner.CurrentChoice+1), task.DeadlineDaySpinner.Number(), 0, 0, 0, 0, time.Now().Location())
}

func (task *Task) CalculateDeadlineDuration() time.Duration {
	return task.DeadlineTime().Sub(time.Now())
}

func (task *Task) Due() int {
	if !task.Complete() && task.Completable() && task.DeadlineCheckbox.Checked {
		// If there's a deadline, let's tell you how long you have
		deadlineDuration := task.CalculateDeadlineDuration()
		if deadlineDuration.Hours() > 0 {
			return TASK_DUE_FUTURE
		} else if deadlineDuration.Hours() >= -24 {
			return TASK_DUE_TODAY
		} else {
			return TASK_DUE_LATE
		}
	}
	return TASK_NOT_DUE
}

func (task *Task) DrawShadow() {

	if task.Visible && !task.Is(TASK_TYPE_LINE) {

		depthRect := task.Rect
		shadowColor := getThemeColor(GUI_SHADOW_COLOR)

		if task.Board.Project.TaskTransparency.Number() < 255 {
			t := float32(task.Board.Project.TaskTransparency.Number())
			alpha := uint8((t / float32(task.Board.Project.TaskTransparency.Maximum)) * (255 - 32))
			shadowColor.A = 32 + alpha
		}

		if task.Board.Project.TaskShadowSpinner.CurrentChoice == 2 || task.Board.Project.TaskShadowSpinner.CurrentChoice == 3 {

			src := rl.Rectangle{224, 0, 8, 8}
			if task.Board.Project.TaskShadowSpinner.CurrentChoice == 3 {
				src.X = 248
			}

			dst := depthRect
			dst.X += dst.Width
			dst.Width = src.Width
			dst.Height = src.Height
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, src, dst, rl.Vector2{0, 0}, 0, shadowColor)

			src.Y += src.Height
			dst.Y += src.Height
			dst.Height = depthRect.Height - src.Height
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, src, dst, rl.Vector2{0, 0}, 0, shadowColor)

			src.Y += src.Height
			dst.Y += dst.Height
			dst.Height = src.Height
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, src, dst, rl.Vector2{0, 0}, 0, shadowColor)

			src.X -= src.Width
			dst.X = depthRect.X + src.Width
			dst.Width = depthRect.Width - src.Width
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, src, dst, rl.Vector2{0, 0}, 0, shadowColor)

			src.X -= src.Width
			dst.X = depthRect.X
			dst.Width = src.Width
			rl.DrawTexturePro(task.Board.Project.GUI_Icons, src, dst, rl.Vector2{0, 0}, 0, shadowColor)

		} else if task.Board.Project.TaskShadowSpinner.CurrentChoice == 1 {

			depthRect.Y += depthRect.Height
			depthRect.Height = 4
			depthRect.X += 4
			rl.DrawRectangleRec(depthRect, shadowColor)

			depthRect.X = task.Rect.X + task.Rect.Width
			depthRect.Y = task.Rect.Y + 4
			depthRect.Width = 4
			depthRect.Height = task.Rect.Height - 4
			rl.DrawRectangleRec(depthRect, shadowColor)

		}

	}

}

func (task *Task) PostDraw() {

	if task.Open {

		column := task.EditPanel.Columns[0]

		column.Mode = task.TaskType.CurrentChoice

		deadlineCheck := task.EditPanel.FindItems("deadline_on")[0]
		deadlineCheck.On = task.Completable()

		if task.Completable() {

			completionTime := task.CompletionTime.Format("Monday, Jan 2, 2006, 15:04")
			if task.CompletionTime.IsZero() {
				completionTime = "N/A"
			}
			task.CompletionTimeLabel.Text = completionTime

		}

		for _, option := range task.EditPanel.FindItems("deadline_sub") {
			option.On = deadlineCheck.On && task.DeadlineCheckbox.Checked
		}

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

		if task.MapImage != nil {

			shiftLeft := task.EditPanel.FindItems("shift left")[0].Element.(*Button)
			shiftRight := task.EditPanel.FindItems("shift right")[0].Element.(*Button)
			shiftUp := task.EditPanel.FindItems("shift up")[0].Element.(*Button)
			shiftDown := task.EditPanel.FindItems("shift down")[0].Element.(*Button)

			if shiftLeft.Clicked {
				task.MapImage.Shift(-1, 0)
			} else if shiftRight.Clicked {
				task.MapImage.Shift(1, 0)
			} else if shiftUp.Clicked {
				task.MapImage.Shift(0, -1)
			} else if shiftDown.Clicked {
				task.MapImage.Shift(0, 1)
			}

			if clear := task.EditPanel.FindItems("clear")[0].Element.(*Button); clear.Clicked {
				task.MapImage.Clear()
			}

		}

		if task.Whiteboard != nil {

			shiftLeft := task.EditPanel.FindItems("shift left")[0].Element.(*Button)
			shiftRight := task.EditPanel.FindItems("shift right")[0].Element.(*Button)
			shiftUp := task.EditPanel.FindItems("shift up")[0].Element.(*Button)
			shiftDown := task.EditPanel.FindItems("shift down")[0].Element.(*Button)

			if shiftLeft.Clicked {
				task.Whiteboard.Shift(-8, 0)
			} else if shiftRight.Clicked {
				task.Whiteboard.Shift(8, 0)
			} else if shiftUp.Clicked {
				task.Whiteboard.Shift(0, -8)
			} else if shiftDown.Clicked {
				task.Whiteboard.Shift(0, 8)
			}

			if clear := task.EditPanel.FindItems("clear")[0].Element.(*Button); clear.Clicked {
				task.Whiteboard.Clear()
			}

			if invert := task.EditPanel.FindItems("invert")[0].Element.(*Button); invert.Clicked {
				task.Whiteboard.Invert()
			}

		}

		if task.EditPanel.Exited {
			task.ReceiveMessage(MessageTaskClose, nil)
		}

	}

}

func (task *Task) Complete() bool {

	if task.Completable() && len(task.SubTasks) > 0 {
		for _, child := range task.SubTasks {
			if !child.Complete() {
				return false
			}
		}
		return true
	} else {
		if task.Is(TASK_TYPE_BOOLEAN) {
			return task.CompletionCheckbox.Checked
		} else if task.Is(TASK_TYPE_PROGRESSION) {
			return task.CompletionProgressionMax.Number() > 0 && task.CompletionProgressionCurrent.Number() >= task.CompletionProgressionMax.Number()
		}
	}
	return false
}

func (task *Task) Completable() bool {
	return task.Is(TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION)
}

func (task *Task) Resizeable() bool {
	return task.Is(TASK_TYPE_IMAGE, TASK_TYPE_MAP, TASK_TYPE_WHITEBOARD)
}

func (task *Task) SetCompletion(complete bool) {

	if task.Completable() {

		if len(task.SubTasks) == 0 {

			task.CompletionCheckbox.Checked = complete

			// VVV This is a nice addition but conversely makes it suuuuuper easy to screw yourself over
			// for _, child := range subTasks {
			// 	child.SetCompletion(complete)
			// }

			if complete {
				task.CompletionProgressionCurrent.SetNumber(task.CompletionProgressionMax.Number())
			} else {
				task.CompletionProgressionCurrent.SetNumber(0)
			}
		}

	} else if task.Is(TASK_TYPE_SOUND) {
		task.ToggleSound()
	} else if task.Is(TASK_TYPE_TIMER) {
		task.ToggleTimer()
	} else if task.Is(TASK_TYPE_LINE) {
		if task.LineBase != nil {
			task.LineBase.Selected = true
			task.LineBase.SetCompletion(true) // Select base
		} else {
			for _, ending := range task.ValidLineEndings() {
				ending.Selected = true
			}
			task.Board.FocusViewOnSelectedTasks()
		}
	} else if task.Is(TASK_TYPE_MAP) && task.MapImage != nil {
		task.MapImage.ToggleEditing()
	} else if task.Is(TASK_TYPE_WHITEBOARD) && task.Whiteboard != nil {
		task.Whiteboard.ToggleEditing()
	}

}

func (task *Task) LoadResource() {

	task.SuccessfullyLoadedResourceOnce = false

	if task.FilePathTextbox.Text() != "" {

		res, _ := task.Board.Project.LoadResource(task.FilePathTextbox.Text())

		if res != nil {

			task.SuccessfullyLoadedResourceOnce = true

			if task.Is(TASK_TYPE_IMAGE) {

				if res.IsTexture() {

					if task.GifAnimation != nil {
						task.GifAnimation.Destroy()
						task.GifAnimation = nil
					}
					task.Image = res.Texture()
					if task.PrevFilePath != task.FilePathTextbox.Text() && task.DisplaySize.X == 0 && task.DisplaySize.Y == 0 {
						task.DisplaySize.X = float32(task.Image.Width)
						task.DisplaySize.Y = float32(task.Image.Height)
					}

				} else if res.IsGIF() {

					if task.GifAnimation != nil && task.PrevFilePath != task.FilePathTextbox.Text() {
						task.DisplaySize.X = 0
						task.DisplaySize.Y = 0
					}
					task.GifAnimation = NewGifAnimation(res.GIF())
					if task.DisplaySize.X == 0 || task.DisplaySize.Y == 0 {
						task.DisplaySize.X = float32(task.GifAnimation.Data.Image[0].Bounds().Size().X)
						task.DisplaySize.Y = float32(task.GifAnimation.Data.Image[0].Bounds().Size().Y)
					}

				}

			} else if task.Is(TASK_TYPE_SOUND) {

				if task.SoundStream != nil {
					speaker.Lock()
					task.SoundControl.Paused = true
					task.SoundControl.Streamer = nil
					task.SoundControl = nil
					speaker.Unlock()
				}

				stream, format, err := res.Audio()

				if err == nil && stream != nil {

					task.SoundStream = stream
					projectSampleRate := beep.SampleRate(task.Board.Project.SampleRate.ChoiceAsInt())

					if format.SampleRate != projectSampleRate {
						task.Board.Project.Log("Sample rate of audio file %s not the same as project sample rate %d.", res.ResourcePath, projectSampleRate)
						task.Board.Project.Log("File will be resampled.")
						// SolarLune: Note the resample quality has to be 1 (poor); otherwise, it seems like some files will cause beep to crash with an invalid
						// index error. Probably has to do something with how the resampling process works combined with particular sound files.
						// For me, it crashes on playing back the file "10 3-audio.wav" on my computer repeatedly (after about 4-6 loops, it crashes).
						task.SoundControl = &beep.Ctrl{
							Streamer: beep.Resample(1, format.SampleRate, projectSampleRate, stream),
							Paused:   true}
					} else {
						task.SoundControl = &beep.Ctrl{Streamer: stream, Paused: true}
					}

					task.SoundVolume = &effects.Volume{
						Streamer: task.SoundControl,
						Base:     2,
					}

					task.UpdateSoundVolume()

					task.Board.Project.Log("Sound file %s loaded properly.", res.ResourcePath)
					speaker.Play(beep.Seq(task.SoundVolume, beep.Callback(task.OnSoundCompletion)))

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
		} else if (!task.Is(TASK_TYPE_MAP) || task.MapImage == nil || !task.MapImage.Editing) && (!task.Is(TASK_TYPE_WHITEBOARD) || task.Whiteboard == nil || !task.Whiteboard.Editing) {

			// We have to consume after double-clicking so you don't click outside of the new panel and exit it immediately
			// or actuate a GUI element accidentally. HOWEVER, we want it here because double-clicking might not actually
			// open the Task, as can be seen here
			ConsumeMouseInput(rl.MouseLeftButton)

			if !task.DeadlineCheckbox.Checked {
				now := time.Now()
				task.DeadlineDaySpinner.SetNumber(now.Day())
				task.DeadlineMonthSpinner.SetChoice(now.Month().String())
				task.DeadlineYearSpinner.SetNumber(time.Now().Year())
			}

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

			if task.Is(TASK_TYPE_MAP) {
				if task.MapImage == nil {
					task.MapImage = NewMapImage(task)
				}
				task.DisplaySize.X = task.MapImage.Width()
				task.DisplaySize.Y = task.MapImage.Height() + float32(task.Board.Project.GridSize)
				task.MapImage.Update()
			}

			if task.Is(TASK_TYPE_WHITEBOARD) {
				if task.Whiteboard == nil {
					task.Whiteboard = NewWhiteboard(task)
				}
				task.DisplaySize.X = float32(task.Whiteboard.Width * 2)
				task.DisplaySize.Y = float32(task.Whiteboard.Height*2 + task.Board.Project.GridSize)
				task.Whiteboard.Update()
			}

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
		if task.Selected && ((task.MapImage == nil || !task.MapImage.Editing) && (task.Whiteboard == nil || !task.Whiteboard.Editing)) {
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
	} else if message == MessageNeighbors {
		task.UpdateNeighbors()
	} else if message == MessageNumbering {
		task.SetPrefix()
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

		if data["task"] == task && task.SoundStream != nil && task.SoundControl != nil {
			task.SoundControl.Paused = true
		}

	} else if message == MessageThemeChange {
		if task.Is(TASK_TYPE_MAP) && task.MapImage != nil {
			task.MapImage.Changed = true // Force update to change color palette
		} else if task.Is(TASK_TYPE_WHITEBOARD) && task.Whiteboard != nil {
			task.Whiteboard.Deserialize(task.Whiteboard.Serialize()) // De and re-serialize to change the colors
		}
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

func (task *Task) ToggleSound() {
	if task.SoundControl != nil {
		speaker.Lock()
		task.SoundControl.Paused = !task.SoundControl.Paused

		_, filename := filepath.Split(task.FilePathTextbox.Text())
		if task.SoundControl.Paused {
			task.Board.Project.Log("Paused [%s].", filename)
		} else {
			task.Board.Project.Log("Playing [%s].", filename)
		}

		speaker.Unlock()
	}
}

func (task *Task) StopSound() {
	if task.SoundControl != nil {
		speaker.Lock()
		task.SoundControl.Paused = true
		speaker.Unlock()
	}
}

func (task *Task) OnSoundCompletion() {
	if task.SoundControl != nil && !task.SoundControl.Paused {
		task.SoundComplete = true
	}
}

func (task *Task) UpdateSoundVolume() {
	speaker.Lock()
	task.SoundVolume.Volume = float64(task.Board.Project.SoundVolume.Number()-10) / 2
	task.SoundVolume.Silent = task.Board.Project.SoundVolume.Number() == 0
	speaker.Unlock()
}

func (task *Task) ToggleTimer() {
	task.TimerRunning = !task.TimerRunning
	if task.TimerRunning {
		task.Board.Project.Log("Timer [%s] started.", task.TimerName.Text())
	} else {
		task.Board.Project.Log("Timer [%s] paused.", task.TimerName.Text())
	}
}

func (task *Task) NeighborInDirection(dirX, dirY float32) *Task {
	if dirX > 0 {
		return task.TaskRight
	} else if dirX < 0 {
		return task.TaskLeft
	} else if dirY < 0 {
		return task.TaskAbove
	} else if dirY > 0 {
		return task.TaskBelow
	}
	return nil
}

func (task *Task) SetPrefix() {

	// Establish the rest of the stack; has to be done here because it has be done after
	// all Tasks have their positions on the Board and neighbors established.

	loopIndex := 0

	if task.Numberable() {

		task.RestOfStack = []*Task{}
		task.SubTasks = []*Task{}
		below := task.TaskBelow
		countingSubTasks := true

		for countingSubTasks && below != nil && below != task {

			// We want to break out in case of situations where Tasks create an infinite loop (a.Below = b, b.Below = c, c.Below = a kind of thing)
			if loopIndex > 1000 {
				break // Emergency in case we get stuck in a loop
			}

			if below.Numberable() {

				task.RestOfStack = append(task.RestOfStack, below)

				taskX, _ := task.Board.Project.WorldToGrid(task.Position.X, task.Position.Y)
				belowX, _ := task.Board.Project.WorldToGrid(below.Position.X, below.Position.Y)

				if countingSubTasks && belowX == taskX+1 {
					task.SubTasks = append(task.SubTasks, below)
				} else if belowX <= taskX {
					countingSubTasks = false
				}

			}

			below = below.TaskBelow

			loopIndex++

		}

	}

	above := task.TaskAbove

	below := task.TaskBelow

	loopIndex = 0

	for above != nil && !above.Numberable() {

		above = above.TaskAbove

		if loopIndex > 1000 {
			break // This SHOULD never happen, but you never know
		}

		loopIndex++

	}

	loopIndex = 0

	for below != nil && !below.Numberable() {

		below = below.TaskBelow

		if loopIndex > 100 {
			break // This SHOULD never happen, but you never know
		}

		loopIndex++
	}

	if above != nil {

		task.NumberingPrefix = append([]int{}, above.NumberingPrefix...)

		if above.Position.X < task.Position.X {
			task.NumberingPrefix = append(task.NumberingPrefix, 0)
		} else if above.Position.X > task.Position.X {
			d := len(above.NumberingPrefix) - int((above.Position.X-task.Position.X)/float32(task.Board.Project.GridSize))
			if d < 1 {
				d = 1
			}

			task.NumberingPrefix = append([]int{}, above.NumberingPrefix[:d]...)
		}

		task.NumberingPrefix[len(task.NumberingPrefix)-1]++

	} else if below != nil {
		task.NumberingPrefix = []int{1}
	} else {
		task.NumberingPrefix = []int{-1}
	}

}

func (task *Task) Numberable() bool {
	return task.Is(TASK_TYPE_BOOLEAN, TASK_TYPE_PROGRESSION)
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

	if task.SoundStream != nil && task.SoundControl != nil {
		task.SoundStream.Close()
		task.SoundControl = nil
	}

	if task.GifAnimation != nil {
		task.GifAnimation.Destroy()
	}

}

func (task *Task) UsesMedia() bool {
	return task.Is(TASK_TYPE_IMAGE, TASK_TYPE_SOUND)
}

func (task *Task) Is(taskTypes ...int) bool {
	for _, taskType := range taskTypes {
		if task.TaskType.CurrentChoice == taskType {
			return true
		}
	}
	return false
}
